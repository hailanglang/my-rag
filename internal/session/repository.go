package session

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Repository persists sessions, messages, and citations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a session store.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateSession inserts a new session and returns its id.
func (r *Repository) CreateSession(ctx context.Context, title string) (string, error) {
	if title == "" {
		title = "Chat"
	}
	id := uuid.New().String()
	now := time.Now().UnixMilli()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, title, now, now)
	return id, err
}

// SessionExists returns true if the session id exists.
func (r *Repository) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&n)
	return n > 0, err
}

// AddMessage inserts a user or assistant message; returns message id.
func (r *Repository) AddMessage(ctx context.Context, sessionID, role, content string) (string, error) {
	id := uuid.New().String()
	now := time.Now().UnixMilli()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, sessionID, role, content, now)
	if err != nil {
		return "", err
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, now, sessionID)
	return id, nil
}

// AddCitation inserts one citation row for an assistant message.
func (r *Repository) AddCitation(ctx context.Context, messageID, chunkID, quote, documentID string) error {
	id := uuid.New().String()
	now := time.Now().UnixMilli()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO message_citations (id, message_id, chunk_id, quote, document_id, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		id, messageID, chunkID, quote, documentID, now)
	return err
}

// ListMessages returns prior messages for a session (oldest first), excluding system.
func (r *Repository) ListMessages(ctx context.Context, sessionID string, limit int) ([]struct {
	Role    string
	Content string
}, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT role, content FROM messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Role    string
		Content string
	}
	for rows.Next() {
		var m struct {
			Role    string
			Content string
		}
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
