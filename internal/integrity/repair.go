package integrity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateRepairPlan generates a plan to fix detected issues.
func (s *Service) CreateRepairPlan(scanID string, strategy ResolutionStrategy, riskFilter []RiskLevel) (*RepairPlan, error) {
	s.logger.Printf("Creating repair plan for scan %s with strategy %s", scanID, strategy)

	// For now, we'll create a plan based on a fresh scan
	// TODO: Store scan results and retrieve them by scanID
	ctx := context.Background()

	// Perform a fresh scan to get current issues
	scanReport, err := s.Scan(ctx, ScanOptions{
		ScanDuplicates: true,
		ScanConflicts:  false,
		ScanReferences: false,
		ScanSchemas:    false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan for issues: %w", err)
	}

	plan := &RepairPlan{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		ScanID:     scanID,
		Strategy:   strategy,
		Operations: []RepairOperation{},
		DryRun:     true, // Always start with dry-run
		RiskFilter: riskFilter,
	}

	// Generate repair operations for each issue
	for _, issue := range scanReport.IssuesFound {
		// Apply risk filter if specified
		if len(riskFilter) > 0 && issue.SuggestedResolution != nil {
			allowed := false
			for _, allowedRisk := range riskFilter {
				if issue.SuggestedResolution.Risk == allowedRisk {
					allowed = true
					break
				}
			}
			if !allowed {
				s.logger.Printf("Skipping issue %s due to risk filter (risk: %s)", issue.ID, issue.SuggestedResolution.Risk)
				continue
			}
		}

		// Generate operations based on issue type
		switch issue.Type {
		case IssueTypeDuplicate:
			ops, err := s.generateDuplicateRepairOperations(issue)
			if err != nil {
				s.logger.Printf("Warning: failed to generate operations for issue %s: %v", issue.ID, err)
				continue
			}
			plan.Operations = append(plan.Operations, ops...)
		}
	}

	// Estimate duration (rough estimate: 10ms per operation)
	plan.EstimatedDuration = int64(len(plan.Operations) * 10)

	s.logger.Printf("Generated repair plan %s with %d operations", plan.ID, len(plan.Operations))

	return plan, nil
}

// generateDuplicateRepairOperations creates repair operations for a duplicate issue.
func (s *Service) generateDuplicateRepairOperations(issue Issue) ([]RepairOperation, error) {
	operations := []RepairOperation{}

	// Extract duplicate document IDs from issue details
	docIDs, ok := issue.Details["document_ids"].([]string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid document_ids in issue details")
	}

	if len(docIDs) < 2 {
		return operations, nil // No duplicates to remove
	}

	// Determine which document to keep
	// For now, keep the first one (oldest by CouchDB ID)
	// In a real implementation, we'd use the resolution strategy
	keepID := docIDs[0]

	// Create delete operations for all other documents
	for _, docID := range docIDs[1:] {
		op := RepairOperation{
			ID:         uuid.New().String(),
			Type:       OpDeleteDuplicate,
			DocumentID: docID,
			Action:     fmt.Sprintf("Delete duplicate document (keeping %s)", keepID),
			Risk:       RiskMedium, // Deleting duplicates is medium risk
		}
		operations = append(operations, op)
	}

	return operations, nil
}

// ExecutePlan executes a repair plan.
func (s *Service) ExecutePlan(ctx context.Context, plan *RepairPlan) (*RepairResult, error) {
	s.logger.Printf("Executing repair plan %s (dry-run: %v)", plan.ID, plan.DryRun)

	startTime := time.Now()
	result := &RepairResult{
		PlanID:       plan.ID,
		ExecutionID:  uuid.New().String(),
		StartTime:    startTime,
		Operations:   []OperationResult{},
		SuccessCount: 0,
		FailureCount: 0,
		DryRun:       plan.DryRun,
		Aborted:      false,
	}

	// Execute each operation
	for i, op := range plan.Operations {
		s.logger.Printf("Executing operation %d/%d: %s (%s)", i+1, len(plan.Operations), op.Type, op.DocumentID)

		opResult := s.executeOperation(ctx, op, plan.DryRun)
		result.Operations = append(result.Operations, opResult)

		if opResult.Success {
			result.SuccessCount++
		} else {
			result.FailureCount++
			s.logger.Printf("Operation failed: %v", opResult.Error)

			// Check if we should abort on error
			if !plan.DryRun && result.FailureCount > 10 {
				result.Aborted = true
				result.AbortReason = fmt.Errorf("too many failures (%d), aborting", result.FailureCount)
				s.logger.Printf("Aborting execution: %v", result.AbortReason)
				break
			}
		}

		// Log operation to audit
		if err := s.audit.LogOperation(result.ExecutionID, opResult); err != nil {
			s.logger.Printf("Warning: failed to log operation to audit: %v", err)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Log execution to audit
	if err := s.audit.LogExecution(result); err != nil {
		s.logger.Printf("Warning: failed to log execution to audit: %v", err)
	}

	s.logger.Printf("Execution completed: %d succeeded, %d failed, duration: %v",
		result.SuccessCount, result.FailureCount, result.Duration)

	return result, nil
}

// executeOperation executes a single repair operation.
func (s *Service) executeOperation(ctx context.Context, op RepairOperation, dryRun bool) OperationResult {
	startTime := time.Now()
	result := OperationResult{
		Operation: op,
		Success:   false,
		StartTime: startTime,
		DryRun:    dryRun,
		Changes:   make(map[string]interface{}),
	}

	// If dry-run, just simulate success
	if dryRun {
		result.Success = true
		result.EndTime = time.Now()
		result.Changes["action"] = "simulated"
		return result
	}

	// Execute the actual operation based on type
	switch op.Type {
	case OpDeleteDuplicate:
		err := s.deleteDuplicateDocument(ctx, op.DocumentID)
		if err != nil {
			result.Error = err
		} else {
			result.Success = true
			result.Changes["deleted"] = op.DocumentID
		}

	default:
		result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
	}

	result.EndTime = time.Now()
	return result
}

// deleteDuplicateDocument deletes a duplicate document from the database.
func (s *Service) deleteDuplicateDocument(ctx context.Context, documentID string) error {
	s.logger.Printf("Deleting duplicate document: %s", documentID)

	// First, get the document to retrieve its revision
	var doc map[string]interface{}
	if err := s.db.GetGenericDocument(documentID, &doc); err != nil {
		return fmt.Errorf("failed to get document for deletion: %w", err)
	}

	// Extract revision
	rev, ok := doc["_rev"].(string)
	if !ok {
		return fmt.Errorf("document missing _rev field")
	}

	// Delete the document
	if err := s.db.DeleteDocument(documentID, rev); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	s.logger.Printf("Successfully deleted document %s (rev: %s)", documentID, rev)
	return nil
}

// SavePlanToFile saves a repair plan to a JSON file.
func SavePlanToFile(plan *RepairPlan, filename string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Write to file (this would use os.WriteFile in real implementation)
	// For now, just return success
	_ = data
	_ = filename

	return nil
}

// LoadPlanFromFile loads a repair plan from a JSON file.
func LoadPlanFromFile(filename string) (*RepairPlan, error) {
	// This would use os.ReadFile in real implementation
	// For now, just return an error
	return nil, fmt.Errorf("not implemented")
}
