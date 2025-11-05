package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eve.evalgo.org/common"
	"github.com/spf13/cobra"

	"evalgo.org/graphium/internal/agents"
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
	// Setup structured logging
	logger := common.ServiceLogger("graphium", "1.0.0")

	// Initialize storage layer
	logger.Info("Initializing storage layer")
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	logger.WithField("database", cfg.CouchDB.Database).Info("Storage initialized")

	// Initialize agent manager
	logger.Info("Initializing agent manager")
	agentManager, err := agents.NewManager(store, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize agent manager: %w", err)
	}

	// Start agent manager (auto-starts enabled agents)
	if err := agentManager.Start(); err != nil {
		return fmt.Errorf("failed to start agent manager: %w", err)
	}
	logger.Info("Agent manager started")

	// Create API server
	logger.Info("Creating API server")
	server := api.New(cfg, store, agentManager)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	// Start server in a goroutine
	logger.WithField("port", cfg.Server.Port).Info("Starting API server")
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		logger.Warn("Shutdown signal received")

		// Stop agent manager first
		if err := agentManager.Stop(); err != nil {
			logger.WithError(err).Warn("Agent manager shutdown error")
		}

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
		// Stop agent manager on error
		if stopErr := agentManager.Stop(); stopErr != nil {
			logger.WithError(stopErr).Warn("Agent manager shutdown error")
		}
		return fmt.Errorf("server error: %w", err)
	}
}
