package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIComplete_RequestAndResponse(t *testing.T) {
	var gotBody openAIRequest
	var gotPath, gotAuth, gotReferer, gotTitle string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []struct {
				Message openAIMessage `json:"message"`
			}{{Message: openAIMessage{Role: "assistant", Content: "hello from gpt"}}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("sk-test", server.URL, "gpt-4", "https://example.com", "my-app")
	out, err := p.Complete(context.Background(), "you are helpful", []Message{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if out != "hello from gpt" {
		t.Errorf("Complete() = %q, want %q", out, "hello from gpt")
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want 'Bearer sk-test'", gotAuth)
	}
	if gotReferer != "https://example.com" {
		t.Errorf("HTTP-Referer = %q, want https://example.com", gotReferer)
	}
	if gotTitle != "my-app" {
		t.Errorf("X-Title = %q, want my-app", gotTitle)
	}
	if gotBody.Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", gotBody.Model)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("messages = %+v, want 2 (system + user)", gotBody.Messages)
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "you are helpful" {
		t.Errorf("messages[0] = %+v, want system message with our prompt", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "hi" {
		t.Errorf("messages[1] = %+v, want user message 'hi'", gotBody.Messages[1])
	}
}

func TestOpenAIComplete_NoOptionalHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HTTP-Referer") != "" || r.Header.Get("X-Title") != "" {
			t.Errorf("expected no optional headers when unset, got Referer=%q Title=%q",
				r.Header.Get("HTTP-Referer"), r.Header.Get("X-Title"))
		}
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []struct {
				Message openAIMessage `json:"message"`
			}{{Message: openAIMessage{Content: "ok"}}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("sk-test", server.URL, "gpt-4", "", "")
	if _, err := p.Complete(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
}

func TestOpenAIComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "rate limited"},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider("sk-test", server.URL, "gpt-4", "", "")
	_, err := p.Complete(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error for 429 response")
	}
}
