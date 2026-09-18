// Package llm defines the LLM provider abstraction used by the agent loop.
package llm

import "context"

// Message is a single turn in the conversation sent to the provider.
type Message struct {
	Role    string // "user" | "assistant"
	Content string
}

// Provider is implemented by each backend (Anthropic, OpenAI-compatible, ...).
// The agent loop only ever talks to this interface, never to a concrete client.
type Provider interface {
	Complete(ctx context.Context, system string, msgs []Message) (string, error)
	Name() string
}
