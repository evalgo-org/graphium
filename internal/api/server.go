// Package api provides the HTTP API server for Graphium.
// It uses Echo framework to serve REST endpoints and WebSocket connections
// for real-time container and host monitoring.
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/internal/web"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// Server represents the Graphium API server.
type Server struct {
	echo    *echo.Echo
	storage *storage.Storage
	config  *config.Config
	wsHub   *Hub // WebSocket hub for real-time updates
}

// New creates a new API server instance.
func New(cfg *config.Config, store *storage.Storage) *Server {
	e := echo.New()

	// Configure Echo
	e.HideBanner = true
	e.HidePort = true

	// Set custom error handler
	e.HTTPErrorHandler = HTTPErrorHandler

	// Create WebSocket hub
	hub := NewHub()

	// Create server instance
	server := &Server{
		echo:    e,
		storage: store,
		config:  cfg,
		wsHub:   hub,
	}

	// Start WebSocket hub in background
	go hub.Run()

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	return server
}

// setupMiddleware configures Echo middleware.
func (s *Server) setupMiddleware() {
	// Logger middleware
	s.echo.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[${time_rfc3339}] ${status} ${method} ${uri} (${latency_human})\n",
	}))

	// Recover middleware
	s.echo.Use(middleware.Recover())

	// CORS middleware
	if len(s.config.Security.AllowedOrigins) > 0 {
		s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: s.config.Security.AllowedOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
			AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		}))
	}

	// Request ID middleware
	s.echo.Use(middleware.RequestID())

	// Rate limiting
	if s.config.Security.RateLimit > 0 {
		s.echo.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(
			rate.Limit(s.config.Security.RateLimit),
		)))
	}

	// Timeout middleware - disabled due to incompatibility with Templ streaming
	// The timeout is still enforced at the HTTP server level (see Start method)
	// if s.config.Server.ReadTimeout > 0 {
	// 	s.echo.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
	// 		Timeout: s.config.Server.ReadTimeout,
	// 	}))
	// }
}

// setupRoutes configures API routes.
func (s *Server) setupRoutes() {
	// Health check
	s.echo.GET("/health", s.healthCheck)
	s.echo.GET("/", s.healthCheck)

	// API v1 group
	v1 := s.echo.Group("/api/v1")

	// Container routes
	containers := v1.Group("/containers")
	containers.GET("", s.listContainers)
	containers.GET("/:id", s.getContainer)
	containers.POST("", s.createContainer)
	containers.PUT("/:id", s.updateContainer)
	containers.DELETE("/:id", s.deleteContainer)
	containers.POST("/bulk", s.bulkCreateContainers)

	// Host routes
	hosts := v1.Group("/hosts")
	hosts.GET("", s.listHosts)
	hosts.GET("/:id", s.getHost)
	hosts.POST("", s.createHost)
	hosts.PUT("/:id", s.updateHost)
	hosts.DELETE("/:id", s.deleteHost)
	hosts.POST("/bulk", s.bulkCreateHosts)

	// Query routes
	query := v1.Group("/query")
	query.GET("/containers/by-host/:hostId", s.getContainersByHost)
	query.GET("/containers/by-status/:status", s.getContainersByStatus)
	query.GET("/hosts/by-datacenter/:datacenter", s.getHostsByDatacenter)
	query.GET("/traverse/:id", s.traverseGraph)
	query.GET("/dependents/:id", s.getDependents)
	query.GET("/topology/:datacenter", s.getDatacenterTopology)

	// Statistics routes
	stats := v1.Group("/stats")
	stats.GET("", s.getStatistics)
	stats.GET("/containers/count", s.getContainerCount)
	stats.GET("/hosts/count", s.getHostCount)
	stats.GET("/distribution", s.getHostContainerDistribution)

	// Validation routes
	validate := v1.Group("/validate")
	validate.POST("/container", s.validateContainer)
	validate.POST("/host", s.validateHost)
	validate.POST("/:type", s.validateGeneric)

	// Database info
	v1.GET("/info", s.getDatabaseInfo)

	// WebSocket routes
	ws := v1.Group("/ws")
	ws.GET("/graph", s.HandleWebSocket)  // WebSocket connection for graph updates
	ws.GET("/stats", s.GetWebSocketStats) // WebSocket stats

	// Graph visualization routes
	graph := v1.Group("/graph")
	graph.GET("", s.GetGraphData)
	graph.GET("/stats", s.GetGraphStats)
	graph.GET("/layout", s.GetGraphLayout)

	// Web UI routes
	webHandler := web.NewHandler(s.storage, s.config)
	s.echo.Static("/static", "static")
	s.echo.GET("/", webHandler.Dashboard)
	webGroup := s.echo.Group("/web")
	webGroup.GET("/containers", webHandler.ContainersList)
	webGroup.GET("/containers/table", webHandler.ContainersTable)
	webGroup.GET("/hosts", webHandler.HostsList)
	webGroup.GET("/hosts/table", webHandler.HostsTable)
	webGroup.GET("/topology", webHandler.TopologyView)
	webGroup.GET("/graph", webHandler.GraphView)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	fmt.Printf("🚀 Starting Graphium API Server\n")
	fmt.Printf("   Address: http://%s\n", addr)
	fmt.Printf("   Database: %s\n", s.config.CouchDB.Database)
	fmt.Printf("   Debug: %v\n", s.config.Server.Debug)
	fmt.Println()

	// Configure server timeouts
	s.echo.Server.ReadTimeout = s.config.Server.ReadTimeout
	s.echo.Server.WriteTimeout = s.config.Server.WriteTimeout

	// Start server
	if s.config.Server.TLSEnabled {
		return s.echo.StartTLS(addr, s.config.Server.TLSCert, s.config.Server.TLSKey)
	}

	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	fmt.Println("\n🛑 Shutting down Graphium API Server...")

	// Shutdown Echo server
	if err := s.echo.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	// Close storage
	if err := s.storage.Close(); err != nil {
		return fmt.Errorf("error closing storage: %w", err)
	}

	fmt.Println("✓ Server shutdown complete")
	return nil
}

// healthCheck handles health check requests.
func (s *Server) healthCheck(c echo.Context) error {
	// Get database info to verify connection
	info, err := s.storage.GetDatabaseInfo()
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "unhealthy",
			"error":   "database connection failed",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "healthy",
		"service":  "graphium",
		"version":  "0.1.0",
		"database": info.DBName,
		"documents": map[string]interface{}{
			"total":   info.DocCount,
			"deleted": info.DocDelCount,
		},
		"uptime": info.InstanceStartTime,
	})
}

// BroadcastGraphEvent broadcasts a graph event to all WebSocket clients
func (s *Server) BroadcastGraphEvent(eventType GraphEventType, data interface{}) {
	event := GraphEvent{
		Type: eventType,
		Data: data,
	}
	if err := s.wsHub.BroadcastEvent(event); err != nil {
		log.Printf("Failed to broadcast event: %v", err)
	}
}
