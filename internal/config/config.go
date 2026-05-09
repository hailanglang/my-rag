package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration from the environment.
type Config struct {
	HTTPAddr           string
	DatabasePath       string
	UploadDir          string
	DeepSeekBaseURL    string
	DeepSeekAPIKey     string
	DeepSeekChatModel  string
	DeepSeekEmbedModel string
	RetrieveTimeout    time.Duration
	LLMStreamTimeout   time.Duration
	IndexTimeout       time.Duration
}

// Load reads configuration from environment variables.
// DEEPSEEK_API_KEY is required.
func Load() (*Config, error) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is required")
	}

	c := &Config{
		HTTPAddr:           getenvDefault("HTTP_ADDR", ":8080"),
		DatabasePath:       getenvDefault("DATABASE_PATH", "./data/app.db"),
		UploadDir:          getenvDefault("UPLOAD_DIR", "./data/uploads"),
		DeepSeekBaseURL:    getenvDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekAPIKey:     key,
		DeepSeekChatModel:  getenvDefault("DEEPSEEK_CHAT_MODEL", "deepseek-chat"),
		DeepSeekEmbedModel: getenvDefault("DEEPSEEK_EMBED_MODEL", "deepseek-embedding"),
		RetrieveTimeout:    durationEnv("RETRIEVE_TIMEOUT_SEC", 10*time.Second),
		LLMStreamTimeout:   durationEnv("LLM_STREAM_TIMEOUT_SEC", 180*time.Second),
		IndexTimeout:       durationEnv("INDEX_TIMEOUT_SEC", 5*time.Minute),
	}
	return c, nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec <= 0 {
		return def
	}
	return time.Duration(sec) * time.Second
}
