package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// OpenAICompat calls an OpenAI-compatible POST /v1/embeddings endpoint.
type OpenAICompat struct {
	BaseURL string // e.g. https://api.deepseek.com (no trailing slash)
	APIKey  string
	Model   string
	Client  *http.Client
}

// NewOpenAICompat returns an embedder backed by baseURL + /v1/embeddings.
func NewOpenAICompat(baseURL, apiKey, model string) *OpenAICompat {
	return &OpenAICompat{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  apiKey,
		Model:   model,
		Client:  http.DefaultClient,
	}
}

type embeddingsRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // []string or string
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Embed implements Embedder.
func (c *OpenAICompat) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if c.BaseURL == "" || c.APIKey == "" || c.Model == "" {
		return nil, fmt.Errorf("embed: BaseURL, APIKey, and Model are required")
	}
	body, err := json.Marshal(embeddingsRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	rb, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("embed: HTTP %d: %s", res.StatusCode, string(bytes.TrimSpace(rb)))
	}
	var parsed embeddingsResponse
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return nil, fmt.Errorf("embed: decode: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embed: want %d vectors, got %d", len(texts), len(parsed.Data))
	}
	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		if len(d.Embedding) == 0 {
			return nil, fmt.Errorf("embed: empty embedding at index %d", i)
		}
		v := make([]float32, len(d.Embedding))
		for j, x := range d.Embedding {
			v[j] = float32(x)
		}
		out[i] = v
	}
	return out, nil
}

var _ Embedder = (*OpenAICompat)(nil)
