I now have all the information needed. Here is the full sequence of events:

  ---
  Re-embedding sequence after a source file is modified

  Components involved: cercled (HTTP daemon), Server (API), handleIndex, IndexDir (indexer), upsertDocument,
  insertChunks, SQLite triggers, EmbedWorker, EmbedAndStore, embedder.Embedder

  Step-by-step

  1. Client → POST /index
  A client (e.g. rlm-index) sends a POST to /index with {"path": "<dir>"}.
  2. Server.handleIndex (internal/api/handlers.go)
  Decodes the request body and calls indexer.IndexDir(...).
  3. IndexDir (internal/indexer/indexer.go)
  Walks the directory. For each supported file it reads the content and calls upsertDocument to insert/update the
  documents row (kind=file). It then extracts symbols via Tree-sitter and calls insertChunks.
  4. upsertDocument (internal/indexer/indexer.go)
  Issues an INSERT … ON CONFLICT DO UPDATE SET content = excluded.content into the documents table. If the file
  changed, the row's content column is updated.
  5. documents_embedding_invalidate trigger (internal/db/schema.go)
  The SQLite trigger fires AFTER UPDATE OF content ON documents WHEN OLD.content != NEW.content and executes:
  DELETE FROM embeddings WHERE doc_id = NEW.id;
  5. This is the pivotal step: it atomically removes the stale embedding for any document whose content actually
  changed.
  6. insertChunks (internal/indexer/indexer.go)
  Upserts one documents row (kind=chunk, parent_id pointing to the file row) per extracted symbol, each with the
  symbol source as content. The same trigger fires for any chunk whose content changed, deleting its embedding too.
   Chunks for symbols that no longer exist are deleted (cascade-removes their embeddings via the ON DELETE CASCADE
  FK).
  7. handleIndex calls worker.Trigger() (internal/api/handlers.go)
  After IndexDir returns, handleIndex sends a non-blocking signal to EmbedWorker.trigger channel.
  8. EmbedWorker.Run (internal/search/embedworker.go)
  The goroutine started at daemon startup receives the trigger signal (or fires on its periodic time.Ticker) and
  calls w.process(ctx) in a loop until it drains.
  9. EmbedWorker.process (internal/search/embedworker.go)
  Queries all documents rows that have no matching row in embeddings (excluding file-kind docs that have chunks,
  since those are superseded by their chunks). Finds the freshly-invalidated chunk rows and queues them.
  10. EmbedAndStore (internal/search/semantic.go)
  For each pending doc, calls embedder.Embedder.Embed(content) to produce a GloVe/FastText vector, then writes it
  to embeddings with:
  INSERT INTO embeddings (doc_id, model, vector) VALUES (?, ?, ?)
  ON CONFLICT(doc_id, model) DO UPDATE SET vector = excluded.vector
  10. The chunk is now re-embedded and available for semantic search.

  ---
  Key insight on invalidation: there is no file-system watcher. The entire flow is triggered by a re-index call.
  The stale embedding is removed synchronously by a SQLite trigger the moment the document content changes; the
  actual re-computation is asynchronous, handled by the background EmbedWorker.