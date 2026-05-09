package llm

import "context"

// Message is one chat turn for the completions API.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Streamer streams chat completion tokens from a provider.
type Streamer interface {
	StreamChat(ctx context.Context, messages []Message, onDelta func(text string) error) error
}
