package api

import (
	"evalgo.org/graphium/models"
	"eve.evalgo.org/db"
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

// BulkResponse represents a bulk operation response.
type BulkResponse struct {
	Total   int             `json:"total"`
	Success int             `json:"success"`
	Failed  int             `json:"failed"`
	Results []db.BulkResult `json:"results"`
}

// WebSocketMessage represents a message sent via WebSocket.
type WebSocketMessage struct {
	Type      string      `json:"type"`   // "container" or "host"
	Action    string      `json:"action"` // "created", "updated", "deleted"
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}
