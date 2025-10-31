package models

import "time"

// AgentState represents the state and metadata of a Graphium agent process.
// Agents monitor Docker hosts and sync container information to the API server.
type AgentState struct {
	// ID is the agent identifier (agent:{host_id})
	ID string `json:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Context is the JSON-LD @context
	Context string `json:"@context"`

	// Type is the JSON-LD @type
	Type string `json:"@type"`

	// HostID is the Docker host this agent is monitoring
	HostID string `json:"hostId"`

	// Status is the agent status (running, stopped, failed, starting, stopping)
	Status string `json:"status"`

	// DockerSocket is the Docker connection string (unix://, ssh://, tcp://)
	DockerSocket string `json:"dockerSocket"`

	// Datacenter is the datacenter location
	Datacenter string `json:"datacenter,omitempty"`

	// Version is the agent version
	Version string `json:"version,omitempty"`

	// StartedAt is when the agent was started
	StartedAt *time.Time `json:"startedAt,omitempty"`

	// StoppedAt is when the agent was stopped
	StoppedAt *time.Time `json:"stoppedAt,omitempty"`

	// LastHeartbeat is the last time the agent checked in
	LastHeartbeat *time.Time `json:"lastHeartbeat"`

	// LastSyncAt is the last time containers were synced
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`

	// ContainerCount is the number of containers discovered
	ContainerCount int `json:"containerCount"`

	// ErrorMessage contains error details if status is failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// ProcessID is the OS process ID (PID)
	ProcessID int `json:"processId,omitempty"`

	// Hostname is the machine hostname where agent is running
	Hostname string `json:"hostname,omitempty"`

	// SyncInterval is the sync interval in seconds
	SyncInterval int `json:"syncInterval,omitempty"`

	// Metrics contains agent performance metrics
	Metrics *AgentMetrics `json:"metrics,omitempty"`
}

// AgentMetrics contains performance and health metrics for an agent.
type AgentMetrics struct {
	// TotalSyncOperations is the total number of sync operations performed
	TotalSyncOperations int64 `json:"totalSyncOperations"`

	// FailedSyncOperations is the number of failed sync operations
	FailedSyncOperations int64 `json:"failedSyncOperations"`

	// AverageSyncDuration is the average sync duration in milliseconds
	AverageSyncDuration float64 `json:"averageSyncDuration"`

	// LastSyncDuration is the duration of the last sync in milliseconds
	LastSyncDuration float64 `json:"lastSyncDuration"`

	// EventsProcessed is the total number of Docker events processed
	EventsProcessed int64 `json:"eventsProcessed"`

	// UptimeSeconds is the agent uptime in seconds
	UptimeSeconds int64 `json:"uptimeSeconds"`
}
