package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migrate applies the schema and any incremental migrations.
// Safe to call on an existing database.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return runAlterations(db)
}

// runAlterations applies additive changes to existing tables.
// Each alteration is idempotent — duplicate column errors are ignored.
func runAlterations(db *sql.DB) error {
	alterations := []string{
		// v2: source tag for agent/project namespacing
		`ALTER TABLE documents ADD COLUMN source TEXT`,
		// v3: parent_id links chunk documents back to their parent file
		`ALTER TABLE documents ADD COLUMN parent_id INTEGER REFERENCES documents(id) ON DELETE CASCADE`,
	}
	for _, stmt := range alterations {
		if _, err := db.Exec(stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return fmt.Errorf("alteration %q: %w", stmt, err)
		}
	}
	idempotent := []string{
		`CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_parent ON documents(parent_id)`,
	}
	for _, stmt := range idempotent {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

const schema = `
-- Core document store.
-- Every file or text chunk ingested gets a row here.
CREATE TABLE IF NOT EXISTS documents (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    path        TEXT    NOT NULL,          -- absolute or relative path / URL
    kind        TEXT    NOT NULL DEFAULT 'file', -- 'file' | 'summary' | 'chunk'
    content     TEXT    NOT NULL,
    lang        TEXT,                      -- detected language (go, python, …)
    source      TEXT,                      -- agent/project identifier (e.g. $PWD)
    indexed_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_path ON documents(path);

-- FTS5 virtual table for fast lexical (full-text) search.
-- Content= keeps this in sync with documents without duplicating text.
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    path,
    content,
    content='documents',
    content_rowid='id',
    tokenize='porter ascii'
);

-- Triggers to keep FTS5 in sync with documents.
CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, path, content)
    VALUES (new.id, new.path, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, path, content)
    VALUES ('delete', old.id, old.path, old.content);
    INSERT INTO documents_fts(rowid, path, content)
    VALUES (new.id, new.path, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, path, content)
    VALUES ('delete', old.id, old.path, old.content);
END;

-- When a document's content changes on re-index, invalidate its embeddings so
-- the embed worker re-processes it. The WHEN clause ensures no-op re-indexes
-- (same content) do not trigger unnecessary re-embedding.
CREATE TRIGGER IF NOT EXISTS documents_embedding_invalidate
    AFTER UPDATE OF content ON documents
    WHEN OLD.content != NEW.content
BEGIN
    DELETE FROM embeddings WHERE doc_id = NEW.id;
END;

-- Structural symbols extracted by Tree-sitter.
CREATE TABLE IF NOT EXISTS symbols (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id      INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL, -- 'function' | 'class' | 'method' | 'import' …
    name        TEXT    NOT NULL,
    signature   TEXT,             -- full signature / declaration text
    start_line  INTEGER,
    end_line    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_doc  ON symbols(doc_id);

-- Dense vector embeddings for semantic search.
-- Stored as raw BLOB (IEEE 754 float32, little-endian, packed).
-- Dim varies by model: 100 or 300 for GloVe/FastText. The blob length is self-describing.
CREATE TABLE IF NOT EXISTS embeddings (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id  INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    model   TEXT    NOT NULL,  -- embedding model identifier
    vector  BLOB    NOT NULL   -- float32[] packed bytes
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_embeddings_doc_model ON embeddings(doc_id, model);

-- Agent-submitted summaries (write-back path from the RLM agent).
-- Summaries are also inserted as documents (kind='summary') so they
-- participate in both FTS5 and embedding search automatically.
CREATE TABLE IF NOT EXISTS summaries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_id     INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tags       TEXT,            -- comma-separated tags
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
`
