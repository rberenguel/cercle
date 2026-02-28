package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruben/cercle/internal/api"
	"github.com/ruben/cercle/internal/db"
	"github.com/ruben/cercle/internal/embedder"
	"github.com/ruben/cercle/internal/search"
)

// Version is set at build time via -ldflags "-X main.Version=<ver>".
var Version = "dev"

func main() {
	dbPath := flag.String("db", os.ExpandEnv("$HOME/.cercle/cercle.db"), "Path to SQLite database")
	addr := flag.String("addr", "127.0.0.1:7770", "Listen address")
	vectorsPath := flag.String("vectors", os.ExpandEnv("$HOME/.cercle/vectors.txt"), "Path to GloVe/FastText vectors file (optional)")
	reset := flag.Bool("reset", false, "Wipe the database and exit")
	flag.Parse()

	if *reset {
		for _, f := range []string{*dbPath, *dbPath + "-wal", *dbPath + "-shm"} {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				log.Fatalf("reset: %v", err)
			}
		}
		log.Printf("wiped %s", *dbPath)
		return
	}

	if err := os.MkdirAll(os.ExpandEnv("$HOME/.cercle"), 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Load word vectors if the file exists. Semantic search is silently
	// disabled if no vectors file is present rather than hard-failing.
	var emb *embedder.Embedder
	if _, err := os.Stat(*vectorsPath); err == nil {
		log.Printf("loading word vectors from %s …", *vectorsPath)
		emb, err = embedder.LoadFile(*vectorsPath)
		if err != nil {
			log.Printf("warning: could not load vectors: %v — semantic search disabled", err)
			emb = nil
		}
	} else {
		log.Printf("no vectors file at %s — semantic search disabled (run rlm/scripts/download-vectors)", *vectorsPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := search.NewEmbedWorker(database, emb)
	go worker.Run(ctx)

	srv := api.NewServer(*addr, database, emb, worker, Version)

	go func() {
		log.Printf("cercled %s listening on %s", Version, *addr)
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down")
}
