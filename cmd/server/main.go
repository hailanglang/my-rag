package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"my-rag/internal/config"
	"my-rag/internal/embed"
	"my-rag/internal/httpapi"
	"my-rag/internal/llm"
	"my-rag/internal/session"
	"my-rag/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		log.Fatal(err)
	}

	go runRetentionPurge(db)

	emb := embed.NewOpenAICompat(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekEmbedModel)
	stream := llm.NewDeepSeek(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekChatModel)
	h := httpapi.NewRouter(&httpapi.RouterConfig{
		DB:                db,
		UploadDir:         cfg.UploadDir,
		Embedder:          emb,
		IndexTimeout:      cfg.IndexTimeout,
		Streamer:          stream,
		RetrieveTimeout:   cfg.RetrieveTimeout,
		LLMStreamTimeout:  cfg.LLMStreamTimeout,
	})

	log.Printf("listening %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, h); err != nil {
		log.Fatal(err)
	}
}

const retentionMessages = 30 * 24 * time.Hour

func runRetentionPurge(db *sql.DB) {
	ctx := context.Background()
	purge := func() {
		if err := session.PurgeOlderThan(ctx, db, retentionMessages); err != nil {
			log.Printf("retention purge: %v", err)
		}
	}
	purge()
	tick := time.NewTicker(time.Hour)
	for range tick.C {
		purge()
	}
}
