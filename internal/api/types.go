package api

import (
	"evalgo.org/graphium/models"
)

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// MessageResponse represents a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}

// ContainersResponse represents a list of containers.
type ContainersResponse struct {
	Count      int                 `json:"count"`
	Containers []*models.Container `json:"containers"`
}

// HostsResponse represents a list of hosts.
type HostsResponse struct {
	Count int            `json:"count"`
	Hosts []*models.Host `json:"hosts"`
}

// PaginatedContainersResponse represents a paginated list of containers.
type PaginatedContainersResponse struct {
	Count      int                 `json:"count"`  // Number of items in current page
	Total      int                 `json:"total"`  // Total number of items
	Limit      int                 `json:"limit"`  // Items per page
	Offset     int                 `json:"offset"` // Current offset
	Containers []*models.Container `json:"containers"`
}

// PaginatedHostsResponse represents a paginated list of hosts.
type PaginatedHostsResponse struct {
	Count  int            `json:"count"`  // Number of items in current page
	Total  int            `json:"total"`  // Total number of items
	Limit  int            `json:"limit"`  // Items per page
	Offset int            `json:"offset"` // Current offset
	Hosts  []*models.Host `json:"hosts"`
}

// BulkResult represents the result of a single bulk operation.
type BulkResult struct {
	ID      string `json:"id"`
	Rev     string `json:"rev,omitempty"`
	Error   string `json:"error,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Success bool   `json:"success"`
}

// BulkResponse represents a bulk operation response.
type BulkResponse struct {
	Total   int          `json:"total"`
	Success int          `json:"success"`
	Failed  int          `json:"failed"`
	Results []BulkResult `json:"results"`
}

// WebSocketMessage represents a message sent via WebSocket.
type WebSocketMessage struct {
	Type      string      `json:"type"`   // "container" or "host"
	Action    string      `json:"action"` // "created", "updated", "deleted"
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// AgentInfo represents combined agent configuration and runtime state.
type AgentInfo struct {
	Config *models.AgentConfig `json:"config"`
	State  *models.AgentState  `json:"state"`
}

// AgentListResponse represents a list of agents with their states.
type AgentListResponse struct {
	Count  int         `json:"count"`
	Agents []AgentInfo `json:"agents"`
}
