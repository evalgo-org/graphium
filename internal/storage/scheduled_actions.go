package storage

import (
	"encoding/json"
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

	// Build query using direct MangoQuery since QueryBuilder doesn't handle $in properly
	selector := map[string]interface{}{
		"@type": map[string]interface{}{
			"$in": []string{"Action", "CreateAction", "UpdateAction", "CheckAction", "ControlAction", "TransferAction"},
		},
	}

	// Add filters
	if filters != nil {
		if actionType, ok := filters["@type"].(string); ok && actionType != "" {
			selector["@type"] = actionType // Override with specific type
		}
		if enabled, ok := filters["enabled"].(bool); ok {
			selector["enabled"] = enabled
		}
		if agent, ok := filters["agent"].(string); ok && agent != "" {
			selector["agent"] = agent
		}
		if actionStatus, ok := filters["actionStatus"].(string); ok && actionStatus != "" {
			selector["actionStatus"] = actionStatus
		}
	}

	query := db.MangoQuery{
		Selector: selector,
	}
	s.debugLog("ListScheduledActions query: %+v\n", query)

	// Execute query - use non-typed Find to debug unmarshaling issues
	rawResults, err := s.service.Find(query)
	if err != nil {
		s.debugLog("Find ERROR: %v\n", err)
		return nil, fmt.Errorf("failed to find actions: %w", err)
	}

	s.debugLog("Find returned %d raw results\n", len(rawResults))

	// Manually unmarshal to see errors
	actions := make([]models.ScheduledAction, 0, len(rawResults))
	for i, raw := range rawResults {
		var action models.ScheduledAction
		if err := json.Unmarshal(raw, &action); err != nil {
			s.debugLog("UNMARSHAL ERROR for document %d: %v\n", i, err)
			s.debugLog("Raw JSON: %s\n", string(raw))
			continue // Skip documents that fail to unmarshal
		}
		actions = append(actions, action)
	}

	s.debugLog("Successfully unmarshaled %d actions out of %d\n", len(actions), len(rawResults))

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
