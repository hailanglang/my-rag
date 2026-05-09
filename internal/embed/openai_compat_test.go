package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompat_Embed_single(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.25,0.5,1.0]}]}`))
	}))
	defer srv.Close()

	e := NewOpenAICompat(srv.URL, "sk-test", "deepseek-embed", 0)
	vecs, err := e.Embed(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 3 {
		t.Fatalf("vecs=%v", vecs)
	}
	if vecs[0][0] != 0.25 || vecs[0][2] != 1.0 {
		t.Fatalf("values=%v", vecs[0])
	}
}

func TestOpenAICompat_Embed_reordersByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"index":1,"embedding":[0,0,2]},
			{"index":0,"embedding":[1,0,0]}
		]}`))
	}))
	defer srv.Close()

	e := NewOpenAICompat(srv.URL, "k", "m", 0)
	vecs, err := e.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 {
		t.Fatalf("len=%d", len(vecs))
	}
	if vecs[0][0] != 1 || vecs[1][2] != 2 {
		t.Fatalf("order wrong: %#v", vecs)
	}
}

func TestOpenAICompat_Embed_emptyInput(t *testing.T) {
	e := NewOpenAICompat("http://x", "k", "m", 0)
	v, err := e.Embed(context.Background(), nil)
	if err != nil || v != nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
}

func TestOpenAICompat_Embed_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusPaymentRequired)
	}))
	defer srv.Close()

	e := NewOpenAICompat(srv.URL, "k", "m", 0)
	_, err := e.Embed(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
