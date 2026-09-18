package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
	anthropicMaxTokens      = 4096
)

// AnthropicProvider talks to the Anthropic Messages API (/v1/messages).
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewAnthropicProvider builds a provider. An empty baseURL falls back to the
// public Anthropic API.
func NewAnthropicProvider(apiKey, baseURL, model string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) Complete(ctx context.Context, system string, msgs []Message) (string, error) {
	reqMsgs := make([]anthropicMessage, len(msgs))
	for i, m := range msgs {
		reqMsgs[i] = anthropicMessage{Role: m.Role, Content: m.Content}
	}
	body, err := json.Marshal(anthropicRequest{
		Model:     p.model,
		MaxTokens: anthropicMaxTokens,
		System:    system,
		Messages:  reqMsgs,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("anthropic api error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("anthropic api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var text bytes.Buffer
	for _, block := range parsed.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String(), nil
}
