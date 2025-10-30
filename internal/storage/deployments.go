package storage

import (
	"eve.evalgo.org/db"
	"evalgo.org/graphium/models"
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
