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
