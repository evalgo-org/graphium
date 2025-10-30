package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/integrity"
)

// IntegrityScanRequest contains options for scanning.
type IntegrityScanRequest struct {
	ScanDuplicates bool `json:"scan_duplicates"`
	ScanConflicts  bool `json:"scan_conflicts"`
	ScanReferences bool `json:"scan_references"`
	ScanSchemas    bool `json:"scan_schemas"`
}

// scanIntegrity handles POST /api/v1/integrity/scan
// @Summary Scan database for integrity issues
// @Description Perform a comprehensive integrity scan checking for duplicates, conflicts, broken references, and schema violations
// @Tags Integrity
// @Accept json
// @Produce json
// @Param options body IntegrityScanRequest true "Scan options"
// @Success 200 {object} integrity.ScanReport
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /integrity/scan [post]
func (s *Server) scanIntegrity(c echo.Context) error {
	var req IntegrityScanRequest

	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	// Default to scanning everything if nothing specified
	if !req.ScanDuplicates && !req.ScanConflicts && !req.ScanReferences && !req.ScanSchemas {
		req.ScanDuplicates = true
		req.ScanConflicts = true
		req.ScanReferences = true
		req.ScanSchemas = true
	}

	// Get integrity service
	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// Perform scan
	report, err := integrityService.Scan(c.Request().Context(), integrity.ScanOptions{
		ScanDuplicates: req.ScanDuplicates,
		ScanConflicts:  req.ScanConflicts,
		ScanReferences: req.ScanReferences,
		ScanSchemas:    req.ScanSchemas,
	})

	if err != nil {
		return InternalError("Integrity scan failed", err.Error())
	}

	return c.JSON(http.StatusOK, report)
}

// getHealth handles GET /api/v1/integrity/health
// @Summary Get database health status
// @Description Get comprehensive database health metrics including issue counts, health score, and recommendations
// @Tags Integrity
// @Accept json
// @Produce json
// @Success 200 {object} integrity.DatabaseHealth
// @Failure 500 {object} ErrorResponse
// @Router /integrity/health [get]
func (s *Server) getHealth(c echo.Context) error {
	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	health, err := integrityService.CheckHealth(c.Request().Context())
	if err != nil {
		return InternalError("Failed to get health status", err.Error())
	}

	return c.JSON(http.StatusOK, health)
}

// getScanReport handles GET /api/v1/integrity/scans/:id
// @Summary Get scan report by ID
// @Description Retrieve a specific integrity scan report
// @Tags Integrity
// @Accept json
// @Produce json
// @Param id path string true "Scan ID"
// @Success 200 {object} integrity.ScanReport
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /integrity/scans/{id} [get]
func (s *Server) getScanReport(c echo.Context) error {
	scanID := c.Param("id")

	if scanID == "" {
		return BadRequestError("Scan ID is required", "The 'id' parameter cannot be empty")
	}

	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// TODO: Implement GetScanReport in integrity service
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"message": "Scan report retrieval not yet implemented",
		"scan_id": scanID,
	})
}

// listScans handles GET /api/v1/integrity/scans
// @Summary List integrity scans
// @Description Get a list of all integrity scan reports
// @Tags Integrity
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of scans to return" default(10)
// @Param offset query int false "Number of scans to skip" default(0)
// @Success 200 {object} object
// @Failure 500 {object} ErrorResponse
// @Router /integrity/scans [get]
func (s *Server) listScans(c echo.Context) error {
	limit, offset := parsePagination(c)

	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// TODO: Implement ListScans in integrity service
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"message": "Scan listing not yet implemented",
		"limit":   limit,
		"offset":  offset,
	})
}

// CreateRepairPlanRequest contains options for creating a repair plan.
type CreateRepairPlanRequest struct {
	ScanID         string                       `json:"scan_id"`
	Strategy       integrity.ResolutionStrategy `json:"strategy"`
	RiskFilter     []integrity.RiskLevel        `json:"risk_filter"`
	DryRun         bool                         `json:"dry_run"`
}

// createRepairPlan handles POST /api/v1/integrity/repair-plans
// @Summary Create a repair plan
// @Description Generate a repair plan for detected integrity issues
// @Tags Integrity
// @Accept json
// @Produce json
// @Param plan body CreateRepairPlanRequest true "Repair plan options"
// @Success 200 {object} integrity.RepairPlan
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /integrity/repair-plans [post]
func (s *Server) createRepairPlan(c echo.Context) error {
	var req CreateRepairPlanRequest

	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	if req.ScanID == "" {
		return BadRequestError("Scan ID is required", "scan_id field cannot be empty")
	}

	// Default strategy if not specified
	if req.Strategy == "" {
		req.Strategy = integrity.StrategyLatestWins
	}

	// Default risk filter if not specified
	if len(req.RiskFilter) == 0 {
		req.RiskFilter = []integrity.RiskLevel{integrity.RiskLow, integrity.RiskMedium}
	}

	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// TODO: Implement CreateRepairPlan in integrity service
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"message":  "Repair plan creation not yet implemented",
		"scan_id":  req.ScanID,
		"strategy": req.Strategy,
	})
}

// ExecuteRepairPlanRequest contains the plan ID to execute.
type ExecuteRepairPlanRequest struct {
	PlanID string `json:"plan_id"`
	DryRun bool   `json:"dry_run"`
}

// executeRepairPlan handles POST /api/v1/integrity/execute
// @Summary Execute a repair plan
// @Description Execute repairs based on a generated plan
// @Tags Integrity
// @Accept json
// @Produce json
// @Param request body ExecuteRepairPlanRequest true "Execution request"
// @Success 200 {object} integrity.RepairResult
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /integrity/execute [post]
func (s *Server) executeRepairPlan(c echo.Context) error {
	var req ExecuteRepairPlanRequest

	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", "Failed to parse JSON: "+err.Error())
	}

	if req.PlanID == "" {
		return BadRequestError("Plan ID is required", "plan_id field cannot be empty")
	}

	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// TODO: Implement ExecutePlan in integrity service
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"message": "Repair plan execution not yet implemented",
		"plan_id": req.PlanID,
		"dry_run": req.DryRun,
	})
}

// getAuditLog handles GET /api/v1/integrity/audit
// @Summary Get integrity audit log
// @Description Retrieve audit log entries for integrity operations
// @Tags Integrity
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of entries to return" default(100)
// @Param offset query int false "Number of entries to skip" default(0)
// @Param operation_type query string false "Filter by operation type"
// @Success 200 {object} object
// @Failure 500 {object} ErrorResponse
// @Router /integrity/audit [get]
func (s *Server) getAuditLog(c echo.Context) error {
	limit, offset := parsePagination(c)
	if limit > 100 {
		limit = 100 // Cap at 100 for audit logs
	}

	operationType := c.QueryParam("operation_type")

	integrityService := s.getIntegrityService()
	if integrityService == nil {
		return InternalError("Integrity service not available", "Service not initialized")
	}

	// TODO: Implement GetAuditLog in integrity service
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"message":        "Audit log retrieval not yet implemented",
		"limit":          limit,
		"offset":         offset,
		"operation_type": operationType,
	})
}

// getIntegrityService returns the integrity service instance.
// This will be implemented when the service is integrated into the Server struct.
func (s *Server) getIntegrityService() *integrity.Service {
	// TODO: Add integrity service to Server struct and return it here
	// For now, return nil to indicate not implemented
	return s.integrity
}
