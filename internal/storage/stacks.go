package storage

import (
	"encoding/json"
	"fmt"

	"eve.evalgo.org/db"

	"evalgo.org/graphium/models"
)

// SaveStack saves a stack to the database.
func (s *Storage) SaveStack(stack *models.Stack) error {
	// Set JSON-LD context and type if not set
	if stack.Context == "" {
		stack.Context = "https://schema.org"
	}
	if stack.Type == "" {
		stack.Type = "ItemList"
	}

	resp, err := s.service.SaveGenericDocument(stack)
	if err != nil {
		return err
	}

	// Update stack with new revision
	stack.Rev = resp.Rev
	return nil
}

// GetStack retrieves a stack by ID.
func (s *Storage) GetStack(id string) (*models.Stack, error) {
	var stack models.Stack
	if err := s.service.GetGenericDocument(id, &stack); err != nil {
		return nil, fmt.Errorf("stack not found: %w", err)
	}
	return &stack, nil
}

// UpdateStack updates an existing stack.
func (s *Storage) UpdateStack(stack *models.Stack) error {
	resp, err := s.service.SaveGenericDocument(stack)
	if err != nil {
		return err
	}

	// Update stack with new revision
	stack.Rev = resp.Rev
	return nil
}

// DeleteStack deletes a stack by ID.
func (s *Storage) DeleteStack(id string) error {
	// Get the current stack to get its revision
	stack, err := s.GetStack(id)
	if err != nil {
		return err
	}

	return s.service.DeleteDocument(id, stack.Rev)
}

// ListStacks retrieves all stacks with optional filters.
func (s *Storage) ListStacks(filters map[string]interface{}) ([]*models.Stack, error) {
	// Build query
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "ItemList")

	// Add filters
	for field, value := range filters {
		qb = qb.And().Where(field, "$eq", value)
	}

	query := qb.Build()

	// Execute query
	stacks, err := db.FindTyped[models.Stack](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.Stack, len(stacks))
	for i := range stacks {
		result[i] = &stacks[i]
	}

	return result, nil
}

// GetStacksByStatus retrieves all stacks with a specific status.
func (s *Storage) GetStacksByStatus(status string) ([]*models.Stack, error) {
	filters := map[string]interface{}{
		"status": status,
	}
	return s.ListStacks(filters)
}

// GetStacksByDatacenter retrieves all stacks in a specific datacenter.
func (s *Storage) GetStacksByDatacenter(datacenter string) ([]*models.Stack, error) {
	filters := map[string]interface{}{
		"location": datacenter,
	}
	return s.ListStacks(filters)
}

// SaveDeployment saves a stack deployment record.
func (s *Storage) SaveDeployment(deployment *models.StackDeployment) error {
	// Create a document wrapper with CouchDB fields
	doc := map[string]interface{}{
		"_id":           fmt.Sprintf("deployment:%s", deployment.StackID),
		"@type":         "StackDeployment",
		"stackId":       deployment.StackID,
		"placements":    deployment.Placements,
		"networkConfig": deployment.NetworkConfig,
		"startedAt":     deployment.StartedAt,
		"completedAt":   deployment.CompletedAt,
		"status":        deployment.Status,
		"errorMessage":  deployment.ErrorMessage,
	}

	_, err := s.service.SaveGenericDocument(doc)
	return err
}

// GetDeployment retrieves a deployment record by stack ID.
func (s *Storage) GetDeployment(stackID string) (*models.StackDeployment, error) {
	docID := fmt.Sprintf("deployment:%s", stackID)

	var doc map[string]interface{}
	if err := s.service.GetGenericDocument(docID, &doc); err != nil {
		return nil, fmt.Errorf("deployment not found: %w", err)
	}

	// Convert to StackDeployment
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}

	var deployment models.StackDeployment
	if err := json.Unmarshal(jsonBytes, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return &deployment, nil
}

// UpdateDeployment updates an existing deployment record.
func (s *Storage) UpdateDeployment(deployment *models.StackDeployment) error {
	return s.SaveDeployment(deployment)
}

// DeleteDeployment deletes a deployment record.
func (s *Storage) DeleteDeployment(stackID string) error {
	docID := fmt.Sprintf("deployment:%s", stackID)

	// Get current document to get revision
	var doc map[string]interface{}
	if err := s.service.GetGenericDocument(docID, &doc); err != nil {
		return err
	}

	rev, ok := doc["_rev"].(string)
	if !ok {
		return fmt.Errorf("invalid revision in deployment document")
	}

	return s.service.DeleteDocument(docID, rev)
}
