package llm

import "fmt"

// Config holds everything needed to build a Provider, sourced from env vars.
type Config struct {
	Provider    string // "anthropic" | "openai"
	BaseURL     string // optional, overrides the provider's default
	Model       string
	APIKey      string
	HTTPReferer string // optional, OpenRouter attribution header
	Title       string // optional, OpenRouter attribution header
}

// NewProviderFromEnv builds a Config from env vars via getenv (os.Getenv in
// production, a fake map in tests) and returns the matching Provider.
func NewProviderFromEnv(getenv func(string) string) (Provider, error) {
	cfg := Config{
		Provider:    getenv("LLM_PROVIDER"),
		BaseURL:     getenv("LLM_BASE_URL"),
		Model:       getenv("LLM_MODEL"),
		APIKey:      getenv("LLM_API_KEY"),
		HTTPReferer: getenv("LLM_HTTP_REFERER"),
		Title:       getenv("LLM_TITLE"),
	}
	return NewProvider(cfg)
}

// NewProvider validates cfg and returns the Provider it describes.
func NewProvider(cfg Config) (Provider, error) {
	if cfg.Provider == "" {
		return nil, fmt.Errorf("LLM_PROVIDER is required (\"anthropic\" or \"openai\")")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("LLM_MODEL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY is required")
	}

	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg.APIKey, cfg.BaseURL, cfg.Model), nil
	case "openai":
		return NewOpenAIProvider(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.HTTPReferer, cfg.Title), nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (want \"anthropic\" or \"openai\")", cfg.Provider)
	}
}
