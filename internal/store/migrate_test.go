package store

import (
	"path/filepath"
	"testing"
)

func TestMigrate_createsDocumentsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='documents'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("documents table missing, count=%d", n)
	}

	// sanity: sessions and chunks exist
	for _, tbl := range []string{"sessions", "chunks", "messages", "message_citations"} {
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("table %s missing", tbl)
		}
	}
}
