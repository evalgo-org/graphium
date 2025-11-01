package storage

import (
	"evalgo.org/graphium/models"
	"eve.evalgo.org/db"
)

// ListDeployments retrieves all deployment states with optional filters.
func (s *Storage) ListDeployments(filters map[string]interface{}) ([]*models.DeploymentState, error) {
	// Build query - deployment documents have @type = "DeploymentState"
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "DeploymentState")

	// Add filters
	for field, value := range filters {
		qb = qb.And().Where(field, "$eq", value)
	}

	query := qb.Build()

	// Execute query
	deployments, err := db.FindTyped[models.DeploymentState](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.DeploymentState, len(deployments))
	for i := range deployments {
		result[i] = &deployments[i]
	}

	return result, nil
}

// GetDeploymentsByStatus retrieves all deployments with a specific status.
func (s *Storage) GetDeploymentsByStatus(status string) ([]*models.DeploymentState, error) {
	filters := map[string]interface{}{
		"status": status,
	}
	return s.ListDeployments(filters)
}

// GetDeploymentsByStackID retrieves all deployments for a specific stack.
func (s *Storage) GetDeploymentsByStackID(stackID string) ([]*models.DeploymentState, error) {
	filters := map[string]interface{}{
		"stackId": stackID,
	}
	return s.ListDeployments(filters)
}

// SaveDeploymentState saves a deployment state document to the database.
func (s *Storage) SaveDeploymentState(state *models.DeploymentState) error {
	// Set JSON-LD type if not set
	if state.Type == "" {
		state.Type = "DeploymentState"
	}
	return s.SaveDocument(state)
}

// GetDeploymentState retrieves a deployment state by ID.
func (s *Storage) GetDeploymentState(id string) (*models.DeploymentState, error) {
	var state models.DeploymentState
	if err := s.GetDocument(id, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// UpdateDeploymentState updates an existing deployment state in the database.
// This is an alias for SaveDeploymentState which handles both create and update operations.
func (s *Storage) UpdateDeploymentState(state *models.DeploymentState) error {
	return s.SaveDeploymentState(state)
}

// DeleteDeploymentState deletes a deployment state by ID.
func (s *Storage) DeleteDeploymentState(id string) error {
	// First get the document to get the revision
	state, err := s.GetDeploymentState(id)
	if err != nil {
		// If document doesn't exist, consider it already deleted
		return nil
	}

	// Delete the document
	return s.service.DeleteDocument(id, state.Rev)
}
