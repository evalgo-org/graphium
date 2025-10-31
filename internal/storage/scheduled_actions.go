package storage

import (
	"fmt"

	"eve.evalgo.org/db"
	"evalgo.org/graphium/models"
)

// CreateScheduledAction creates a new scheduled action
func (s *Storage) CreateScheduledAction(action *models.ScheduledAction) error {
	s.debugLog("Creating scheduled action: %s (type: %s)\n", action.Name, action.Type)

	// Validate required fields
	if action.Name == "" {
		return fmt.Errorf("action name is required")
	}
	if action.Agent == "" {
		return fmt.Errorf("agent (host ID) is required")
	}
	if action.Schedule == nil {
		return fmt.Errorf("schedule is required")
	}
	if action.Schedule.RepeatFrequency == "" {
		return fmt.Errorf("schedule repeat frequency is required")
	}

	// Set defaults
	if action.Context == "" {
		action.Context = "https://schema.org"
	}
	if action.Type == "" {
		action.Type = models.ActionTypeAction
	}
	if action.ActionStatus == "" {
		action.ActionStatus = models.ActionStatusPotential
	}
	if action.Schedule.Type == "" {
		action.Schedule.Type = "Schedule"
	}
	if action.Schedule.ScheduleTimezone == "" {
		action.Schedule.ScheduleTimezone = "UTC"
	}

	// Generate ID if not set
	if action.ID == "" {
		action.ID = models.GenerateID("action")
	}

	// Store in CouchDB
	resp, err := s.service.SaveGenericDocument(action)
	if err != nil {
		return fmt.Errorf("failed to create action: %w", err)
	}

	action.Rev = resp.Rev
	s.debugLog("Created scheduled action %s with rev %s\n", action.ID, resp.Rev)

	return nil
}

// GetScheduledAction retrieves a scheduled action by ID
func (s *Storage) GetScheduledAction(id string) (*models.ScheduledAction, error) {
	s.debugLog("Getting scheduled action: %s\n", id)

	var action models.ScheduledAction
	if err := s.service.GetGenericDocument(id, &action); err != nil {
		return nil, fmt.Errorf("failed to read action: %w", err)
	}

	return &action, nil
}

// ListScheduledActions lists all scheduled actions with optional filters
func (s *Storage) ListScheduledActions(filters map[string]interface{}) ([]*models.ScheduledAction, error) {
	s.debugLog("Listing scheduled actions with filters: %+v\n", filters)

	// Build query
	qb := db.NewQueryBuilder().
		Where("schedule", "$exists", true) // Actions must have a schedule

	// Add filters
	if filters != nil {
		if actionType, ok := filters["@type"].(string); ok && actionType != "" {
			qb = qb.And().Where("@type", "$eq", actionType)
		}
		if enabled, ok := filters["enabled"].(bool); ok {
			qb = qb.And().Where("enabled", "$eq", enabled)
		}
		if agent, ok := filters["agent"].(string); ok && agent != "" {
			qb = qb.And().Where("agent", "$eq", agent)
		}
		if actionStatus, ok := filters["actionStatus"].(string); ok && actionStatus != "" {
			qb = qb.And().Where("actionStatus", "$eq", actionStatus)
		}
	}

	query := qb.Build()
	s.debugLog("ListScheduledActions query: %+v\n", query)

	// Execute query
	actions, err := db.FindTyped[models.ScheduledAction](s.service, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find actions: %w", err)
	}

	s.debugLog("ListScheduledActions returning %d actions\n", len(actions))

	// Convert to pointer slice
	result := make([]*models.ScheduledAction, len(actions))
	for i := range actions {
		result[i] = &actions[i]
	}

	return result, nil
}

// UpdateScheduledAction updates an existing scheduled action
func (s *Storage) UpdateScheduledAction(action *models.ScheduledAction) error {
	s.debugLog("Updating scheduled action: %s\n", action.ID)

	if action.ID == "" {
		return fmt.Errorf("action ID is required")
	}
	if action.Rev == "" {
		return fmt.Errorf("action revision is required for update")
	}

	// Update in CouchDB
	resp, err := s.service.SaveGenericDocument(action)
	if err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	action.Rev = resp.Rev
	s.debugLog("Updated scheduled action %s to rev %s\n", action.ID, resp.Rev)

	return nil
}

// DeleteScheduledAction deletes a scheduled action
func (s *Storage) DeleteScheduledAction(id, rev string) error {
	s.debugLog("Deleting scheduled action: %s (rev: %s)\n", id, rev)

	if id == "" {
		return fmt.Errorf("action ID is required")
	}
	if rev == "" {
		return fmt.Errorf("action revision is required for delete")
	}

	if err := s.service.DeleteDocument(id, rev); err != nil {
		return fmt.Errorf("failed to delete action: %w", err)
	}

	s.debugLog("Deleted scheduled action %s\n", id)
	return nil
}

// GetScheduledActionsByAgent returns all scheduled actions for a specific agent (host)
func (s *Storage) GetScheduledActionsByAgent(agentID string) ([]*models.ScheduledAction, error) {
	return s.ListScheduledActions(map[string]interface{}{
		"agent":   agentID,
		"enabled": true,
	})
}

// GetActiveScheduledActions returns all enabled actions that are not currently executing
func (s *Storage) GetActiveScheduledActions() ([]*models.ScheduledAction, error) {
	return s.ListScheduledActions(map[string]interface{}{
		"enabled": true,
	})
}

// GetScheduledActionsByType returns all scheduled actions of a specific type
func (s *Storage) GetScheduledActionsByType(actionType string) ([]*models.ScheduledAction, error) {
	return s.ListScheduledActions(map[string]interface{}{
		"@type": actionType,
	})
}
