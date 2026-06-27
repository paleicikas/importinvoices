package service

import (
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/processor"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

type Worker interface {
	Queue(invoiceID string)
}

type Service struct {
	store              *db.Store
	storage            *storage.Storage
	media              *media.MediaService
	Webhook            *WebhookService
	worker             Worker
	processorOverride  processor.Processor
}

func New(store *db.Store, storage *storage.Storage, media *media.MediaService) *Service {
	s := &Service{
		store:   store,
		storage: storage,
		media:   media,
	}
	s.Webhook = NewWebhookService(s)
	return s
}

func (s *Service) SetWorker(w Worker) {
	s.worker = w
}

// SetProcessorOverride replaces LLM resolution. Used by tests; pass nil to reset.
func (s *Service) SetProcessorOverride(p processor.Processor) {
	s.processorOverride = p
}

func (s *Service) Store() *db.Store {
	return s.store
}

func (s *Service) Storage() *storage.Storage {
	return s.storage
}
