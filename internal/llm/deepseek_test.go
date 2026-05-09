package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeepSeek_StreamChat_accumulates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
		fl.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		fl.Flush()
	}))
	defer srv.Close()

	d := NewDeepSeek(srv.URL, "sk", "deepseek-chat")
	var b strings.Builder
	err := d.StreamChat(context.Background(), []Message{{Role: "user", Content: "x"}}, func(text string) error {
		b.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != "Hi" {
		t.Fatalf("got %q", b.String())
	}
}

func TestDeepSeek_StreamChat_contextCancel(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"H\"}}]}\n\n"))
		fl.Flush()
		close(block)
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	d := NewDeepSeek(srv.URL, "sk", "m")

	var got string
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.StreamChat(ctx, []Message{{Role: "user", Content: "x"}}, func(text string) error {
			got += text
			cancel()
			return nil
		})
	}()

	<-block
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel-related error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for StreamChat")
	}
	if got != "H" {
		t.Fatalf("got %q", got)
	}
}
