package models

import "time"

// ScheduledAction represents a schema.org Action with a Schedule
// This allows for semantic task orchestration following schema.org vocabulary
type ScheduledAction struct {
	// JSON-LD context
	Context string `json:"@context" couchdb:"@context"`
	Type    string `json:"@type" couchdb:"@type"`

	// CouchDB fields
	ID  string `json:"@id" couchdb:"_id"`
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Action properties (schema.org/Action)
	Name         string                 `json:"name"`                  // schema:name - Human-readable action name
	Description  string                 `json:"description,omitempty"` // schema:description
	ActionStatus string                 `json:"actionStatus"`          // schema:actionStatus (PotentialActionStatus, ActiveActionStatus, CompletedActionStatus, FailedActionStatus)
	Agent        string                 `json:"agent"`                 // schema:agent - Host ID that executes the action
	Object       *ActionObject          `json:"object"`                // schema:object - Target of the action
	Instrument   map[string]interface{} `json:"instrument,omitempty"`  // schema:instrument - Parameters/configuration
	Result       *ActionResult          `json:"result,omitempty"`      // schema:result - Last execution result
	Error        *ActionError           `json:"error,omitempty"`       // schema:error - Last error if any

	// Scheduling properties (schema:Schedule embedded)
	Schedule *Schedule `json:"schedule"` // When and how often to execute

	// Execution tracking
	StartTime *time.Time `json:"startTime,omitempty"` // schema:startTime - Last execution start
	EndTime   *time.Time `json:"endTime,omitempty"`   // schema:endTime - Last execution end

	// Graphium extensions
	Enabled   bool      `json:"enabled"` // Whether schedule is active
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Schedule represents schema.org Schedule
// See: https://schema.org/Schedule
type Schedule struct {
	Type             string     `json:"@type"`                      // schema:Schedule
	RepeatFrequency  string     `json:"repeatFrequency"`            // ISO 8601 duration (PT5M) or cron expression (*/5 * * * *)
	RepeatCount      *int       `json:"repeatCount,omitempty"`      // Number of times to repeat (nil = infinite)
	StartDate        *time.Time `json:"startDate,omitempty"`        // When to start schedule
	EndDate          *time.Time `json:"endDate,omitempty"`          // When to end schedule
	ScheduleTimezone string     `json:"scheduleTimezone,omitempty"` // IANA timezone (e.g., "America/New_York", "UTC")
	ByDay            []string   `json:"byDay,omitempty"`            // Days of week (Monday, Tuesday, etc.)
	ByMonth          []int      `json:"byMonth,omitempty"`          // Months (1-12)
	ByMonthDay       []int      `json:"byMonthDay,omitempty"`       // Days of month (1-31)
	ExceptDate       []string   `json:"exceptDate,omitempty"`       // Dates to skip (ISO 8601 format)
}

// ActionObject represents the target of an action
type ActionObject struct {
	Type        string `json:"@type"`                 // e.g., "SoftwareApplication" (container), "URL", "ComputerSystem" (host)
	ID          string `json:"@id,omitempty"`         // Container ID, host ID, etc.
	URL         string `json:"url,omitempty"`         // For HTTP actions
	ContentType string `json:"contentType,omitempty"` // Expected content type
}

// ActionResult represents the result of an action execution
type ActionResult struct {
	Type        string                 `json:"@type"`                 // schema:Thing
	Name        string                 `json:"name"`                  // Result name/summary
	Description string                 `json:"description,omitempty"` // Detailed result description
	Value       map[string]interface{} `json:"value,omitempty"`       // Structured result data
	Timestamp   time.Time              `json:"timestamp"`             // When result was generated
	Duration    int64                  `json:"duration"`              // Execution duration in milliseconds
}

// ActionError represents an error from action execution
type ActionError struct {
	Type        string    `json:"@type"`       // schema:Thing
	Name        string    `json:"name"`        // Error name/type
	Description string    `json:"description"` // Error description
	Timestamp   time.Time `json:"timestamp"`   // When error occurred
}

// Action status constants (schema.org ActionStatusType)
const (
	ActionStatusPotential = "PotentialActionStatus" // Not yet executed
	ActionStatusActive    = "ActiveActionStatus"    // Currently executing
	ActionStatusCompleted = "CompletedActionStatus" // Successfully completed
	ActionStatusFailed    = "FailedActionStatus"    // Failed
)

// Action type constants (schema.org Action types)
const (
	ActionTypeCheck    = "CheckAction"    // For health checks, cert checks
	ActionTypeControl  = "ControlAction"  // For start/stop/restart operations
	ActionTypeCreate   = "CreateAction"   // For creating resources
	ActionTypeUpdate   = "UpdateAction"   // For updating configurations
	ActionTypeTransfer = "TransferAction" // For backups, log collection
	ActionTypeAction   = "Action"         // Generic action
)

// NewScheduledAction creates a new scheduled action with defaults
func NewScheduledAction(actionType, name, agent string, schedule *Schedule) *ScheduledAction {
	now := time.Now()
	return &ScheduledAction{
		Context:      "https://schema.org",
		Type:         actionType,
		ID:           GenerateID("action"),
		Name:         name,
		ActionStatus: ActionStatusPotential,
		Agent:        agent,
		Schedule:     schedule,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// IsActive returns true if the action is currently active
func (a *ScheduledAction) IsActive() bool {
	return a.Enabled && a.ActionStatus == ActionStatusActive
}

// IsPending returns true if the action is pending execution
func (a *ScheduledAction) IsPending() bool {
	return a.Enabled && a.ActionStatus == ActionStatusPotential
}

// MarkStarted marks the action as started
func (a *ScheduledAction) MarkStarted() {
	now := time.Now()
	a.StartTime = &now
	a.ActionStatus = ActionStatusActive
	a.UpdatedAt = now
}

// MarkCompleted marks the action as completed with a result
func (a *ScheduledAction) MarkCompleted(result *ActionResult) {
	now := time.Now()
	a.EndTime = &now
	a.Result = result
	a.Error = nil
	a.ActionStatus = ActionStatusCompleted
	a.UpdatedAt = now
}

// MarkFailed marks the action as failed with an error
func (a *ScheduledAction) MarkFailed(err *ActionError) {
	now := time.Now()
	a.EndTime = &now
	a.Error = err
	a.ActionStatus = ActionStatusFailed
	a.UpdatedAt = now
}

// GetNextScheduledTime calculates when this action should next execute
// This is a placeholder - actual implementation will be in the scheduler service
func (a *ScheduledAction) GetNextScheduledTime() *time.Time {
	// This will be implemented by the scheduler service
	// which will parse the RepeatFrequency and calculate next run
	return nil
}
