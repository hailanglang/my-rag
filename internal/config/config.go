package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration from the environment.
type Config struct {
	HTTPAddr          string
	DatabasePath      string
	UploadDir         string
	DeepSeekBaseURL   string
	DeepSeekAPIKey    string
	DeepSeekChatModel string
	// Embed* use OpenAI-compatible /v1/embeddings (default: Aliyun Model Studio / DashScope, text-embedding-v4).
	EmbedBaseURL     string
	EmbedAPIKey      string
	EmbedModel       string
	EmbedDimensions  int // 0 = omit dimensions in JSON; >0 sent as "dimensions" (e.g. 1024 for text-embedding-v4)
	RetrieveTimeout  time.Duration
	LLMStreamTimeout time.Duration
	IndexTimeout     time.Duration
}

// Load reads configuration from environment variables.
// DEEPSEEK_API_KEY is required.
func Load() (*Config, error) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is required")
	}

	embedKey := firstNonEmpty(os.Getenv("DASHSCOPE_API_KEY"), os.Getenv("EMBED_API_KEY"), key)

	c := &Config{
		HTTPAddr:          getenvDefault("HTTP_ADDR", ":8080"),
		DatabasePath:      getenvDefault("DATABASE_PATH", "./data/app.db"),
		UploadDir:         getenvDefault("UPLOAD_DIR", "./data/uploads"),
		DeepSeekBaseURL:   getenvDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekAPIKey:    key,
		DeepSeekChatModel: getenvDefault("DEEPSEEK_CHAT_MODEL", "deepseek-chat"),
		EmbedBaseURL: getenvDefault(
			"EMBED_BASE_URL",
			"https://dashscope.aliyuncs.com/compatible-mode/v1",
		),
		EmbedAPIKey:      embedKey,
		EmbedModel:       getenvDefault("EMBED_MODEL", "text-embedding-v4"),
		EmbedDimensions:  embedDimensionsFromEnv(),
		RetrieveTimeout:  durationEnv("RETRIEVE_TIMEOUT_SEC", 10*time.Second),
		LLMStreamTimeout: durationEnv("LLM_STREAM_TIMEOUT_SEC", 180*time.Second),
		IndexTimeout:     durationEnv("INDEX_TIMEOUT_SEC", 5*time.Minute),
	}
	return c, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// embedDimensionsFromEnv returns dimensions to send to OpenAI-compatible embeddings.
// Empty env → 1024 (Aliyun text-embedding-v4 default per docs). "0" → omit (use provider default).
func embedDimensionsFromEnv() int {
	s := strings.TrimSpace(os.Getenv("EMBED_DIMENSIONS"))
	if s == "" {
		return 1024
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 1024
	}
	return n
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
