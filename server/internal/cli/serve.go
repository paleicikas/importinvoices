package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/config"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/httpapi"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
	"github.com/paleicikas/importinvoices/server/internal/webui"
	"github.com/paleicikas/importinvoices/server/internal/worker"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the importinvoices server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Resolve(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve config: %w", err)
		}

		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer func() { _ = store.Close() }()

		if err := store.Migrate(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		strg, err := storage.New(cfg.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		mediaSvc := media.New(filepath.Join(cfg.DataDir, "temp"))
		svc := service.New(store, strg, mediaSvc)
		w := worker.New(store, svc, mediaSvc)
		svc.SetWorker(w)

		if err := svc.SeedExportTemplates(cmd.Context()); err != nil {
			return fmt.Errorf("failed to seed export templates: %w", err)
		}

		if err := svc.CleanupExpiredSessions(cmd.Context()); err != nil {
			return fmt.Errorf("failed to clean up expired sessions: %w", err)
		}

		render, err := webui.NewRenderer()
		if err != nil {
			return fmt.Errorf("failed to initialize renderer: %w", err)
		}
		server := httpapi.NewServer(svc, render, cfg.StoragePath, cfg.MaxUploadBytes, cfg.TrustedProxies)

		srv := &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: server.Router(),
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go w.Start(ctx)

		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := svc.CleanupExpiredSessions(context.Background()); err != nil {
						fmt.Fprintf(os.Stderr, "session cleanup error: %v\n", err)
					}
				}
			}
		}()

		go func() {
			fmt.Printf("Starting importinvoices server on %s\n", cfg.HTTPAddr)
			fmt.Printf("Data directory: %s\n", cfg.DataDir)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			}
		}()

		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
