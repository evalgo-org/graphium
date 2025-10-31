package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// Initialize storage layer
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize agent manager
	agentManager, err := agents.NewManager(store, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize agent manager: %w", err)
	}

	// Start agent manager (auto-starts enabled agents)
	if err := agentManager.Start(); err != nil {
		return fmt.Errorf("failed to start agent manager: %w", err)
	}
	log.Println("Agent manager started")

	// Create API server
	server := api.New(cfg, store, agentManager)

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

		// Stop agent manager first
		if err := agentManager.Stop(); err != nil {
			log.Printf("Warning: agent manager shutdown error: %v", err)
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
			log.Printf("Warning: agent manager shutdown error: %v", stopErr)
		}
		return fmt.Errorf("server error: %w", err)
	}
}
