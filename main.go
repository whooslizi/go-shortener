package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[link-shortener] ")

	log.Println("starting...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.LogConfig()

	db, err := InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	poller := NewPoller(cfg, db)
	go poller.Start()

	handler, err := NewHandler(cfg, db)
	if err != nil {
		log.Fatalf("handler init: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("got %v, shutting down", sig)
		db.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
