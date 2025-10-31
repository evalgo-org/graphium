// Package api provides the HTTP API server for Graphium.
// It uses Echo framework to serve REST endpoints and WebSocket connections
// for real-time container and host monitoring.
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"golang.org/x/time/rate"

	_ "evalgo.org/graphium/docs" // Import generated docs
	"evalgo.org/graphium/internal/agents"
	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/integrity"
	"evalgo.org/graphium/internal/scheduler"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/internal/web"
)

// Server represents the Graphium API server.
type Server struct {
	echo         *echo.Echo
	storage      *storage.Storage
	config       *config.Config
	wsHub        *Hub // WebSocket hub for real-time updates
	authMiddle   *auth.Middleware
	integrity    *integrity.Service // Database integrity service
	agentManager *agents.Manager    // Agent process manager
	scheduler    *scheduler.Scheduler // Scheduled actions scheduler
}

// debugLog logs a message only if debug mode is enabled in config
func (s *Server) debugLog(format string, args ...interface{}) {
	if s.config.Server.Debug {
		log.Printf(format, args...)
	}
}

// New creates a new API server instance.
func New(cfg *config.Config, store *storage.Storage, agentMgr *agents.Manager) *Server {
	e := echo.New()

	// Configure Echo
	e.HideBanner = true
	e.HidePort = true
	e.Debug = cfg.Server.Debug

	// Set custom error handler
	e.HTTPErrorHandler = HTTPErrorHandler

	// Create WebSocket hub
	hub := NewHub()

	// Create auth middleware
	authMiddle := auth.NewMiddleware(cfg)

	// Initialize integrity service
	integrityService, err := integrity.NewService(store.GetDBService(), cfg, log.Default())
	if err != nil {
		log.Printf("Warning: Failed to initialize integrity service: %v", err)
		// Continue without integrity service - it's optional
		integrityService = nil
	}

	// Initialize scheduler for scheduled actions
	sched := scheduler.New(store)

	// Create server instance
	server := &Server{
		echo:         e,
		storage:      store,
		config:       cfg,
		wsHub:        hub,
		authMiddle:   authMiddle,
		integrity:    integrityService,
		agentManager: agentMgr,
		scheduler:    sched,
	}

	// Start WebSocket hub in background
	go hub.Run()

	// Start task monitor for automatic cleanup
	go server.runTaskMonitor()

	// Start scheduler for scheduled actions
	sched.Start()
	log.Println("Scheduled actions scheduler started")

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

	// Security headers middleware
	s.echo.Use(SecurityHeaders)

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

	// Content-Type validation middleware for API routes
	s.echo.Use(ValidateContentType)

	// Accept header validation middleware
	s.echo.Use(ValidateAcceptHeader)

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

	// Swagger UI documentation (public - but API endpoints are still protected)
	s.echo.GET("/docs/*", echoSwagger.WrapHandler)

	// API v1 group
	v1 := s.echo.Group("/api/v1")

	// Container routes
	containers := v1.Group("/containers")
	containers.Use(ValidateQueryParams) // Validate query parameters for list operations
	containers.GET("", s.listContainers, s.authMiddle.RequireRead)
	containers.GET("/ignored", s.listIgnored, s.authMiddle.RequireAgentOrWrite) // List all ignored containers
	containers.GET("/:id", s.getContainer, ValidateIDFormat, s.authMiddle.RequireRead)
	containers.HEAD("/:id/ignored", s.checkContainerIgnored, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	containers.DELETE("/:id/ignored", s.removeFromIgnoreList, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	// Note: logs endpoints moved after webHandler creation (see below)
	containers.POST("", s.createContainer, s.authMiddle.RequireAgentOrWrite)
	containers.PUT("/:id", s.updateContainer, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	containers.DELETE("/:id", s.deleteContainer, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	containers.POST("/bulk", s.bulkCreateContainers, s.authMiddle.RequireAgentOrWrite)

	// Host routes
	hosts := v1.Group("/hosts")
	hosts.Use(ValidateQueryParams) // Validate query parameters for list operations
	hosts.GET("", s.listHosts, s.authMiddle.RequireRead)
	hosts.GET("/:id", s.getHost, ValidateIDFormat, s.authMiddle.RequireRead)
	hosts.POST("", s.createHost, s.authMiddle.RequireAgentOrWrite)
	hosts.PUT("/:id", s.updateHost, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	hosts.PUT("/:id/metrics", s.updateHostMetrics, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	hosts.DELETE("/:id", s.deleteHost, ValidateIDFormat, s.authMiddle.RequireAgentOrWrite)
	hosts.POST("/bulk", s.bulkCreateHosts, s.authMiddle.RequireAgentOrWrite)

	// Query routes
	query := v1.Group("/query")
	query.GET("/containers/by-host/:hostId", s.getContainersByHost, ValidateIDFormat, s.authMiddle.RequireRead)
	query.GET("/containers/by-status/:status", s.getContainersByStatus, s.authMiddle.RequireRead)
	query.GET("/hosts/by-datacenter/:datacenter", s.getHostsByDatacenter, s.authMiddle.RequireRead)
	query.GET("/traverse/:id", s.traverseGraph, ValidateIDFormat, s.authMiddle.RequireRead)
	query.GET("/dependents/:id", s.getDependents, ValidateIDFormat, s.authMiddle.RequireRead)
	query.GET("/topology/:datacenter", s.getDatacenterTopology, s.authMiddle.RequireRead)

	// Validation routes
	validate := v1.Group("/validate")
	validate.POST("/container", s.validateContainer, s.authMiddle.RequireRead)
	validate.POST("/host", s.validateHost, s.authMiddle.RequireRead)
	validate.POST("/:type", s.validateGeneric, s.authMiddle.RequireRead)

	// Database info
	v1.GET("/info", s.getDatabaseInfo, s.authMiddle.RequireRead)

	// Stack routes (basic CRUD)
	stackRoutes := v1.Group("/stacks")
	stackRoutes.GET("", s.listStacks, s.authMiddle.RequireRead)
	stackRoutes.GET("/:id", s.getStack, ValidateIDFormat, s.authMiddle.RequireRead)
	stackRoutes.GET("/:id/deployment", s.getStackDeployment, ValidateIDFormat, s.authMiddle.RequireRead)

	// JSON-LD Stack deployment routes
	jsonldStacks := v1.Group("/stacks/jsonld")
	jsonldStacks.POST("", s.deployJSONLDStack, s.authMiddle.RequireWrite)
	jsonldStacks.POST("/validate", s.validateJSONLDStack, s.authMiddle.RequireRead)
	jsonldStacks.GET("/deployments", s.listJSONLDDeployments, s.authMiddle.RequireRead)
	jsonldStacks.GET("/deployments/:id", s.getJSONLDDeployment, ValidateIDFormat, s.authMiddle.RequireRead)

	// Authentication routes
	authRoutes := v1.Group("/auth")
	authRoutes.POST("/login", s.login)
	authRoutes.POST("/register", s.register, s.authMiddle.RequireAdmin)
	authRoutes.POST("/refresh", s.refresh)
	authRoutes.POST("/logout", s.logout, s.authMiddle.RequireAuth)
	authRoutes.GET("/me", s.me, s.authMiddle.RequireAuth)

	// User management routes
	users := v1.Group("/users")
	users.GET("", s.listUsers, s.authMiddle.RequireAdmin)
	users.GET("/:id", s.getUser, s.authMiddle.RequireAdmin)
	users.PUT("/:id", s.updateUser, s.authMiddle.RequireAdmin)
	users.DELETE("/:id", s.deleteUser, s.authMiddle.RequireAdmin)
	users.POST("/password", s.changePassword, s.authMiddle.RequireAuth)
	users.POST("/api-keys", s.generateAPIKey, s.authMiddle.RequireAuth)
	users.DELETE("/api-keys/:index", s.revokeAPIKey, s.authMiddle.RequireAuth)

	// Agent management API routes
	agents := v1.Group("/agents")
	agents.POST("/:id/start", s.startAgent, ValidateIDFormat, s.authMiddle.RequireWrite)
	agents.POST("/:id/stop", s.stopAgent, ValidateIDFormat, s.authMiddle.RequireWrite)
	agents.POST("/:id/restart", s.restartAgent, ValidateIDFormat, s.authMiddle.RequireWrite)

	// Web UI routes
	webHandler := web.NewHandler(s.storage, s.config, &serverBroadcaster{server: s}, s.agentManager)

	// Statistics routes (support both JWT and web session auth for web UI compatibility)
	stats := v1.Group("/stats")
	stats.GET("", s.getStatistics, webHandler.WebAuthMiddleware)
	stats.GET("/containers/count", s.getContainerCount, webHandler.WebAuthMiddleware)
	stats.GET("/hosts/count", s.getHostCount, webHandler.WebAuthMiddleware)
	stats.GET("/distribution", s.getHostContainerDistribution, webHandler.WebAuthMiddleware)

	// Container logs routes (support web session auth for web UI compatibility)
	v1.GET("/containers/:id/logs", s.getContainerLogs, ValidateIDFormat, webHandler.WebAuthMiddleware)
	v1.GET("/containers/:id/logs/download", s.downloadContainerLogs, ValidateIDFormat, webHandler.WebAuthMiddleware)

	// Integrity routes (database health and repair)
	integrityRoutes := v1.Group("/integrity")
	integrityRoutes.POST("/scan", s.scanIntegrity, s.authMiddle.RequireAdmin)
	integrityRoutes.GET("/health", s.getHealth, s.authMiddle.RequireRead)
	integrityRoutes.GET("/scans/:id", s.getScanReport, ValidateIDFormat, s.authMiddle.RequireRead)
	integrityRoutes.GET("/scans", s.listScans, s.authMiddle.RequireRead)
	integrityRoutes.POST("/repair-plans", s.createRepairPlan, s.authMiddle.RequireAdmin)
	integrityRoutes.POST("/execute", s.executeRepairPlan, s.authMiddle.RequireAdmin)
	integrityRoutes.GET("/audit", s.getAuditLog, s.authMiddle.RequireAdmin)

	// Agent management routes
	agentRoutes := v1.Group("/agents")
	agentRoutes.GET("", s.listAgents, s.authMiddle.RequireRead)
	agentRoutes.GET("/:id", s.getAgent, ValidateIDFormat, s.authMiddle.RequireRead)
	agentRoutes.POST("", s.createAgent, s.authMiddle.RequireAdmin)
	agentRoutes.PUT("/:id", s.updateAgent, ValidateIDFormat, s.authMiddle.RequireAdmin)
	agentRoutes.DELETE("/:id", s.deleteAgent, ValidateIDFormat, s.authMiddle.RequireAdmin)
	agentRoutes.POST("/:id/start", s.startAgent, ValidateIDFormat, s.authMiddle.RequireAdmin)
	agentRoutes.POST("/:id/stop", s.stopAgent, ValidateIDFormat, s.authMiddle.RequireAdmin)
	agentRoutes.POST("/:id/restart", s.restartAgent, ValidateIDFormat, s.authMiddle.RequireAdmin)

	// Agent task routes (for agents to poll and update task status)
	agentRoutes.GET("/:id/tasks", s.getAgentTasks, ValidateIDFormat, s.authMiddle.RequireAgentAuth)

	// Task management routes
	tasks := v1.Group("/tasks")
	tasks.POST("", s.createTask, s.authMiddle.RequireWrite)
	tasks.GET("", s.listTasks, s.authMiddle.RequireRead)
	tasks.GET("/stats", s.getTaskStatistics, s.authMiddle.RequireRead)
	tasks.GET("/:id", s.getTask, ValidateIDFormat, s.authMiddle.RequireRead)
	tasks.PUT("/:id/status", s.updateTaskStatus, ValidateIDFormat, s.authMiddle.RequireAgentAuth)
	tasks.POST("/:id/retry", s.retryTask, ValidateIDFormat, s.authMiddle.RequireWrite)
	tasks.POST("/:id/cancel", s.cancelTask, ValidateIDFormat, s.authMiddle.RequireWrite)

	// Scheduled Actions routes (schema.org Actions with schedules)
	actions := v1.Group("/actions")
	actions.POST("", s.CreateScheduledAction, s.authMiddle.RequireWrite)
	actions.GET("", s.ListScheduledActions, s.authMiddle.RequireRead)
	actions.GET("/:id", s.GetScheduledAction, ValidateIDFormat, s.authMiddle.RequireRead)
	actions.PUT("/:id", s.UpdateScheduledAction, ValidateIDFormat, s.authMiddle.RequireWrite)
	actions.DELETE("/:id", s.DeleteScheduledAction, ValidateIDFormat, s.authMiddle.RequireWrite)
	actions.POST("/:id/execute", s.ExecuteScheduledAction, ValidateIDFormat, s.authMiddle.RequireWrite)
	actions.GET("/:id/history", s.GetScheduledActionHistory, ValidateIDFormat, s.authMiddle.RequireRead)

	// WebSocket routes (use web auth middleware for session cookie support)
	ws := v1.Group("/ws")
	ws.GET("/stats", s.GetWebSocketStats, webHandler.WebAuthMiddleware) // WebSocket stats
	s.echo.Static("/static", "static")

	// Public routes (redirect to login if not authenticated)
	s.echo.GET("/", webHandler.Dashboard, webHandler.WebAuthMiddleware)

	// Authentication routes (no auth required for login, required for logout)
	webAuth := s.echo.Group("/web/auth")
	webAuth.GET("/login", webHandler.LoginPage)
	webAuth.POST("/login", webHandler.Login)
	webAuth.GET("/logout", webHandler.Logout, webHandler.WebAuthMiddleware)

	// Profile routes (auth required)
	s.echo.GET("/web/profile", webHandler.Profile, webHandler.WebAuthMiddleware)
	s.echo.POST("/web/profile/password", webHandler.ChangePassword, webHandler.WebAuthMiddleware)

	// User management routes (auth required, admin for list/create/delete)
	webUsers := s.echo.Group("/web/users")
	webUsers.Use(webHandler.WebAuthMiddleware)
	webUsers.GET("", webHandler.ListUsers, webHandler.WebAdminMiddleware)
	webUsers.GET("/new", webHandler.NewUserForm, webHandler.WebAdminMiddleware)
	webUsers.POST("/create", webHandler.CreateUser, webHandler.WebAdminMiddleware)
	webUsers.GET("/:id", webHandler.ViewUser)
	webUsers.GET("/:id/edit", webHandler.EditUserForm)
	webUsers.POST("/:id/update", webHandler.UpdateUser)
	webUsers.POST("/:id/delete", webHandler.DeleteUser, webHandler.WebAdminMiddleware)
	webUsers.POST("/:id/api-keys/generate", webHandler.GenerateAPIKey)
	webUsers.POST("/:id/api-keys/:index/revoke", webHandler.RevokeAPIKey)

	// Container and host routes (authentication REQUIRED)
	webGroup := s.echo.Group("/web")
	webGroup.Use(webHandler.WebAuthMiddleware) // Require authentication for all web pages
	webGroup.GET("/containers", webHandler.ContainersList)
	webGroup.GET("/containers/table", webHandler.ContainersTable)
	webGroup.GET("/containers/:id", webHandler.ContainerDetail)
	webGroup.GET("/containers/:id/logs", webHandler.ContainerLogs)
	webGroup.POST("/containers/:id/delete", webHandler.DeleteContainer)
	webGroup.POST("/containers/bulk/delete", webHandler.BulkDeleteContainers)
	webGroup.POST("/containers/bulk/stop", webHandler.BulkStopContainers)
	webGroup.POST("/containers/bulk/start", webHandler.BulkStartContainers)
	webGroup.POST("/containers/bulk/restart", webHandler.BulkRestartContainers)
	webGroup.GET("/hosts", webHandler.HostsList)
	webGroup.GET("/hosts/table", webHandler.HostsTable)
	webGroup.GET("/hosts/new", webHandler.CreateHostForm)
	webGroup.POST("/hosts/create", webHandler.CreateHost)
	webGroup.GET("/hosts/:id", webHandler.HostDetail)
	webGroup.GET("/hosts/:id/edit", webHandler.EditHostForm)
	webGroup.POST("/hosts/:id/update", webHandler.UpdateHost)
	webGroup.POST("/hosts/:id/delete", webHandler.DeleteHost)
	webGroup.GET("/stacks", webHandler.StacksList)
	webGroup.GET("/stacks/table", webHandler.StacksTable)
	webGroup.GET("/stacks/json", webHandler.GetStacksJSON)
	webGroup.GET("/stacks/new", webHandler.DeployStackForm)
	webGroup.POST("/stacks/deploy", webHandler.DeployStack)
	webGroup.GET("/stacks/:id", webHandler.StackDetail)
	webGroup.GET("/stacks/:id/edit", webHandler.EditStackForm)
	webGroup.POST("/stacks/:id/update", webHandler.UpdateStack)
	webGroup.GET("/stacks/:id/logs", webHandler.StackLogs)
	webGroup.POST("/stacks/:id/start", webHandler.StartStack)
	webGroup.POST("/stacks/:id/stop", webHandler.StopStack)
	webGroup.POST("/stacks/:id/restart", webHandler.RestartStack)
	webGroup.POST("/stacks/:id/delete", webHandler.DeleteStack)
	webGroup.POST("/stacks/:id/containers/assign", webHandler.AssignContainersToStack)

	// JSON-LD Stack Deployment routes (Web UI)
	webGroup.GET("/stacks/jsonld/deploy", webHandler.JSONLDDeployPage)
	webGroup.POST("/stacks/jsonld/deploy", webHandler.JSONLDDeploy)
	webGroup.POST("/stacks/jsonld/validate", webHandler.JSONLDValidate)
	webGroup.GET("/stacks/jsonld/deployments/:id", webHandler.JSONLDDeploymentDetail)

	webGroup.GET("/topology", webHandler.TopologyView)

	// Agent management routes (web UI)
	webGroup.GET("/agents", webHandler.AgentsPage)
	webGroup.GET("/agents/new", webHandler.CreateAgentFormHandler)
	webGroup.POST("/agents/create", webHandler.CreateAgentHandler)
	webGroup.GET("/agents/table", webHandler.AgentsTableHandler)
	webGroup.GET("/agents/:id", webHandler.AgentDetailPage)
	webGroup.POST("/agents/:id/start", webHandler.StartAgentHandler)
	webGroup.POST("/agents/:id/stop", webHandler.StopAgentHandler)
	webGroup.POST("/agents/:id/restart", webHandler.RestartAgentHandler)
	webGroup.DELETE("/agents/:id", webHandler.DeleteAgentHandler)
	webGroup.GET("/agents/:id/logs/download", webHandler.AgentLogsDownloadHandler)
	webGroup.GET("/agents/:id/logs", webHandler.AgentLogsHandler)

	// Scheduled Actions routes (web UI)
	webGroup.GET("/actions", webHandler.ActionsPage)
	webGroup.GET("/actions/new", webHandler.CreateActionFormHandler)
	webGroup.POST("/actions/create", webHandler.CreateActionHandler)
	webGroup.GET("/actions/table", webHandler.ActionsTableHandler)
	webGroup.GET("/actions/:id", webHandler.ActionDetailPage)
	webGroup.POST("/actions/:id/execute", webHandler.ExecuteActionHandler)
	webGroup.POST("/actions/:id/toggle", webHandler.ToggleActionHandler)
	webGroup.POST("/actions/:id/update", webHandler.UpdateActionHandler)
	webGroup.POST("/actions/:id/delete", webHandler.DeleteActionHandler)
}

// runTaskMonitor watches for completed deletion tasks and cleans up stack metadata.
func (s *Server) runTaskMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	s.debugLog("Task monitor started")

	for range ticker.C {
		s.checkCompletedStackDeletions()
	}
}

// checkCompletedStackDeletions checks for stacks with status="deleting" and cleans up
// the stack metadata once all deletion tasks are complete.
func (s *Server) checkCompletedStackDeletions() {
	// Get all stacks with status="deleting"
	stacks, err := s.storage.ListStacks(map[string]interface{}{
		"status": "deleting",
	})
	if err != nil {
		s.debugLog("Task monitor: Failed to list deleting stacks: %v", err)
		return
	}

	if len(stacks) == 0 {
		return
	}

	s.debugLog("Task monitor: Found %d stack(s) in deleting state", len(stacks))

	for _, stack := range stacks {
		// Get tasks for this stack
		tasks, err := s.storage.GetTasksByStack(stack.ID)
		if err != nil {
			s.debugLog("Task monitor: Failed to get tasks for stack %s: %v", stack.ID, err)
			continue
		}

		// Check if all tasks are complete (completed, failed, or cancelled)
		allComplete := true
		completedCount := 0
		failedCount := 0
		cancelledCount := 0

		for _, task := range tasks {
			switch task.Status {
			case "completed":
				completedCount++
			case "failed":
				failedCount++
			case "cancelled":
				cancelledCount++
			default:
				allComplete = false
			}
		}

		if allComplete && len(tasks) > 0 {
			s.debugLog("Task monitor: All %d task(s) complete for stack %s (completed: %d, failed: %d, cancelled: %d)",
				len(tasks), stack.ID, completedCount, failedCount, cancelledCount)

			// Delete deployment state
			if err := s.storage.DeleteDeploymentState(stack.ID); err != nil {
				s.debugLog("Task monitor: Failed to delete deployment state for stack %s: %v", stack.ID, err)
			}

			// Delete stack metadata
			if err := s.storage.DeleteStack(stack.ID); err != nil {
				s.debugLog("Task monitor: Failed to delete stack %s: %v", stack.ID, err)
				continue
			}

			// Broadcast event
			s.BroadcastGraphEvent("stack_deleted", map[string]interface{}{
				"stackId": stack.ID,
				"name":    stack.Name,
			})

			s.debugLog("Task monitor: Successfully deleted stack %s", stack.ID)
		} else if !allComplete {
			s.debugLog("Task monitor: Stack %s still has %d pending/running task(s)", stack.ID, len(tasks)-(completedCount+failedCount+cancelledCount))
		}
	}
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

	// Stop scheduler
	if s.scheduler != nil {
		log.Println("Stopping scheduler...")
		s.scheduler.Stop()
	}

	// Shutdown Echo server
	if err := s.echo.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	// Close integrity service
	if s.integrity != nil {
		if err := s.integrity.Close(); err != nil {
			log.Printf("Warning: error closing integrity service: %v", err)
		}
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
	s.debugLog("DEBUG: BroadcastGraphEvent called with type=%s, data=%+v", eventType, data)
	event := GraphEvent{
		Type: eventType,
		Data: data,
	}
	s.debugLog("DEBUG: Broadcasting to %d WebSocket clients", s.wsHub.ClientCount())
	if err := s.wsHub.BroadcastEvent(event); err != nil {
		log.Printf("ERROR: Failed to broadcast event: %v", err)
	} else {
		s.debugLog("DEBUG: Successfully broadcast %s event to hub", eventType)
	}
}

// serverBroadcaster adapts Server to web.EventBroadcaster interface
type serverBroadcaster struct {
	server *Server
}

// BroadcastGraphEvent implements web.EventBroadcaster
func (sb *serverBroadcaster) BroadcastGraphEvent(eventType string, data interface{}) {
	sb.server.BroadcastGraphEvent(GraphEventType(eventType), data)
}

// ServeHTTP allows Server to implement http.Handler for testing
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
