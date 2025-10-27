package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"evalgo.org/graphium/internal/api"
	"evalgo.org/graphium/internal/storage"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Long:  `Start the HTTP API server with Echo framework`,
	RunE:  runServer,
}

func runServer(cmd *cobra.Command, args []string) error {
	// Initialize storage layer
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Create API server
	server := api.New(cfg, store)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		fmt.Println("\n⚠️  Shutdown signal received")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Duration(cfg.Server.ShutdownTimeout)*time.Second,
		)
		defer cancel()

		// Graceful shutdown
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		return nil

	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}
