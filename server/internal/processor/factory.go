package processor

import (
	"fmt"
)

func New(provider, apiKey, model string) (Processor, error) {
	switch provider {
	case "openai":
		return NewOpenAIProcessor(apiKey, model), nil
	case "google":
		return NewGeminiProcessor(apiKey, model)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
