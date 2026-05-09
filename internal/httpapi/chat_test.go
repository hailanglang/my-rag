package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"my-rag/internal/llm"
	"my-rag/internal/store"
	"my-rag/internal/vector"
)

type steerEmb struct {
	vec []float32
}

func (s steerEmb) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = append([]float32(nil), s.vec...)
	}
	return out, nil
}

func TestChat_SSE_citationsAndTokens(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	_, err = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'a.txt', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	vs := vector.NewStore(db)
	if err := vs.UpsertChunk(ctx, "c1", "d1", 0, "chunk body", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}

	uploadDir := filepath.Join(dir, "up")
	mux := NewRouter(&RouterConfig{
		DB:               db,
		UploadDir:        uploadDir,
		Embedder:         steerEmb{vec: []float32{1, 0}},
		IndexTimeout:     time.Minute,
		Streamer:         okStreamChat{},
		RetrieveTimeout:  5 * time.Second,
		LLMStreamTimeout: time.Minute,
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/api/sessions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create session %d", res.StatusCode)
	}
	var sess struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&sess); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/api/sessions/"+sess.ID+"/messages", strings.NewReader(`{"content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	msgRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer msgRes.Body.Close()
	if msgRes.StatusCode != http.StatusOK {
		t.Fatalf("message status=%d", msgRes.StatusCode)
	}
	if ct := msgRes.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type=%q", ct)
	}
	body, err := io.ReadAll(msgRes.Body)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "event: citations") {
		t.Fatalf("missing citations: %q", s)
	}
	if !strings.Contains(s, "event: token") {
		t.Fatalf("missing token: %q", s)
	}
}

type okStreamChat struct{}

func (okStreamChat) StreamChat(ctx context.Context, msgs []llm.Message, onDelta func(string) error) error {
	return onDelta("x")
}

type slowStream struct{}

func (slowStream) StreamChat(ctx context.Context, msgs []llm.Message, onDelta func(string) error) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestChat_clientCancel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "c2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	_, _ = db.ExecContext(ctx, `
INSERT INTO documents (id, filename, mime, size, storage_path, status, error_message, created_at, updated_at)
VALUES ('d1', 'a.txt', 'text/plain', 1, '/x', 'ready', NULL, ?, ?)`, now, now)
	vs := vector.NewStore(db)
	_ = vs.UpsertChunk(ctx, "c1", "d1", 0, "x", []float32{1, 0})

	_, _ = db.ExecContext(ctx, `INSERT INTO sessions (id, title, created_at, updated_at) VALUES ('sid', 't', ?, ?)`, now, now)

	uploadDir := filepath.Join(dir, "up")
	mux := NewRouter(&RouterConfig{
		DB:               db,
		UploadDir:        uploadDir,
		Embedder:         steerEmb{vec: []float32{1, 0}},
		IndexTimeout:     time.Minute,
		Streamer:         slowStream{},
		RetrieveTimeout:  time.Second,
		LLMStreamTimeout: time.Minute,
	})

	reqCtx, cancel := context.WithCancel(ctx)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	req := httptest.NewRequestWithContext(reqCtx, http.MethodPost, "http://x/api/sessions/sid/messages", strings.NewReader(`{"content":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CANCELED") {
		t.Fatalf("expected cancel marker, body=%q", rec.Body.String())
	}
}
