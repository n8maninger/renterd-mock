package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"go.sia.tech/renterd/worker"
	"lukechampine.com/frand"
)

func TestCompat(t *testing.T) {
	apiListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer apiListener.Close()

	web := http.Server{
		Handler:     Handler(t.TempDir()),
		ReadTimeout: 30 * time.Second,
	}
	defer web.Close()

	go func() {
		if err := web.Serve(apiListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	apiAddr := "http://" + apiListener.Addr().String() + "/api/worker"
	client := worker.NewClient(apiAddr, hex.EncodeToString(frand.Bytes(16)))

	obj := make([]byte, 1<<22)
	copy(obj, frand.Bytes(1024))

	err = client.UploadObject(context.Background(), bytes.NewReader(obj), "foo/foo.jpg")
	if err != nil {
		t.Fatal(err)
	}

	// list the object
	entries, err := client.ObjectEntries(context.Background(), "foo")
	if err != nil {
		t.Fatal(err)
	} else if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	} else if entries[0].Name != "foo.jpg" {
		t.Fatal("unexpected name:", entries[0].Name)
	} else if entries[0].Size != 4194304 {
		t.Fatal("unexpected size:", entries[0].Size)
	}

	// download the object
	var buf bytes.Buffer
	if err = client.DownloadObject(context.Background(), &buf, "foo/foo.jpg"); err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(buf.Bytes(), obj) {
		t.Fatal("downloaded object does not match original")
	}

	// check that files do not leak to other users
	client = worker.NewClient(apiAddr, hex.EncodeToString(frand.Bytes(16)))
	_, err = client.ObjectEntries(context.Background(), "foo")
	if err == nil {
		t.Fatal("expected error")
	}
}
