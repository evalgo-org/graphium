// Package integrity provides database integrity checking, validation, and automated
// repair capabilities for the Graphium CouchDB database.
//
// The integrity service helps maintain data consistency and health by:
//   - Scanning for duplicate documents with the same semantic ID
//   - Detecting revision conflicts in CouchDB
//   - Validating referential integrity between documents
//   - Checking JSON-LD schema compliance
//   - Generating repair plans for detected issues
//   - Executing repairs with configurable risk levels
//   - Maintaining audit logs of all operations
//   - Computing database health scores
//
// Architecture:
//
// The service is organized into several components:
//   - Scanner: Detects integrity issues across multiple categories
//   - Repair Planner: Generates repair plans with risk assessment
//   - Executor: Applies repair operations with dry-run support
//   - Audit Logger: Tracks all scans, plans, and repairs
//
// Issue Types:
//
// The service can detect and repair several types of issues:
//   - Duplicates: Multiple documents with same @id in JSON-LD
//   - Conflicts: CouchDB revision conflicts
//   - Broken References: References to non-existent documents
//   - Schema Violations: Invalid JSON-LD structure
//
// Resolution Strategies:
//
// Multiple resolution strategies are supported:
//   - latest-wins: Keep most recently modified document
//   - oldest-wins: Keep oldest document
//   - merge: Combine data from duplicates (manual review required)
//   - manual: Flag for manual resolution
//
// Risk Levels:
//
// Each repair operation is assigned a risk level:
//   - low: Safe automated repairs (e.g., delete exact duplicates)
//   - medium: Generally safe but may need review
//   - high: Requires careful review before execution
//   - critical: Manual intervention strongly recommended
//
// Example usage:
//
//	service, err := integrity.NewService(dbService, config, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer service.Close()
//
//	// Scan for issues
//	report, err := service.Scan(ctx, integrity.ScanOptions{
//	    ScanDuplicates: true,
//	    ScanConflicts:  true,
//	})
//
//	// Create repair plan
//	plan, err := service.CreateRepairPlan(
//	    report.ID,
//	    integrity.StrategyLatestWins,
//	    []integrity.RiskLevel{integrity.RiskLow, integrity.RiskMedium},
//	)
//
//	// Execute repairs (dry-run first)
//	result, err := service.ExecutePlan(ctx, plan)
package integrity

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"evalgo.org/graphium/internal/config"
	"eve.evalgo.org/db"
)

// Service provides database integrity checking and repair capabilities.
// It scans for duplicates, conflicts, and referential integrity issues,
// then generates and executes repair plans to maintain database health.
type Service struct {
	db     *db.CouchDBService
	config *Config
	logger *log.Logger
	audit  *AuditLogger
}

// Config contains configuration for the integrity service.
type Config struct {
	// Enabled determines if integrity checking is active
	Enabled bool

	// ScanSchedule in cron format (e.g., "0 2 * * *" for daily at 2 AM)
	ScanSchedule string

	// AutoRepair configuration
	AutoRepair AutoRepairConfig

	// Resolution strategies
	Resolution ResolutionConfig

	// Validation rules
	Validation ValidationConfig

	// Monitoring settings
	Monitoring MonitoringConfig

	// Audit logging
	Audit AuditConfig

	// Performance tuning
	Performance PerformanceConfig
}

// AutoRepairConfig controls automatic repair behavior.
type AutoRepairConfig struct {
	// Enabled determines if repairs run automatically
	Enabled bool

	// MaxRisk is the maximum risk level for auto-repair
	MaxRisk RiskLevel

	// Strategies allowed for automatic resolution
	Strategies []ResolutionStrategy
}

// ResolutionConfig defines resolution strategies.
type ResolutionConfig struct {
	// DefaultStrategy used when no specific strategy is configured
	DefaultStrategy ResolutionStrategy

	// StrategyByType maps document types to strategies
	StrategyByType map[string]ResolutionStrategy
}

// ValidationConfig controls validation behavior.
type ValidationConfig struct {
	// CheckReferences validates referential integrity
	CheckReferences bool

	// CheckSchemas validates against JSON schemas
	CheckSchemas bool

	// StrictMode causes validation failures to block operations
	StrictMode bool
}

// MonitoringConfig controls health monitoring.
type MonitoringConfig struct {
	// HealthCheckInterval for periodic health checks
	HealthCheckInterval time.Duration

	// AlertThreshold health score that triggers alerts
	AlertThreshold int

	// MetricsRetention how long to keep metrics
	MetricsRetention time.Duration
}

// AuditConfig controls audit logging.
type AuditConfig struct {
	// Enabled determines if audit logging is active
	Enabled bool

	// Retention how long to keep audit logs
	Retention time.Duration

	// LogPath where to store audit logs
	LogPath string
}

// PerformanceConfig controls performance settings.
type PerformanceConfig struct {
	// MaxConcurrentOperations limits parallel repairs
	MaxConcurrentOperations int

	// BatchSize for bulk operations
	BatchSize int

	// Timeout for individual operations
	Timeout time.Duration
}

// NewService creates a new integrity service.
func NewService(dbService *db.CouchDBService, appConfig *config.Config, logger *log.Logger) (*Service, error) {
	if dbService == nil {
		return nil, fmt.Errorf("database service is required")
	}

	if logger == nil {
		logger = log.Default()
	}

	// Build integrity config from app config
	cfg := buildConfig(appConfig)

	// Initialize audit logger
	audit, err := NewAuditLogger(cfg.Audit)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	service := &Service{
		db:     dbService,
		config: cfg,
		logger: logger,
		audit:  audit,
	}

	return service, nil
}

// buildConfig constructs integrity config from app config.
func buildConfig(appConfig *config.Config) *Config {
	// Default configuration
	cfg := &Config{
		Enabled:      true,
		ScanSchedule: "0 2 * * *", // Daily at 2 AM
		AutoRepair: AutoRepairConfig{
			Enabled:    false, // Manual approval by default
			MaxRisk:    RiskLow,
			Strategies: []ResolutionStrategy{StrategyLatestWins},
		},
		Resolution: ResolutionConfig{
			DefaultStrategy: StrategyLatestWins,
			StrategyByType: map[string]ResolutionStrategy{
				"SoftwareApplication": StrategyMerge,
				"ComputerServer":      StrategyLatestWins,
				"Stack":               StrategyManual,
			},
		},
		Validation: ValidationConfig{
			CheckReferences: true,
			CheckSchemas:    true,
			StrictMode:      false,
		},
		Monitoring: MonitoringConfig{
			HealthCheckInterval: 5 * time.Minute,
			AlertThreshold:      80,
			MetricsRetention:    30 * 24 * time.Hour, // 30 days
		},
		Audit: AuditConfig{
			Enabled:   true,
			Retention: 90 * 24 * time.Hour, // 90 days
			LogPath:   "./logs/integrity/",
		},
		Performance: PerformanceConfig{
			MaxConcurrentOperations: 5,
			BatchSize:               100,
			Timeout:                 30 * time.Second,
		},
	}

	// TODO: Override with actual config values when integrity section is added to config.yaml

	return cfg
}

// Scan performs a comprehensive integrity scan of the database.
func (s *Service) Scan(ctx context.Context, options ScanOptions) (*ScanReport, error) {
	s.logger.Printf("Starting integrity scan with options: %+v", options)

	startTime := time.Now()
	scanID := uuid.New().String()

	report := &ScanReport{
		ID:               scanID,
		Timestamp:        startTime,
		DocumentsScanned: 0,
		IssuesFound:      []Issue{},
		Summary: ScanSummary{
			ByType:     make(map[IssueType]int),
			BySeverity: make(map[Severity]int),
		},
	}

	// Scan for different issue types based on options
	if options.ScanDuplicates {
		duplicates, err := s.scanDuplicates(ctx)
		if err != nil {
			s.logger.Printf("Error scanning for duplicates: %v", err)
		} else {
			report.IssuesFound = append(report.IssuesFound, duplicates...)
		}
	}

	if options.ScanConflicts {
		conflicts, err := s.scanConflicts(ctx)
		if err != nil {
			s.logger.Printf("Error scanning for conflicts: %v", err)
		} else {
			report.IssuesFound = append(report.IssuesFound, conflicts...)
		}
	}

	if options.ScanReferences {
		refIssues, err := s.scanReferences(ctx)
		if err != nil {
			s.logger.Printf("Error scanning references: %v", err)
		} else {
			report.IssuesFound = append(report.IssuesFound, refIssues...)
		}
	}

	// Calculate summary
	report.Summary.TotalIssues = len(report.IssuesFound)
	for _, issue := range report.IssuesFound {
		report.Summary.ByType[issue.Type]++
		report.Summary.BySeverity[issue.Severity]++
	}

	// Calculate health score
	report.Summary.HealthScore = s.calculateHealthScore(report)

	// Record duration
	report.Duration = time.Since(startTime)

	// Log to audit
	s.audit.LogScan(report)

	s.logger.Printf("Scan completed: found %d issues in %v", report.Summary.TotalIssues, report.Duration)

	return report, nil
}

// ScanOptions configures what to scan for.
type ScanOptions struct {
	// ScanDuplicates checks for duplicate documents
	ScanDuplicates bool

	// ScanConflicts checks for revision conflicts
	ScanConflicts bool

	// ScanReferences checks referential integrity
	ScanReferences bool

	// ScanSchemas validates against schemas
	ScanSchemas bool

	// DocumentTypes limits scan to specific types (empty = all)
	DocumentTypes []string
}

// scanDuplicates is implemented in duplicates.go

// scanConflicts finds documents with revision conflicts.
func (s *Service) scanConflicts(ctx context.Context) ([]Issue, error) {
	s.logger.Println("Scanning for revision conflicts...")

	// TODO: Implement actual conflict detection
	// This is a placeholder for Phase 2
	issues := []Issue{}

	return issues, nil
}

// scanReferences validates referential integrity.
func (s *Service) scanReferences(ctx context.Context) ([]Issue, error) {
	s.logger.Println("Scanning for reference integrity issues...")

	// TODO: Implement actual reference validation
	// This is a placeholder for Phase 4
	issues := []Issue{}

	return issues, nil
}

// calculateHealthScore computes a 0-100 health score based on issues found.
func (s *Service) calculateHealthScore(report *ScanReport) int {
	score := 100

	// Deduct for issues based on severity
	for severity, count := range report.Summary.BySeverity {
		switch severity {
		case SeverityCritical:
			score -= count * 20 // 20 points per critical issue
		case SeverityHigh:
			score -= count * 10 // 10 points per high issue
		case SeverityMedium:
			score -= count * 3 // 3 points per medium issue
		case SeverityLow:
			score -= count * 1 // 1 point per low issue
		}
	}

	// Ensure score stays in range [0, 100]
	if score < 0 {
		score = 0
	}

	return score
}

// CreateRepairPlan is implemented in repair.go

// ExecutePlan is implemented in repair.go

// CheckHealth performs a quick health check.
func (s *Service) CheckHealth(ctx context.Context) (*DatabaseHealth, error) {
	s.logger.Println("Checking database health...")

	health := &DatabaseHealth{
		Timestamp:        time.Now(),
		IssuesByType:     make(map[IssueType]int),
		IssuesBySeverity: make(map[Severity]int),
		Recommendations:  []string{},
	}

	// TODO: Implement actual health checks
	// This is a placeholder for Phase 6

	// Quick scan for critical issues only
	options := ScanOptions{
		ScanDuplicates: true,
		ScanConflicts:  true,
	}

	report, err := s.Scan(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to scan for health check: %w", err)
	}

	health.IssueCount = report.Summary.TotalIssues
	health.IssuesByType = report.Summary.ByType
	health.IssuesBySeverity = report.Summary.BySeverity
	health.HealthScore = report.Summary.HealthScore

	// Generate recommendations
	if health.HealthScore < 50 {
		health.Recommendations = append(health.Recommendations,
			"Critical: Database health is poor. Run immediate integrity scan and repairs.")
	} else if health.HealthScore < 80 {
		health.Recommendations = append(health.Recommendations,
			"Warning: Database has integrity issues. Schedule maintenance.")
	}

	if health.IssuesByType[IssueTypeDuplicate] > 10 {
		health.Recommendations = append(health.Recommendations,
			"High number of duplicates detected. Run duplicate cleanup.")
	}

	if health.IssuesByType[IssueTypeConflict] > 5 {
		health.Recommendations = append(health.Recommendations,
			"Revision conflicts detected. Run conflict resolution.")
	}

	return health, nil
}

// Close cleans up service resources.
func (s *Service) Close() error {
	if s.audit != nil {
		return s.audit.Close()
	}
	return nil
}
