package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Open opens (or creates) the SQLite database with required pragmas.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	return db, db.Ping()
}
