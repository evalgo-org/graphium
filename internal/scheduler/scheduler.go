package scheduler

import (
	"fmt"
	"log"
	"strings"
	"time"

	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// Scheduler manages scheduled actions and creates tasks for execution
type Scheduler struct {
	storage *storage.Storage
	ticker  *time.Ticker
	stop    chan bool
	running bool
}

// New creates a new scheduler instance
func New(store *storage.Storage) *Scheduler {
	return &Scheduler{
		storage: store,
		stop:    make(chan bool),
		running: false,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start() {
	if s.running {
		log.Println("Scheduler already running")
		return
	}

	s.running = true
	s.ticker = time.NewTicker(30 * time.Second) // Evaluate every 30 seconds

	log.Println("Scheduler started - evaluating schedules every 30 seconds")

	go func() {
		// Evaluate immediately on start
		s.evaluateSchedules()

		for {
			select {
			case <-s.ticker.C:
				s.evaluateSchedules()
			case <-s.stop:
				s.ticker.Stop()
				s.running = false
				log.Println("Scheduler stopped")
				return
			}
		}
	}()
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
	if s.running {
		s.stop <- true
	}
}

// evaluateSchedules checks all enabled scheduled actions and creates tasks if needed
func (s *Scheduler) evaluateSchedules() {
	// Get all enabled scheduled actions
	actions, err := s.storage.GetActiveScheduledActions()
	if err != nil {
		log.Printf("Error getting scheduled actions: %v\n", err)
		return
	}

	if len(actions) == 0 {
		return
	}

	log.Printf("Evaluating %d scheduled action(s)\n", len(actions))

	now := time.Now()

	for _, action := range actions {
		// Skip if currently executing
		if action.ActionStatus == models.ActionStatusActive {
			continue
		}

		// Determine if action should execute now
		if s.shouldExecute(action, now) {
			log.Printf("Executing scheduled action: %s (type: %s)\n", action.Name, action.Type)

			// Create task from action
			task, err := s.createTaskFromAction(action)
			if err != nil {
				log.Printf("Error creating task from action %s: %v\n", action.ID, err)
				continue
			}

			// Create the task
			if err := s.storage.CreateTask(task); err != nil {
				log.Printf("Error creating task for action %s: %v\n", action.ID, err)
				continue
			}

			// Update action status
			action.MarkStarted()
			if err := s.storage.UpdateScheduledAction(action); err != nil {
				log.Printf("Error updating action %s: %v\n", action.ID, err)
			}

			log.Printf("Created task %s for scheduled action %s\n", task.ID, action.ID)
		}
	}
}

// shouldExecute determines if an action should execute at the given time
func (s *Scheduler) shouldExecute(action *models.ScheduledAction, now time.Time) bool {
	schedule := action.Schedule
	if schedule == nil {
		return false
	}

	// Check if schedule has ended
	if schedule.EndDate != nil && now.After(*schedule.EndDate) {
		return false
	}

	// Check if schedule hasn't started yet
	if schedule.StartDate != nil && now.Before(*schedule.StartDate) {
		return false
	}

	// Check if repeat count has been reached
	if schedule.RepeatCount != nil && *schedule.RepeatCount <= 0 {
		return false
	}

	// Check except dates
	if s.isExceptDate(now, schedule.ExceptDate) {
		return false
	}

	// Get timezone
	loc, err := time.LoadLocation(schedule.ScheduleTimezone)
	if err != nil {
		loc = time.UTC
	}
	now = now.In(loc)

	// Check by day, month, monthday constraints
	if !s.matchesDayConstraints(now, schedule) {
		return false
	}

	// Determine last execution time
	lastExecution := action.StartTime
	if lastExecution == nil {
		// Never executed before - check if we should execute now
		return s.shouldExecuteFirstTime(action, now)
	}

	// Calculate next execution time based on repeat frequency
	nextExecution := s.calculateNextExecution(*lastExecution, schedule)
	if nextExecution == nil {
		return false
	}

	// Execute if current time is at or past next execution time
	return now.After(*nextExecution) || now.Equal(*nextExecution)
}

// shouldExecuteFirstTime determines if an action should execute for the first time
func (s *Scheduler) shouldExecuteFirstTime(action *models.ScheduledAction, now time.Time) bool {
	schedule := action.Schedule

	// If there's a start date, check if we've passed it
	if schedule.StartDate != nil {
		if now.Before(*schedule.StartDate) {
			return false
		}
		// If we're past the start date, execute now
		return true
	}

	// No start date - execute now
	return true
}

// calculateNextExecution calculates when the action should next execute
func (s *Scheduler) calculateNextExecution(lastExecution time.Time, schedule *Schedule) *time.Time {
	// Parse repeat frequency
	duration, isCron := s.parseRepeatFrequency(schedule.RepeatFrequency)

	if isCron {
		// For cron expressions, we need a cron parser
		// For now, return nil - we'll implement this next
		return nil
	}

	// Add duration to last execution
	next := lastExecution.Add(duration)
	return &next
}

// parseRepeatFrequency parses ISO 8601 duration or cron expression
// Returns duration and bool indicating if it's a cron expression
func (s *Scheduler) parseRepeatFrequency(freq string) (time.Duration, bool) {
	// Check if it's a cron expression (contains spaces)
	if strings.Contains(freq, " ") {
		return 0, true
	}

	// Parse ISO 8601 duration
	duration, err := parseISO8601Duration(freq)
	if err != nil {
		log.Printf("Error parsing repeat frequency '%s': %v\n", freq, err)
		return 0, false
	}

	return duration, false
}

// parseISO8601Duration parses ISO 8601 duration strings
// Examples: PT5M (5 minutes), PT1H (1 hour), P1D (1 day), P1W (1 week)
func parseISO8601Duration(duration string) (time.Duration, error) {
	if duration == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Simple parser for common cases
	// Full ISO 8601 parser would be more complex
	switch {
	case strings.HasPrefix(duration, "PT"):
		// Time duration
		timepart := duration[2:]
		if strings.HasSuffix(timepart, "S") {
			// Seconds
			var seconds int
			fmt.Sscanf(timepart, "%dS", &seconds)
			return time.Duration(seconds) * time.Second, nil
		} else if strings.HasSuffix(timepart, "M") {
			// Minutes
			var minutes int
			fmt.Sscanf(timepart, "%dM", &minutes)
			return time.Duration(minutes) * time.Minute, nil
		} else if strings.HasSuffix(timepart, "H") {
			// Hours
			var hours int
			fmt.Sscanf(timepart, "%dH", &hours)
			return time.Duration(hours) * time.Hour, nil
		}
	case strings.HasPrefix(duration, "P"):
		// Date duration
		datepart := duration[1:]
		if strings.HasSuffix(datepart, "D") {
			// Days
			var days int
			fmt.Sscanf(datepart, "%dD", &days)
			return time.Duration(days) * 24 * time.Hour, nil
		} else if strings.HasSuffix(datepart, "W") {
			// Weeks
			var weeks int
			fmt.Sscanf(datepart, "%dW", &weeks)
			return time.Duration(weeks) * 7 * 24 * time.Hour, nil
		} else if strings.HasSuffix(datepart, "M") {
			// Months (approximate - 30 days)
			var months int
			fmt.Sscanf(datepart, "%dM", &months)
			return time.Duration(months) * 30 * 24 * time.Hour, nil
		}
	}

	return 0, fmt.Errorf("unsupported duration format: %s", duration)
}

// matchesDayConstraints checks if the given time matches day/month constraints
func (s *Scheduler) matchesDayConstraints(now time.Time, schedule *Schedule) bool {
	// Check by month
	if len(schedule.ByMonth) > 0 {
		month := int(now.Month())
		found := false
		for _, m := range schedule.ByMonth {
			if m == month {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check by month day
	if len(schedule.ByMonthDay) > 0 {
		day := now.Day()
		found := false
		for _, d := range schedule.ByMonthDay {
			if d == day {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check by weekday
	if len(schedule.ByDay) > 0 {
		weekday := now.Weekday().String()
		found := false
		for _, d := range schedule.ByDay {
			if strings.EqualFold(d, weekday) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// isExceptDate checks if the given date is in the except list
func (s *Scheduler) isExceptDate(now time.Time, exceptDates []string) bool {
	dateStr := now.Format("2006-01-02")
	for _, except := range exceptDates {
		if strings.HasPrefix(except, dateStr) {
			return true
		}
	}
	return false
}

// createTaskFromAction creates a Task from a ScheduledAction
func (s *Scheduler) createTaskFromAction(action *models.ScheduledAction) (*models.Task, error) {
	task := &models.Task{
		Type:        "AgentTask",
		ID:          models.GenerateID("task"),
		HostID:      action.Agent,
		AgentID:     action.Agent,
		Status:      models.TaskStatusPending,
		ScheduledBy: action.ID, // Link task to the action that created it
	}

	// Check if this is a composite action (workflow)
	isCompositeAction := false
	if action.Instrument != nil {
		if compositeVal, ok := action.Instrument["compositeAction"]; ok {
			isCompositeAction, _ = compositeVal.(bool)
		}
	}

	// Set task type based on whether it's a composite action (workflow)
	if isCompositeAction {
		task.TaskType = "workflow"
	} else {
		// Map action type to task type
		switch action.Type {
		case models.ActionTypeCheck:
			task.TaskType = "check"
		case models.ActionTypeControl:
			task.TaskType = "control"
		case models.ActionTypeCreate:
			task.TaskType = "create"
		case models.ActionTypeUpdate:
			task.TaskType = "update"
		case models.ActionTypeTransfer:
			task.TaskType = "transfer"
		default:
			task.TaskType = "action"
		}
	}

	// Build payload from action instrument and object
	payload := make(map[string]interface{})

	// Copy parameters from action instrument
	if action.Instrument != nil {
		for k, v := range action.Instrument {
			payload[k] = v
		}
	}

	// Add object information to payload
	if action.Object != nil {
		payload["object"] = action.Object
	}

	// Set the payload
	if len(payload) > 0 {
		if err := task.SetPayload(payload); err != nil {
			return nil, fmt.Errorf("failed to set task payload: %w", err)
		}
	}

	return task, nil
}

// Alias Schedule to models.Schedule for convenience
type Schedule = models.Schedule
