package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicComplete_RequestAndResponse(t *testing.T) {
	var gotBody anthropicRequest
	var gotPath, gotAPIKey, gotVersion string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []anthropicContentBlock{{Type: "text", Text: "hello from claude"}},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider("sk-test", server.URL, "claude-sonnet-4.5")
	out, err := p.Complete(context.Background(), "you are helpful", []Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if out != "hello from claude" {
		t.Errorf("Complete() = %q, want %q", out, "hello from claude")
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotAPIKey != "sk-test" {
		t.Errorf("x-api-key = %q, want sk-test", gotAPIKey)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", gotVersion, anthropicVersion)
	}
	if gotBody.Model != "claude-sonnet-4.5" {
		t.Errorf("model = %q, want claude-sonnet-4.5", gotBody.Model)
	}
	if gotBody.System != "you are helpful" {
		t.Errorf("system = %q, want %q", gotBody.System, "you are helpful")
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Content != "hi" {
		t.Errorf("messages = %+v, want one message with content 'hi'", gotBody.Messages)
	}
}

func TestAnthropicComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "invalid api key"},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider("bad-key", server.URL, "claude-sonnet-4.5")
	_, err := p.Complete(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error for 401 response")
	}
}
