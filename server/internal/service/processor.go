package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/processor"
)

func (s *Service) GetProcessor(ctx context.Context) (processor.Processor, error) {
	if s.processorOverride != nil {
		return s.processorOverride, nil
	}

	provider, _ := s.GetSetting(ctx, "llm_provider")
	if provider == "" {
		provider = "openai"
	}
	
	apiKey, _ := s.GetSetting(ctx, provider+"_api_key")
	if apiKey == "" {
		// fallback to env for dev
		apiKey = os.Getenv(strings.ToUpper(provider) + "_API_KEY")
	}
	
	if apiKey == "" {
		return nil, fmt.Errorf("API key for %s not configured", provider)
	}

	model, _ := s.GetSetting(ctx, provider+"_model")
	
	return processor.New(provider, apiKey, model)
}

func (s *Service) IsLLMConfigured(ctx context.Context) (bool, error) {
	provider, _ := s.GetSetting(ctx, "llm_provider")
	if provider == "" {
		provider = "openai"
	}

	apiKey, _ := s.GetSetting(ctx, provider+"_api_key")
	if apiKey == "" {
		// fallback to env for dev
		apiKey = os.Getenv(strings.ToUpper(provider) + "_API_KEY")
	}

	return apiKey != "", nil
}
