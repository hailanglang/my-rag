package documents

import (
	"context"
	"database/sql"
)

// Repository persists documents in SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a document store backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Insert inserts a new document row.
func (r *Repository) Insert(ctx context.Context, d Document) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		d.ID, d.Filename, d.Mime, d.Size, d.StoragePath, d.Status, d.CreatedAt, d.UpdatedAt)
	return err
}

// List returns all documents newest first.
func (r *Repository) List(ctx context.Context) ([]Document, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, filename, mime, size, storage_path, status, error_message, created_at, updated_at
FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		var errMsg sql.NullString
		if err := rows.Scan(&d.ID, &d.Filename, &d.Mime, &d.Size, &d.StoragePath, &d.Status, &errMsg, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			s := errMsg.String
			d.ErrorMessage = &s
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Document{}
	}
	return out, nil
}

// GetStoragePath returns the absolute storage path for a document id.
func (r *Repository) GetStoragePath(ctx context.Context, id string) (string, error) {
	var p string
	err := r.db.QueryRowContext(ctx, `SELECT storage_path FROM documents WHERE id = ?`, id).Scan(&p)
	return p, err
}

// Delete removes a document row (chunks cascade).
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
