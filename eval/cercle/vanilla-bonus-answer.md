Here is the full sequence:

  ---
  1. External trigger — POST /index (HTTP handler handleIndex)

  A caller (e.g. the rlm-index shell script) sends POST /index with {"path": "<dir>"}. The Server.handleIndex
  handler in internal/api/handlers.go receives it.

  2. Walk & upsert — indexer.IndexDir

  handleIndex calls indexer.IndexDir (internal/indexer/indexer.go). It walks the directory tree and, for each
  supported file, reads the file off disk and calls upsertDocument. The upsert uses ON CONFLICT(path) DO UPDATE
  SET content = ..., indexed_at = unixepoch(), so a modified file's new content replaces the old row in the
  documents table.

  3. Symbol extraction & chunk upsert — insertChunks

  For files with a recognised language, IndexDir calls extractSymbols (tree-sitter parse), then insertChunks. For
  each symbol, insertChunks upserts a kind='chunk' row whose path is <file>::<symbol>@<line>. The ON CONFLICT DO
  UPDATE SET content = ... clause overwrites the chunk's content, and crucially does not touch the embeddings
  table — so the existing embedding row for that doc_id is now stale. Chunks that no longer exist are deleted
  (along with their embeddings rows, via ON DELETE CASCADE).

  4. Stale embedding detection — the "pending queue" query

  The embeddings table is keyed on (doc_id, model). After the upsert, the chunk document still has its old
  embedding. There is no automatic invalidation; the pending-queue query in EmbedWorker.process selects documents
  that have no embedding row at all:

  WHERE NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.doc_id = d.id AND e.model = ?)

  So a chunk whose content changed but whose embedding row survived is not re-embedded automatically by the
  background worker in the current design. However, see step 5 for the path that does cause re-embedding.

  Edge case — re-embedding does happen when a chunk is deleted and recreated. If a symbol is renamed or its line
  number changes, insertChunks deletes the old chunk row (cascading to embeddings) and inserts a fresh one. That
  new chunk has no embedding, so it falls into the pending queue.

  5. Worker wake-up — EmbedWorker.Trigger

  Back in handleIndex, after IndexDir returns, s.worker.Trigger() sends to the trigger channel (non-blocking,
  coalesces rapid calls).

  6. Batch embedding — EmbedWorker.process → EmbedAndStore

  EmbedWorker.Run (running in a background goroutine started at daemon boot in cmd/cercled/main.go) receives the
  trigger. It calls process, which queries the pending queue, collects up to 100 document IDs/contents, fans them
  out across 4 goroutines (workerConcurrency), and each goroutine calls search.EmbedAndStore.

  7. Vector generation & storage — EmbedAndStore + Embedder

  EmbedAndStore (internal/search/semantic.go) calls emb.Embed(text) on the embedder.Embedder (a GloVe in-process
  model), converts the result to a little-endian float32 BLOB, and does:

  INSERT INTO embeddings (doc_id, model, vector) VALUES (?, ?, ?)
  ON CONFLICT(doc_id, model) DO UPDATE SET vector = excluded.vector

  The chunk document now has a fresh embedding and will be returned by future semantic searches.

  ---
  Components involved, in order:

  ┌─────┬────────────────────────────────────────────┬────────────────────────────────────────────────────────┐
  │  #  │                 Component                  │                          Role                          │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 1   │ rlm-index / HTTP client                    │ Triggers re-index via POST /index                      │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 2   │ Server.handleIndex                         │ Dispatches to indexer, fires trigger                   │
  │     │ (internal/api/handlers.go)                 │                                                        │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 3   │ indexer.IndexDir                           │ Walks disk, upserts file document                      │
  │     │ (internal/indexer/indexer.go)              │                                                        │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 4   │ indexer.insertChunks                       │ Upserts/prunes chunk documents in documents table      │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 5   │ SQLite documents table                     │ Persists updated content; embeddings row survives      │
  │     │                                            │ (stale) or is cascade-deleted (new chunk)              │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 6   │ EmbedWorker.Trigger                        │ Wakes the background worker                            │
  │     │ (internal/search/embedworker.go)           │                                                        │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 7   │ EmbedWorker.process                        │ Queries pending queue, batches work                    │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 8   │ search.EmbedAndStore                       │ Calls embedder, upserts into embeddings                │
  │     │ (internal/search/semantic.go)              │                                                        │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 9   │ embedder.Embedder                          │ Computes the GloVe vector for the chunk text           │
  ├─────┼────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ 10  │ SQLite embeddings table                    │ Stores the new vector, keyed by (doc_id, model)        │
  └─────┴────────────────────────────────────────────┴────────────────────────────────────────────────────────┘

  Important nuance: The background worker only re-embeds chunks whose embeddings row was deleted (because the
  chunk path changed — symbol renamed, line number shifted, or chunk removed). A chunk whose content changed but
  whose path stayed the same retains its stale embedding until an explicit DELETE + re-insert cycle occurs or the
  POST /embed endpoint is called manually.