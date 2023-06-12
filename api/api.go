package api

import (
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"go.sia.tech/jape"
	"go.sia.tech/renterd/api"
	"lukechampine.com/frand"
)

type (
	server struct {
		root string

		mu   sync.Mutex
		keys map[string]bool
	}
)

func (s *server) userID(c jape.Context) (string, bool) {
	_, password, ok := c.Request.BasicAuth()
	if !ok {
		c.Error(errors.New("unauthorized"), http.StatusUnauthorized)
		return "", false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.keys[password] {
		c.Error(errors.New("unauthorized"), http.StatusUnauthorized)
	}
	return password, true
}

func (s *server) userPath(c jape.Context, fp string) (string, bool) {
	id, ok := s.userID(c)
	if !ok {
		return "", false
	}
	return path.Clean(path.Join(s.root, id, fp)), true
}

func (s *server) handleGETWorkerObjects(c jape.Context) {
	fp := strings.TrimPrefix(path.Clean(c.PathParam("path")), "/")
	if len(fp) == 0 {
		fp = "."
	}

	// separate applicant's objects into their own directory
	fp, ok := s.userPath(c, fp)
	if !ok {
		return
	}

	f, err := os.Open(fp)
	if errors.Is(err, os.ErrNotExist) {
		c.Error(errors.New("not found"), http.StatusNotFound)
		return
	} else if err != nil {
		log.Println(err)
		c.Error(err, http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		c.Error(err, http.StatusInternalServerError)
		return
	} else if stat.IsDir() {
		// list directory
		children, err := os.ReadDir(fp)
		if err != nil {
			c.Error(err, http.StatusInternalServerError)
			return
		}

		var objects []api.ObjectMetadata
		for _, child := range children {
			meta := api.ObjectMetadata{
				Name: child.Name(),
			}

			info, err := child.Info()
			if err != nil {
				c.Error(err, http.StatusInternalServerError)
				return
			} else if !info.IsDir() {
				meta.Size = info.Size()
			}
			objects = append(objects, meta)
		}

		c.Encode(objects)
		return
	}

	http.ServeContent(c.ResponseWriter, c.Request, fp, stat.ModTime(), f)
}

func (s *server) handlePUTWorkerObjects(c jape.Context) {
	fp := strings.TrimPrefix(path.Clean(c.PathParam("path")), "/")
	fp, ok := s.userPath(c, fp)
	if !ok {
		return
	}

	if err := os.MkdirAll(path.Dir(fp), 0755); err != nil {
		c.Error(err, http.StatusInternalServerError)
		return
	}

	r := http.MaxBytesReader(c.ResponseWriter, c.Request.Body, 1<<28) // 256 MiB
	defer r.Close()

	f, err := os.Create(fp)
	if err != nil {
		c.Error(err, http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		c.Error(err, http.StatusInternalServerError)
		return
	}
}

func (s *server) handleDELETEWorkerObjects(c jape.Context) {
	fp := strings.TrimPrefix(path.Clean(c.PathParam("path")), "/")
	fp, ok := s.userPath(c, fp)
	if !ok {
		return
	}

	if _, err := os.Stat(fp); errors.Is(err, os.ErrNotExist) {
		c.Error(errors.New("not found"), http.StatusNotFound)
		return
	}

	if err := os.Remove(fp); err != nil {
		c.Error(err, http.StatusInternalServerError)
		return
	}
}

func (s *server) handlePOSTTestKey(c jape.Context) {
	key := hex.EncodeToString(frand.Bytes(16))
	s.mu.Lock()
	s.keys[key] = true
	s.mu.Unlock()
	c.Encode(key)
}

func Handler(root string) http.Handler {
	s := server{
		root: root,
		keys: make(map[string]bool),
	}

	return jape.Mux(map[string]jape.Handler{
		"GET /api/worker/objects/*path":    s.handleGETWorkerObjects,
		"PUT /api/worker/objects/*path":    s.handlePUTWorkerObjects,
		"DELETE /api/worker/objects/*path": s.handleDELETEWorkerObjects,

		"POST /api/test/register": s.handlePOSTTestKey,
	})
}
