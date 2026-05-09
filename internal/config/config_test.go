package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr=%q", c.HTTPAddr)
	}
	if c.RetrieveTimeout != 10*time.Second {
		t.Fatalf("RetrieveTimeout=%v", c.RetrieveTimeout)
	}
	if c.LLMStreamTimeout != 180*time.Second {
		t.Fatalf("LLMStreamTimeout=%v", c.LLMStreamTimeout)
	}
	if c.IndexTimeout != 5*time.Minute {
		t.Fatalf("IndexTimeout=%v", c.IndexTimeout)
	}
	if c.EmbedBaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("EmbedBaseURL=%q", c.EmbedBaseURL)
	}
	if c.EmbedModel != "text-embedding-v4" {
		t.Fatalf("EmbedModel=%q", c.EmbedModel)
	}
	if c.EmbedDimensions != 1024 {
		t.Fatalf("EmbedDimensions=%d", c.EmbedDimensions)
	}
	if c.EmbedAPIKey != "sk-test" {
		t.Fatalf("EmbedAPIKey=%q", c.EmbedAPIKey)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_EmbedKeyPrefersDashScope(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deep")
	t.Setenv("DASHSCOPE_API_KEY", "sk-dash")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.EmbedAPIKey != "sk-dash" {
		t.Fatalf("EmbedAPIKey=%q want sk-dash", c.EmbedAPIKey)
	}
}
