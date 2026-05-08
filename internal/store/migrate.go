package store

import (
	"database/sql"
)

const migrateSQL = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	filename TEXT NOT NULL,
	mime TEXT,
	size INTEGER NOT NULL,
	storage_path TEXT NOT NULL,
	status TEXT NOT NULL,
	error_message TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	title TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
	id TEXT PRIMARY KEY,
	document_id TEXT NOT NULL,
	chunk_index INTEGER NOT NULL,
	text TEXT NOT NULL,
	embedding BLOB,
	created_at INTEGER NOT NULL,
	FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS message_citations (
	id TEXT PRIMARY KEY,
	message_id TEXT NOT NULL,
	chunk_id TEXT NOT NULL,
	quote TEXT,
	document_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
	FOREIGN KEY (chunk_id) REFERENCES chunks(id) ON DELETE CASCADE,
	FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_message_citations_message ON message_citations(message_id);
`

// Migrate applies the schema to db.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(migrateSQL)
	return err
}
