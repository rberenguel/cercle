# cercle codebase — eval questions

1. What is the name of the SQL trigger that invalidates stale embeddings when a document's content is updated?
2. What delimiter characters does the FTS5 snippet() function use to mark matched text in cercle's lexical search?
3. How many documents does the embed worker fetch per batch in its SQL query (the LIMIT value)?
4. What is the default TCP port that the cercled daemon listens on?
5. What command-line flag causes cercled to wipe the database and exit immediately without starting the HTTP server?
6. What column was added to the documents table in schema migration version 3 (v3)?
7. What model ID string does the GloVe local embedder return for 100-dimensional vectors?
8. What environment variable do the cercle rlm scripts read to determine the source namespace?
9. What two separator characters appear in the chunk path format used when indexing code symbols (e.g. path::Name@line)?
10. What min_similarity value does the cercle skill documentation recommend to suppress noise in semantic search results?
11. Name two directory names that the cercle indexer skips during file tree traversal.
12. What is the name of the Go function in cercle that deletes a summary and its underlying document?
13. What is the default result limit for the GET /summaries API endpoint in cercle?
14. What macOS command must be run after building the cercled binary to allow it to use the SQLite extension?
15. What Go build tag is required when compiling cercled to enable FTS5 full-text search support?
16. What is the maximum number of concurrent goroutines the embed worker uses to embed documents in parallel?
17. What GloVe dataset (by filename prefix) does the cercle download-vectors script fetch from Stanford NLP?
18. What are the two JSON field names for the start and end line range in cercle's Symbol struct?
19. What Go module path provides the SQLite3 driver used by cercle (from go.mod)?
20. What does a `semantic: false` value in the /health response indicate about the cercled server's state?
