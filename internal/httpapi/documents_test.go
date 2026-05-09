package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-rag/internal/documents"
	"my-rag/internal/store"
)

type fakeEmb struct{}

func (fakeEmb) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i + 1), 0}
	}
	return out, nil
}

func TestDocuments_upload_list_delete(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(dir, "uploads")
	mux := NewRouter(&RouterConfig{
		DB: db, UploadDir: uploadDir, Embedder: fakeEmb{}, IndexTimeout: time.Minute,
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(fw, "hello rag"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/api/documents", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST status=%d", res.StatusCode)
	}
	var created documents.Document
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != "pending" {
		t.Fatalf("bad create: %+v", created)
	}

	// wait for async index
	for i := 0; i < 100; i++ {
		res, err := http.Get(srv.URL + "/api/documents")
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			t.Fatalf("GET status=%d", res.StatusCode)
		}
		var list []documents.Document
		if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
			res.Body.Close()
			t.Fatal(err)
		}
		res.Body.Close()
		if len(list) == 1 && list[0].Status == "ready" {
			break
		}
		if i == 99 {
			t.Fatalf("index did not finish: %#v", list)
		}
		time.Sleep(10 * time.Millisecond)
	}

	var chunkCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE document_id = ?`, created.ID).Scan(&chunkCount); err != nil {
		t.Fatal(err)
	}
	if chunkCount < 1 {
		t.Fatalf("chunks=%d", chunkCount)
	}

	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, srv.URL+"/api/documents/"+created.ID, nil)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status=%d", delRes.StatusCode)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&chunkCount); err != nil {
		t.Fatal(err)
	}
	if chunkCount != 0 {
		t.Fatalf("chunks after delete=%d", chunkCount)
	}

	if _, err := os.Stat(filepath.Join(uploadDir, created.ID+".txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
}
