package llm

import "testing"

func envMap(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestNewProviderFromEnv_Anthropic(t *testing.T) {
	p, err := NewProviderFromEnv(envMap(map[string]string{
		"LLM_PROVIDER": "anthropic",
		"LLM_MODEL":    "claude-sonnet-4.5",
		"LLM_API_KEY":  "sk-test",
	}))
	if err != nil {
		t.Fatalf("NewProviderFromEnv() error = %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", p.Name())
	}
}

func TestNewProviderFromEnv_OpenAI(t *testing.T) {
	p, err := NewProviderFromEnv(envMap(map[string]string{
		"LLM_PROVIDER": "openai",
		"LLM_MODEL":    "gpt-4",
		"LLM_API_KEY":  "sk-test",
		"LLM_BASE_URL": "https://openrouter.ai/api/v1",
	}))
	if err != nil {
		t.Fatalf("NewProviderFromEnv() error = %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want openai", p.Name())
	}
}

func TestNewProviderFromEnv_MissingVars(t *testing.T) {
	cases := []map[string]string{
		{"LLM_MODEL": "m", "LLM_API_KEY": "k"},            // missing provider
		{"LLM_PROVIDER": "anthropic", "LLM_API_KEY": "k"}, // missing model
		{"LLM_PROVIDER": "anthropic", "LLM_MODEL": "m"},   // missing api key
	}
	for _, c := range cases {
		if _, err := NewProviderFromEnv(envMap(c)); err == nil {
			t.Errorf("NewProviderFromEnv(%v) error = nil, want error", c)
		}
	}
}

func TestNewProviderFromEnv_UnknownProvider(t *testing.T) {
	_, err := NewProviderFromEnv(envMap(map[string]string{
		"LLM_PROVIDER": "cohere",
		"LLM_MODEL":    "m",
		"LLM_API_KEY":  "k",
	}))
	if err == nil {
		t.Fatal("NewProviderFromEnv() error = nil, want error for unknown provider")
	}
}
