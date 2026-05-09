package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PurgeOlderThan deletes messages (and, via FK CASCADE, their message_citations)
// with created_at strictly older than maxAge, then removes sessions that have no
// remaining messages.
func PurgeOlderThan(ctx context.Context, db *sql.DB, maxAge time.Duration) error {
	if maxAge <= 0 {
		return fmt.Errorf("session: maxAge must be positive")
	}
	cutoff := time.Now().Add(-maxAge).UnixMilli()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE created_at < ?`, cutoff); err != nil {
		return err
	}

	// Remove sessions that no longer have any messages (empty chats).
	if _, err := tx.ExecContext(ctx, `
DELETE FROM sessions WHERE id NOT IN (SELECT DISTINCT session_id FROM messages)`); err != nil {
		return err
	}

	return tx.Commit()
}
