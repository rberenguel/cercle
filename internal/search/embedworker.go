package search

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/ruben/cercle/internal/embedder"
)

const (
	workerConcurrency  = 4
	workerPollInterval = 30 * time.Second
)

// EmbedWorker drains the un-embedded documents queue in the background.
type EmbedWorker struct {
	db      *sql.DB
	emb     *embedder.Embedder // nil if no vectors loaded
	trigger chan struct{}
}

// NewEmbedWorker creates a worker. Call Run in a goroutine.
// emb may be nil; the worker will start but do nothing until vectors are available.
func NewEmbedWorker(db *sql.DB, emb *embedder.Embedder) *EmbedWorker {
	return &EmbedWorker{
		db:      db,
		emb:     emb,
		trigger: make(chan struct{}, 1),
	}
}

// Trigger wakes the worker immediately (non-blocking; coalesces rapid calls).
func (w *EmbedWorker) Trigger() {
	if w.emb == nil {
		return
	}
	select {
	case w.trigger <- struct{}{}:
	default:
	}
}

// Run processes un-embedded documents until ctx is cancelled.
func (w *EmbedWorker) Run(ctx context.Context) {
	if w.emb == nil {
		log.Println("embedworker: no word vectors loaded — semantic search disabled")
		return
	}
	ticker := time.NewTicker(workerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for w.process(ctx) {
			}
		case <-w.trigger:
			for w.process(ctx) {
			}
		}
	}
}

// process embeds one batch of up to 100 pending documents.
// Returns true if work was done (caller should keep looping), false when queue is empty.
func (w *EmbedWorker) process(ctx context.Context) bool {
	rows, err := w.db.QueryContext(ctx, `
		SELECT d.id, d.content FROM documents d
		WHERE NOT EXISTS (
			SELECT 1 FROM embeddings e WHERE e.doc_id = d.id AND e.model = ?
		)
		AND NOT (
			d.kind = 'file' AND EXISTS (
				SELECT 1 FROM documents c WHERE c.parent_id = d.id
			)
		)
		ORDER BY d.id
		LIMIT 100
	`, ModelID(w.emb))
	if err != nil {
		log.Printf("embedworker: query pending: %v", err)
		return false
	}

	type doc struct {
		id      int64
		content string
	}
	var docs []doc
	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.id, &d.content); err != nil {
			log.Printf("embedworker: scan: %v", err)
		} else {
			docs = append(docs, d)
		}
	}
	rows.Close()

	if len(docs) == 0 {
		return false
	}

	log.Printf("embedworker: embedding %d document(s)", len(docs))

	sem := make(chan struct{}, workerConcurrency)
	var wg sync.WaitGroup
	for _, d := range docs {
		d := d
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := EmbedAndStore(ctx, w.db, w.emb, d.id, d.content); err != nil {
				log.Printf("embedworker: embed doc %d: %v", d.id, err)
			}
		}()
	}
	wg.Wait()
	log.Printf("embedworker: batch done")
	return true
}
