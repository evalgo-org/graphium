package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/gorilla/mux"
)

// startHTTPServer starts the agent's HTTP server for direct communication.
// This allows the Graphium server to directly request operations from the agent
// rather than using the task polling system.
func (a *Agent) startHTTPServer(ctx context.Context, port int) error {
	if port == 0 {
		// HTTP server disabled
		return nil
	}

	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", a.handleHealth).Methods("GET")

	// Container logs endpoint
	router.HandleFunc("/containers/{id}/logs", a.handleContainerLogs).Methods("GET")

	// Container inspect endpoint
	router.HandleFunc("/containers/{id}", a.handleContainerInspect).Methods("GET")

	// List containers endpoint
	router.HandleFunc("/containers", a.handleListContainers).Methods("GET")

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting agent HTTP server on port %d", port)

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Shutdown handler
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	return nil
}

// handleHealth returns agent health status
func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(a.startTime)

	response := map[string]interface{}{
		"status":           "healthy",
		"hostId":           a.hostID,
		"datacenter":       a.datacenter,
		"uptime":           uptime.Seconds(),
		"syncCount":        a.syncCount,
		"failedSyncs":      a.failedSyncs,
		"eventsCount":      a.eventsCount,
		"lastSync":         a.lastSyncTime,
		"lastSyncDuration": a.lastSyncDuration.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleContainerLogs streams container logs
func (a *Agent) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	if containerID == "" {
		http.Error(w, "Container ID is required", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	follow := r.URL.Query().Get("follow") == "true"
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	since := r.URL.Query().Get("since")
	timestamps := r.URL.Query().Get("timestamps") != "false" // default true

	// Create context with timeout for non-streaming requests
	ctx := r.Context()
	if !follow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Fetch logs from Docker
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: timestamps,
		Follow:     follow,
		Tail:       tail,
	}

	if since != "" {
		logOptions.Since = since
	}

	logs, err := a.docker.ContainerLogs(ctx, containerID, logOptions)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch logs: %v", err), http.StatusInternalServerError)
		return
	}
	defer logs.Close()

	// Set response headers
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Container-ID", containerID)
	w.Header().Set("X-Host-ID", a.hostID)

	if follow {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	}

	// Stream logs to response
	flusher, ok := w.(http.Flusher)
	if !ok && follow {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 8192)
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			// Docker logs include 8-byte headers, strip them
			if n > 8 {
				if _, writeErr := w.Write(buf[8:n]); writeErr != nil {
					return
				}
				if follow && flusher != nil {
					flusher.Flush()
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading logs: %v", err)
			return
		}
	}
}

// handleContainerInspect returns detailed container information
func (a *Agent) handleContainerInspect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	if containerID == "" {
		http.Error(w, "Container ID is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	containerJSON, err := a.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to inspect container: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containerJSON)
}

// handleListContainers returns a list of containers
func (a *Agent) handleListContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	containers, err := a.docker.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list containers: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hostId":     a.hostID,
		"datacenter": a.datacenter,
		"count":      len(containers),
		"containers": containers,
	})
}
