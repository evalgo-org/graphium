// Package integrity provides database integrity checking and repair services.
// This package detects and resolves issues like duplicate documents, revision
// conflicts, and referential integrity violations in CouchDB.
package integrity

import (
	"time"
)

// IssueType represents the type of integrity issue detected.
type IssueType string

const (
	// IssueTypeDuplicate indicates multiple documents with the same ID
	IssueTypeDuplicate IssueType = "duplicate"

	// IssueTypeConflict indicates a revision conflict in CouchDB
	IssueTypeConflict IssueType = "conflict"

	// IssueTypeInvalidReference indicates a broken reference to another document
	IssueTypeInvalidReference IssueType = "invalid_reference"

	// IssueTypeInvalidSchema indicates a document that doesn't match its schema
	IssueTypeInvalidSchema IssueType = "invalid_schema"

	// IssueTypeOrphaned indicates a document with no valid references
	IssueTypeOrphaned IssueType = "orphaned"
)

// Severity represents how critical an issue is.
type Severity string

const (
	// SeverityLow indicates a minor issue that doesn't affect functionality
	SeverityLow Severity = "low"

	// SeverityMedium indicates an issue that may cause problems
	SeverityMedium Severity = "medium"

	// SeverityHigh indicates a critical issue that needs immediate attention
	SeverityHigh Severity = "high"

	// SeverityCritical indicates a severe issue causing system failure
	SeverityCritical Severity = "critical"
)

// ResolutionStrategy determines how conflicts are resolved.
type ResolutionStrategy string

const (
	// StrategyLatestWins uses the document with the latest timestamp
	StrategyLatestWins ResolutionStrategy = "latest_wins"

	// StrategyHighestRev uses the document with the highest revision number
	StrategyHighestRev ResolutionStrategy = "highest_rev"

	// StrategyMerge attempts to merge conflicting fields intelligently
	StrategyMerge ResolutionStrategy = "merge"

	// StrategyManual flags the issue for human review
	StrategyManual ResolutionStrategy = "manual"
)

// RiskLevel indicates the risk of a repair operation.
type RiskLevel string

const (
	// RiskLow indicates safe operations with no data loss risk
	RiskLow RiskLevel = "low"

	// RiskMedium indicates operations that may affect performance
	RiskMedium RiskLevel = "medium"

	// RiskHigh indicates operations with potential data loss
	RiskHigh RiskLevel = "high"
)

// ScanReport contains the results of an integrity scan.
type ScanReport struct {
	// ID uniquely identifies this scan
	ID string `json:"id"`

	// Timestamp when the scan was performed
	Timestamp time.Time `json:"timestamp"`

	// Duration of the scan
	Duration time.Duration `json:"duration"`

	// DocumentsScanned is the total number of documents checked
	DocumentsScanned int `json:"documents_scanned"`

	// IssuesFound contains all detected issues
	IssuesFound []Issue `json:"issues_found"`

	// Summary provides aggregated statistics
	Summary ScanSummary `json:"summary"`
}

// ScanSummary provides aggregated scan statistics.
type ScanSummary struct {
	// TotalIssues is the count of all issues found
	TotalIssues int `json:"total_issues"`

	// ByType breaks down issues by type
	ByType map[IssueType]int `json:"by_type"`

	// BySeverity breaks down issues by severity
	BySeverity map[Severity]int `json:"by_severity"`

	// HealthScore is a 0-100 score indicating database health
	HealthScore int `json:"health_score"`
}

// Issue represents a single integrity problem.
type Issue struct {
	// ID uniquely identifies this issue
	ID string `json:"id"`

	// Type categorizes the issue
	Type IssueType `json:"type"`

	// Severity indicates how critical this issue is
	Severity Severity `json:"severity"`

	// DocumentID is the ID of the affected document
	DocumentID string `json:"document_id"`

	// DocumentType is the @type of the document (e.g., SoftwareApplication)
	DocumentType string `json:"document_type"`

	// Description provides human-readable details
	Description string `json:"description"`

	// Details contains additional structured information
	Details map[string]interface{} `json:"details,omitempty"`

	// DetectedAt is when this issue was found
	DetectedAt time.Time `json:"detected_at"`

	// SuggestedResolution recommends how to fix this issue
	SuggestedResolution *Resolution `json:"suggested_resolution,omitempty"`
}

// Resolution describes how to fix an issue.
type Resolution struct {
	// Strategy indicates the resolution method
	Strategy ResolutionStrategy `json:"strategy"`

	// Risk indicates the risk level of this resolution
	Risk RiskLevel `json:"risk"`

	// Description explains what the resolution will do
	Description string `json:"description"`

	// Operations contains the specific steps to perform
	Operations []RepairOperation `json:"operations"`

	// RequiresApproval indicates if manual approval is needed
	RequiresApproval bool `json:"requires_approval"`
}

// RepairOperation represents a single repair action.
type RepairOperation struct {
	// ID uniquely identifies this operation
	ID string `json:"id"`

	// Type categorizes the operation
	Type OperationType `json:"type"`

	// DocumentID is the document to operate on
	DocumentID string `json:"document_id"`

	// Action describes what will be done
	Action string `json:"action"`

	// OldValue is the current state (for rollback)
	OldValue interface{} `json:"old_value,omitempty"`

	// NewValue is the target state
	NewValue interface{} `json:"new_value,omitempty"`

	// Risk indicates the risk level
	Risk RiskLevel `json:"risk"`
}

// OperationType categorizes repair operations.
type OperationType string

const (
	// OpDeleteDuplicate removes a duplicate document
	OpDeleteDuplicate OperationType = "delete_duplicate"

	// OpResolveConflict resolves a revision conflict
	OpResolveConflict OperationType = "resolve_conflict"

	// OpFixReference repairs a broken reference
	OpFixReference OperationType = "fix_reference"

	// OpUpdateSchema updates a document to match its schema
	OpUpdateSchema OperationType = "update_schema"

	// OpDeleteOrphaned removes an orphaned document
	OpDeleteOrphaned OperationType = "delete_orphaned"
)

// RepairPlan contains a sequence of operations to fix issues.
type RepairPlan struct {
	// ID uniquely identifies this plan
	ID string `json:"id"`

	// Timestamp when the plan was created
	Timestamp time.Time `json:"timestamp"`

	// ScanID references the scan that generated this plan
	ScanID string `json:"scan_id"`

	// Strategy used for resolving conflicts
	Strategy ResolutionStrategy `json:"strategy"`

	// Operations to perform
	Operations []RepairOperation `json:"operations"`

	// EstimatedDuration in milliseconds
	EstimatedDuration int64 `json:"estimated_duration_ms"`

	// DryRun indicates if this is a simulation
	DryRun bool `json:"dry_run"`

	// RiskFilter limits operations to certain risk levels
	RiskFilter []RiskLevel `json:"risk_filter"`
}

// RepairResult contains the outcome of executing a repair plan.
type RepairResult struct {
	// PlanID references the executed plan
	PlanID string `json:"plan_id"`

	// ExecutionID uniquely identifies this execution
	ExecutionID string `json:"execution_id"`

	// StartTime when execution began
	StartTime time.Time `json:"start_time"`

	// EndTime when execution completed
	EndTime time.Time `json:"end_time"`

	// Duration of the execution
	Duration time.Duration `json:"duration"`

	// Operations contains results for each operation
	Operations []OperationResult `json:"operations"`

	// SuccessCount is the number of successful operations
	SuccessCount int `json:"success_count"`

	// FailureCount is the number of failed operations
	FailureCount int `json:"failure_count"`

	// Aborted indicates if execution was stopped early
	Aborted bool `json:"aborted"`

	// AbortReason explains why execution was aborted
	AbortReason error `json:"abort_reason,omitempty"`

	// DryRun indicates if this was a simulation
	DryRun bool `json:"dry_run"`
}

// OperationResult contains the outcome of a single operation.
type OperationResult struct {
	// Operation that was executed
	Operation RepairOperation `json:"operation"`

	// Success indicates if the operation completed successfully
	Success bool `json:"success"`

	// Error contains any error that occurred
	Error error `json:"error,omitempty"`

	// StartTime when this operation began
	StartTime time.Time `json:"start_time"`

	// EndTime when this operation completed
	EndTime time.Time `json:"end_time"`

	// DryRun indicates if this was a simulation
	DryRun bool `json:"dry_run"`

	// Changes made by this operation
	Changes map[string]interface{} `json:"changes,omitempty"`
}

// DatabaseHealth represents the overall health of the database.
type DatabaseHealth struct {
	// Timestamp when health was checked
	Timestamp time.Time `json:"timestamp"`

	// TotalDocuments in the database
	TotalDocuments int `json:"total_documents"`

	// IssueCount is the total number of issues
	IssueCount int `json:"issue_count"`

	// IssuesByType breaks down issues by type
	IssuesByType map[IssueType]int `json:"issues_by_type"`

	// IssuesBySeverity breaks down issues by severity
	IssuesBySeverity map[Severity]int `json:"issues_by_severity"`

	// DatabaseSize in bytes
	DatabaseSize int64 `json:"database_size_bytes"`

	// DiskUsage as a percentage (0.0 to 1.0)
	DiskUsage float64 `json:"disk_usage"`

	// AverageRevisions per document
	AverageRevisions float64 `json:"average_revisions"`

	// RecommendCompaction indicates if compaction is advised
	RecommendCompaction bool `json:"recommend_compaction"`

	// HealthScore is a 0-100 score
	HealthScore int `json:"health_score"`

	// Recommendations for improving health
	Recommendations []string `json:"recommendations"`
}

// AuditEntry records an integrity operation for auditing.
type AuditEntry struct {
	// ID uniquely identifies this audit entry
	ID string `json:"id"`

	// Timestamp when the operation occurred
	Timestamp time.Time `json:"timestamp"`

	// Type of operation performed
	OperationType string `json:"operation_type"`

	// User who initiated the operation (if applicable)
	User string `json:"user,omitempty"`

	// ScanID if related to a scan
	ScanID string `json:"scan_id,omitempty"`

	// PlanID if related to a repair plan
	PlanID string `json:"plan_id,omitempty"`

	// ExecutionID if related to an execution
	ExecutionID string `json:"execution_id,omitempty"`

	// Success indicates if the operation succeeded
	Success bool `json:"success"`

	// Error message if the operation failed
	Error string `json:"error,omitempty"`

	// Details contains operation-specific information
	Details map[string]interface{} `json:"details,omitempty"`

	// Changes records what was modified
	Changes []AuditChange `json:"changes,omitempty"`
}

// AuditChange records a single change made during an operation.
type AuditChange struct {
	// DocumentID that was modified
	DocumentID string `json:"document_id"`

	// Field that was changed
	Field string `json:"field"`

	// OldValue before the change
	OldValue interface{} `json:"old_value"`

	// NewValue after the change
	NewValue interface{} `json:"new_value"`

	// Operation performed (create, update, delete)
	Operation string `json:"operation"`
}
