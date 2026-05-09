package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DeepSeek streams OpenAI-compatible chat completions from baseURL.
type DeepSeek struct {
	BaseURL string // no trailing slash, e.g. https://api.deepseek.com
	APIKey  string
	Model   string
	Client  *http.Client
}

// NewDeepSeek returns a Streamer for DeepSeek (or any OpenAI-compatible host).
func NewDeepSeek(baseURL, apiKey, model string) *DeepSeek {
	return &DeepSeek{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  apiKey,
		Model:   model,
		Client:  http.DefaultClient,
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Stream   bool      `json:"stream"`
	Messages []Message `json:"messages"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// StreamChat implements Streamer.
func (d *DeepSeek) StreamChat(ctx context.Context, messages []Message, onDelta func(text string) error) error {
	if d.BaseURL == "" || d.APIKey == "" || d.Model == "" {
		return fmt.Errorf("llm: BaseURL, APIKey, and Model are required")
	}
	body, err := json.Marshal(chatRequest{Model: d.Model, Stream: true, Messages: messages})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+d.APIKey)

	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		rb, _ := io.ReadAll(res.Body)
		return fmt.Errorf("llm: HTTP %d: %s", res.StatusCode, string(bytes.TrimSpace(rb)))
	}

	sc := bufio.NewScanner(res.Body)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return nil
		}
		var ch streamChunk
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			return fmt.Errorf("llm: decode chunk: %w", err)
		}
		if len(ch.Choices) == 0 {
			continue
		}
		text := ch.Choices[0].Delta.Content
		if text == "" {
			continue
		}
		if err := onDelta(text); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return err
	}
	return nil
}

var _ Streamer = (*DeepSeek)(nil)
