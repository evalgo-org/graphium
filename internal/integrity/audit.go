package integrity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditLogger records all integrity operations for auditing and compliance.
type AuditLogger struct {
	config    AuditConfig
	file      *os.File
	mu        sync.Mutex
	buffer    []AuditEntry
	flushSize int
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(config AuditConfig) (*AuditLogger, error) {
	if !config.Enabled {
		return &AuditLogger{
			config:    config,
			flushSize: 100,
		}, nil
	}

	// Ensure log directory exists
	if err := os.MkdirAll(config.LogPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Create audit log file
	filename := filepath.Join(config.LogPath, fmt.Sprintf("integrity-audit-%s.jsonl",
		time.Now().Format("2006-01-02")))

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &AuditLogger{
		config:    config,
		file:      file,
		buffer:    make([]AuditEntry, 0, 100),
		flushSize: 100,
	}, nil
}

// LogScan records a scan operation.
func (a *AuditLogger) LogScan(report *ScanReport) error {
	if !a.config.Enabled {
		return nil
	}

	entry := AuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationType: "scan",
		ScanID:        report.ID,
		Success:       true,
		Details: map[string]interface{}{
			"duration_ms":       report.Duration.Milliseconds(),
			"documents_scanned": report.DocumentsScanned,
			"issues_found":      report.Summary.TotalIssues,
			"health_score":      report.Summary.HealthScore,
		},
	}

	return a.writeEntry(entry)
}

// LogExecution records a repair execution.
func (a *AuditLogger) LogExecution(result *RepairResult) error {
	if !a.config.Enabled {
		return nil
	}

	entry := AuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationType: "execution",
		PlanID:        result.PlanID,
		ExecutionID:   result.ExecutionID,
		Success:       !result.Aborted && result.FailureCount == 0,
		Details: map[string]interface{}{
			"duration_ms":   result.Duration.Milliseconds(),
			"success_count": result.SuccessCount,
			"failure_count": result.FailureCount,
			"dry_run":       result.DryRun,
			"aborted":       result.Aborted,
		},
	}

	if result.AbortReason != nil {
		entry.Error = result.AbortReason.Error()
	}

	// Record changes
	changes := make([]AuditChange, 0)
	for _, opResult := range result.Operations {
		if opResult.Success && !result.DryRun {
			change := AuditChange{
				DocumentID: opResult.Operation.DocumentID,
				Field:      "multiple",
				OldValue:   opResult.Operation.OldValue,
				NewValue:   opResult.Operation.NewValue,
				Operation:  string(opResult.Operation.Type),
			}
			changes = append(changes, change)
		}
	}
	entry.Changes = changes

	return a.writeEntry(entry)
}

// LogOperation records a single repair operation.
func (a *AuditLogger) LogOperation(executionID string, opResult OperationResult) error {
	if !a.config.Enabled {
		return nil
	}

	entry := AuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationType: "operation",
		ExecutionID:   executionID,
		Success:       opResult.Success,
		Details: map[string]interface{}{
			"operation_type": opResult.Operation.Type,
			"document_id":    opResult.Operation.DocumentID,
			"risk":           opResult.Operation.Risk,
			"dry_run":        opResult.DryRun,
		},
	}

	if opResult.Error != nil {
		entry.Error = opResult.Error.Error()
	}

	if opResult.Success && !opResult.DryRun {
		change := AuditChange{
			DocumentID: opResult.Operation.DocumentID,
			Field:      "document",
			OldValue:   opResult.Operation.OldValue,
			NewValue:   opResult.Operation.NewValue,
			Operation:  string(opResult.Operation.Type),
		}
		entry.Changes = []AuditChange{change}
	}

	return a.writeEntry(entry)
}

// LogHealthCheck records a health check operation.
func (a *AuditLogger) LogHealthCheck(health *DatabaseHealth) error {
	if !a.config.Enabled {
		return nil
	}

	entry := AuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationType: "health_check",
		Success:       true,
		Details: map[string]interface{}{
			"health_score":    health.HealthScore,
			"issue_count":     health.IssueCount,
			"total_documents": health.TotalDocuments,
			"database_size":   health.DatabaseSize,
		},
	}

	return a.writeEntry(entry)
}

// LogManualIntervention records a manual operation.
func (a *AuditLogger) LogManualIntervention(user, operation, description string, details map[string]interface{}) error {
	if !a.config.Enabled {
		return nil
	}

	entry := AuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationType: fmt.Sprintf("manual_%s", operation),
		User:          user,
		Success:       true,
		Details:       details,
	}

	if description != "" {
		entry.Details["description"] = description
	}

	return a.writeEntry(entry)
}

// writeEntry writes an audit entry to the log.
func (a *AuditLogger) writeEntry(entry AuditEntry) error {
	if !a.config.Enabled {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Add to buffer
	a.buffer = append(a.buffer, entry)

	// Flush if buffer is full
	if len(a.buffer) >= a.flushSize {
		return a.flushLocked()
	}

	return nil
}

// Flush writes all buffered entries to disk.
func (a *AuditLogger) Flush() error {
	if !a.config.Enabled {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	return a.flushLocked()
}

// flushLocked writes buffered entries (must be called with lock held).
func (a *AuditLogger) flushLocked() error {
	if len(a.buffer) == 0 {
		return nil
	}

	// Write each entry as a JSON line
	for _, entry := range a.buffer {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal audit entry: %w", err)
		}

		if _, err := fmt.Fprintf(a.file, "%s\n", data); err != nil {
			return fmt.Errorf("failed to write audit entry: %w", err)
		}
	}

	// Sync to disk
	if err := a.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	// Clear buffer
	a.buffer = a.buffer[:0]

	return nil
}

// Close closes the audit logger and flushes any remaining entries.
func (a *AuditLogger) Close() error {
	if !a.config.Enabled || a.file == nil {
		return nil
	}

	// Flush remaining entries
	if err := a.Flush(); err != nil {
		return err
	}

	// Close file
	return a.file.Close()
}

// Rotate creates a new log file and closes the old one.
// This should be called daily to prevent log files from growing too large.
func (a *AuditLogger) Rotate() error {
	if !a.config.Enabled || a.file == nil {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Flush and close current file
	if err := a.flushLocked(); err != nil {
		return err
	}

	if err := a.file.Close(); err != nil {
		return err
	}

	// Create new file
	filename := filepath.Join(a.config.LogPath, fmt.Sprintf("integrity-audit-%s.jsonl",
		time.Now().Format("2006-01-02")))

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new audit log file: %w", err)
	}

	a.file = file

	return nil
}

// Query searches audit logs for entries matching criteria.
func (a *AuditLogger) Query(criteria AuditQuery) ([]AuditEntry, error) {
	if !a.config.Enabled {
		return nil, fmt.Errorf("audit logging is not enabled")
	}

	// TODO: Implement efficient audit log querying
	// For now, return empty results
	return []AuditEntry{}, nil
}

// AuditQuery defines search criteria for audit logs.
type AuditQuery struct {
	// StartTime for the search window
	StartTime time.Time

	// EndTime for the search window
	EndTime time.Time

	// OperationType to filter by
	OperationType string

	// User to filter by
	User string

	// Success to filter by success status (nil = no filter)
	Success *bool

	// Limit maximum number of results
	Limit int
}
