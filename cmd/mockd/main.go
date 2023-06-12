package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/n8maninger/renterd-mock/api"
)

var (
	dir     string
	apiAddr string
)

func init() {
	flag.StringVar(&dir, "dir", ".", "directory to use for root")
	flag.StringVar(&apiAddr, "api.addr", ":9980", "address to listen on for API requests")
	flag.Parse()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	apiListener, err := net.Listen("tcp", apiAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer apiListener.Close()

	web := http.Server{
		Handler:     api.Handler(dir),
		ReadTimeout: 30 * time.Second,
	}
	defer web.Close()

	go func() {
		if err := web.Serve(apiListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	web.Shutdown(shutdownCtx)
}
