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
}

func TestLoad_MissingAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}
