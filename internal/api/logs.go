package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/labstack/echo/v4"
)

// ContainerLogsRequest represents the request parameters for fetching container logs
type ContainerLogsRequest struct {
	Lines      int    `query:"lines"`      // Number of lines to fetch (default: 100)
	Follow     bool   `query:"follow"`     // Stream logs (default: false)
	Timestamps bool   `query:"timestamps"` // Include timestamps (default: true)
	Since      string `query:"since"`      // Show logs since timestamp/duration
	Tail       string `query:"tail"`       // Number of lines from end (default: "100")
}

// getContainerLogs godoc
// @Summary Get container logs
// @Description Get logs from a container via the agent's Docker socket
// @Tags containers
// @Accept json
// @Produce text/plain
// @Param id path string true "Container ID"
// @Param lines query int false "Number of lines" default(100)
// @Param follow query bool false "Stream logs" default(false)
// @Param timestamps query bool false "Include timestamps" default(true)
// @Param tail query string false "Lines from end" default("100")
// @Success 200 {string} string "Container logs"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /containers/{id}/logs [get]
func (s *Server) getContainerLogs(c echo.Context) error {
	containerID := c.Param("id")
	if containerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	// Parse query parameters
	var req ContainerLogsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}

	// Set defaults
	if req.Tail == "" {
		req.Tail = "100"
	}

	// Get container from storage to verify it exists
	cont, err := s.storage.GetContainer(containerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Container not found")
	}

	// Check if we need to proxy through agent HTTP server
	if cont.HostedOn != "" && s.agentManager != nil {
		// Try to get agent HTTP URL
		agentURL := s.agentManager.GetAgentHTTPURL(cont.HostedOn)
		if agentURL != "" {
			// Proxy request to agent HTTP server
			return s.proxyLogsToAgent(c, containerID, agentURL)
		}
	}

	// Fall back to local Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to connect to Docker: %v", err))
	}
	defer dockerClient.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	// Fetch logs from Docker
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: req.Timestamps,
		Follow:     req.Follow,
		Tail:       req.Tail,
	}

	// Add since parameter if provided
	if req.Since != "" {
		logOptions.Since = req.Since
	}

	logs, err := dockerClient.ContainerLogs(ctx, containerID, logOptions)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch logs: %v", err))
	}
	defer logs.Close()

	// Set response headers
	c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
	c.Response().Header().Set("X-Container-ID", containerID)
	c.Response().Header().Set("X-Container-Name", cont.Name)

	// If following logs, use chunked transfer
	if req.Follow {
		c.Response().Header().Set("Transfer-Encoding", "chunked")
		c.Response().Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
		c.Response().WriteHeader(http.StatusOK)

		// Stream logs
		flusher, ok := c.Response().Writer.(http.Flusher)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Streaming not supported")
		}

		buf := make([]byte, 8192)
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				n, err := logs.Read(buf)
				if n > 0 {
					// Docker logs include 8-byte headers, we need to strip them
					if n > 8 {
						// Write without the Docker stream header
						if _, writeErr := c.Response().Write(buf[8:n]); writeErr != nil {
							return writeErr
						}
						flusher.Flush()
					}
				}
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
			}
		}
	}

	// For non-streaming, read all logs and return
	c.Response().WriteHeader(http.StatusOK)

	// Copy logs to response, stripping Docker headers
	buf := make([]byte, 8192)
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			// Strip Docker stream header (first 8 bytes)
			if n > 8 {
				if _, writeErr := c.Response().Write(buf[8:n]); writeErr != nil {
					return writeErr
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// proxyLogsToAgent proxies container logs request to an agent HTTP server
func (s *Server) proxyLogsToAgent(c echo.Context, containerID, agentURL string) error {
	// Build URL to agent HTTP server
	url := fmt.Sprintf("%s/containers/%s/logs", agentURL, containerID)

	// Forward all query parameters
	if c.Request().URL.RawQuery != "" {
		url += "?" + c.Request().URL.RawQuery
	}

	// Create HTTP request to agent
	req, err := http.NewRequestWithContext(c.Request().Context(), "GET", url, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Forward relevant headers
	if userAgent := c.Request().Header.Get("User-Agent"); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	// Create HTTP client with no timeout for streaming
	client := &http.Client{
		Timeout: 0, // No timeout for streaming logs
	}

	// Execute request to agent
	resp, err := client.Do(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, fmt.Sprintf("Failed to connect to agent: %v", err))
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return echo.NewHTTPError(resp.StatusCode, "Agent returned error")
	}

	// Forward response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Response().Header().Add(key, value)
		}
	}

	// Set status code
	c.Response().WriteHeader(resp.StatusCode)

	// Stream response body to client
	buf := make([]byte, 8192)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := c.Response().Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			// Flush if possible (for streaming)
			if flusher, ok := c.Response().Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadContainerLogs downloads container logs as a file
func (s *Server) downloadContainerLogs(c echo.Context) error {
	containerID := c.Param("id")
	if containerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	// Get lines parameter
	linesStr := c.QueryParam("lines")
	lines := 1000
	if linesStr != "" {
		if parsed, err := strconv.Atoi(linesStr); err == nil {
			lines = parsed
		}
	}

	// Get container from storage
	cont, err := s.storage.GetContainer(containerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Container not found")
	}

	// Check if container is on a remote host (needs agent proxy)
	var dockerClient *client.Client
	var isRemote bool

	if cont.HostedOn != "" && s.agentManager != nil {
		// Try to get Docker client from agent
		dockerClient, err = s.agentManager.GetDockerClient(cont.HostedOn)
		if err == nil {
			isRemote = true
		}
	}

	// If no agent or agent not available, try local Docker
	if dockerClient == nil {
		dockerClient, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to Docker")
		}
	}
	// Only close if it's a local client we created
	if !isRemote {
		defer dockerClient.Close()
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	// Fetch logs
	logs, err := dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       strconv.Itoa(lines),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch logs")
	}
	defer logs.Close()

	// Set download headers
	filename := fmt.Sprintf("%s-logs-%s.txt", cont.Name, time.Now().Format("20060102-150405"))
	c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	// Copy logs, stripping headers
	buf := make([]byte, 8192)
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			if n > 8 {
				if _, writeErr := c.Response().Write(buf[8:n]); writeErr != nil {
					return writeErr
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}
