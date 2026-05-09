package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"my-rag/internal/embed"
	"my-rag/internal/llm"
	"my-rag/internal/rag"
	"my-rag/internal/session"
)

// cancelEntry identifies one in-flight generation per session (CancelFunc is not comparable for sync.Map).
type cancelEntry struct {
	gen    string
	cancel context.CancelFunc
}

type chatHandlers struct {
	db              *sql.DB
	sessions        *session.Repository
	querier         *rag.Querier
	streamer        llm.Streamer
	retrieveTimeout time.Duration
	llmTimeout      time.Duration
	inflightCancels sync.Map // sessionID -> *cancelEntry
}

func newChatHandlers(db *sql.DB, emb embed.Embedder, stream llm.Streamer, retrieveTimeout, llmTimeout time.Duration) *chatHandlers {
	return &chatHandlers{
		db:              db,
		sessions:        session.NewRepository(db),
		querier:         rag.NewQuerier(db, emb),
		streamer:        stream,
		retrieveTimeout: retrieveTimeout,
		llmTimeout:      llmTimeout,
	}
}

func (ch *chatHandlers) createSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in)
	}
	id, err := ch.sessions.CreateSession(r.Context(), strings.TrimSpace(in.Title))
	if err != nil {
		writeAPIError(w, "SESSION_CREATE_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (ch *chatHandlers) postMessage(w http.ResponseWriter, r *http.Request) {
	var err error
	sid := r.PathValue("id")
	if sid == "" {
		writeAPIError(w, "BAD_REQUEST", "missing session id", http.StatusBadRequest)
		return
	}
	var ok bool
	ok, err = ch.sessions.SessionExists(r.Context(), sid)
	if err != nil {
		writeAPIError(w, "SESSION_LOOKUP_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		writeAPIError(w, "NOT_FOUND", "session not found", http.StatusNotFound)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeAPIError(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	userContent := strings.TrimSpace(body.Content)
	if userContent == "" {
		writeAPIError(w, "BAD_REQUEST", "empty content", http.StatusBadRequest)
		return
	}

	rid := r.Header.Get("X-Request-ID")
	if rid == "" {
		rid = uuid.New().String()
	}
	log.Printf("request_id=%s chat content_len=%d", rid, len(userContent))

	if _, err = ch.sessions.AddMessage(r.Context(), sid, "user", userContent); err != nil {
		writeAPIError(w, "MESSAGE_SAVE_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	if ch.llmTimeout > 0 {
		var cancelTO context.CancelFunc
		ctx, cancelTO = context.WithTimeout(ctx, ch.llmTimeout)
		defer cancelTO()
	}

	if v, ok := ch.inflightCancels.Load(sid); ok {
		if old, ok := v.(*cancelEntry); ok && old != nil {
			old.cancel()
		}
	}
	ctx, cancelRun := context.WithCancel(ctx)
	gen := uuid.New().String()
	ent := &cancelEntry{gen: gen, cancel: cancelRun}
	ch.inflightCancels.Store(sid, ent)
	defer func() {
		if v, ok := ch.inflightCancels.Load(sid); ok {
			if cur, ok := v.(*cancelEntry); ok && cur != nil && cur.gen == gen {
				ch.inflightCancels.Delete(sid)
			}
		}
		cancelRun()
	}()

	var block string
	var cites []rag.Citation
	if ch.retrieveTimeout > 0 {
		qctx, qc := context.WithTimeout(ctx, ch.retrieveTimeout)
		block, cites, err = ch.querier.BuildContext(qctx, userContent)
		qc()
	} else {
		block, cites, err = ch.querier.BuildContext(ctx, userContent)
	}
	if err != nil {
		// Degrade to no retrieval (empty context) so chat still streams when embed/upstream
		// is misconfigured or unavailable; avoids hard 500 for the whole POST.
		log.Printf("request_id=%s rag_build_context: %v", rid, err)
		block, cites = "", nil
	}
	if cites == nil {
		cites = []rag.Citation{}
	}

	history, err := ch.sessions.ListMessages(ctx, sid, 50)
	if err != nil {
		writeAPIError(w, "HISTORY_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}

	system := "You are a helpful assistant. Use the following context when it is relevant.\n\n" + block
	msgs := []llm.Message{{Role: "system", Content: system}}
	for _, m := range history {
		msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
	}

	fl, okFl := w.(http.Flusher)
	if !okFl {
		writeAPIError(w, "STREAM_UNSUPPORTED", "response writer cannot flush", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	citeJSON, _ := json.Marshal(cites)
	if err = writeSSE(w, fl, "citations", string(citeJSON)); err != nil {
		return
	}

	var assistant strings.Builder
	err = ch.streamer.StreamChat(ctx, msgs, func(text string) error {
		if text == "" {
			return nil
		}
		assistant.WriteString(text)
		line, _ := json.Marshal(map[string]string{"t": text})
		return writeSSE(w, fl, "token", string(line))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			_ = writeSSE(w, fl, "error", `{"code":"CANCELED","message":"request canceled"}`)
			return
		}
		em, _ := json.Marshal(map[string]string{"code": "LLM_ERROR", "message": err.Error()})
		_ = writeSSE(w, fl, "error", string(em))
		return
	}

	var aid string
	aid, err = ch.sessions.AddMessage(ctx, sid, "assistant", assistant.String())
	if err != nil {
		em, _ := json.Marshal(map[string]string{"code": "SAVE_FAILED", "message": err.Error()})
		_ = writeSSE(w, fl, "error", string(em))
		return
	}
	for _, c := range cites {
		if cerr := ch.sessions.AddCitation(ctx, aid, c.ChunkID, c.Quote, c.DocumentID); cerr != nil {
			log.Printf("request_id=%s citation_save_err=%v", rid, cerr)
		}
	}
	_ = writeSSE(w, fl, "done", "{}")
}

func (ch *chatHandlers) abortSession(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	if sid == "" {
		writeAPIError(w, "BAD_REQUEST", "missing session id", http.StatusBadRequest)
		return
	}
	if v, ok := ch.inflightCancels.LoadAndDelete(sid); ok {
		if ent, ok := v.(*cancelEntry); ok && ent != nil {
			ent.cancel()
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeSSE(w http.ResponseWriter, fl http.Flusher, event, data string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if fl != nil {
		fl.Flush()
	}
	return err
}
