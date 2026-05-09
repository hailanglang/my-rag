package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"my-rag/internal/config"
	"my-rag/internal/embed"
	"my-rag/internal/httpapi"
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

	emb := embed.NewOpenAICompat(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekEmbedModel)
	h := httpapi.NewRouter(&httpapi.RouterConfig{
		DB:           db,
		UploadDir:    cfg.UploadDir,
		Embedder:     emb,
		IndexTimeout: cfg.IndexTimeout,
	})

	log.Printf("listening %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, h); err != nil {
		log.Fatal(err)
	}
}
