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

const openAIDefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider talks to any OpenAI-compatible chat/completions endpoint.
// This covers OpenAI, OpenRouter, Groq, Together, DeepSeek, Ollama, etc. —
// they only differ by base URL and model name.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	referer string // optional, recommended by OpenRouter
	title   string // optional, recommended by OpenRouter
	client  *http.Client
}

// NewOpenAIProvider builds a provider. An empty baseURL falls back to the
// public OpenAI API. referer/title are sent as HTTP-Referer/X-Title headers
// when non-empty (OpenRouter uses these to attribute usage), and are
// harmless no-ops against any other OpenAI-compatible endpoint.
func NewOpenAIProvider(apiKey, baseURL, model, referer, title string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = openAIDefaultBaseURL
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		referer: referer,
		title:   title,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) Complete(ctx context.Context, system string, msgs []Message) (string, error) {
	reqMsgs := make([]openAIMessage, 0, len(msgs)+1)
	reqMsgs = append(reqMsgs, openAIMessage{Role: "system", Content: system})
	for _, m := range msgs {
		reqMsgs = append(reqMsgs, openAIMessage{Role: m.Role, Content: m.Content})
	}

	body, err := json.Marshal(openAIRequest{Model: p.model, Messages: reqMsgs})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.referer != "" {
		req.Header.Set("HTTP-Referer", p.referer)
	}
	if p.title != "" {
		req.Header.Set("X-Title", p.title)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("openai api error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("openai api error (%d): %s", resp.StatusCode, string(respBody))
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai response has no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
