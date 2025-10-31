package models

import "time"

// AgentConfig represents the configuration for an agent managed by the Graphium server.
// This is the persistent configuration stored in the database.
type AgentConfig struct {
	// ID is the agent identifier (agent:{host_id})
	ID string `json:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Context is the JSON-LD @context
	Context string `json:"@context"`

	// Type is the JSON-LD @type (datacenter:AgentConfig)
	Type string `json:"@type"`

	// Name is a friendly name for the agent
	Name string `json:"name"`

	// HostID is the Docker host this agent monitors
	HostID string `json:"hostId"`

	// DockerSocket is the Docker connection string (unix://, ssh://, tcp://)
	DockerSocket string `json:"dockerSocket"`

	// SSHKeyPath is the path to the SSH private key for remote Docker connections
	// Used when DockerSocket is ssh://user@host format
	SSHKeyPath string `json:"sshKeyPath,omitempty"`

	// Datacenter is the datacenter location
	Datacenter string `json:"datacenter,omitempty"`

	// SyncInterval is the sync interval in seconds (default: 30)
	SyncInterval int `json:"syncInterval,omitempty"`

	// AutoStart determines if agent should start with server
	AutoStart bool `json:"autoStart"`

	// Enabled determines if agent is enabled
	Enabled bool `json:"enabled"`

	// Created is when the agent config was created
	Created time.Time `json:"dateCreated"`

	// Modified is when the agent config was last modified
	Modified time.Time `json:"dateModified"`
}

// AgentState represents the runtime state of an agent process.
// This tracks the current state of a running (or stopped) agent.
type AgentState struct {
	// ConfigID is the reference to AgentConfig
	ConfigID string `json:"configId"`

	// Status is the agent status (running, stopped, failed, starting, stopping)
	Status string `json:"status"`

	// StartedAt is when the agent was started
	StartedAt *time.Time `json:"startedAt,omitempty"`

	// StoppedAt is when the agent was stopped
	StoppedAt *time.Time `json:"stoppedAt,omitempty"`

	// LastHeartbeat is the last time the agent checked in
	LastHeartbeat *time.Time `json:"lastHeartbeat,omitempty"`

	// LastSyncAt is the last time containers were synced
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`

	// ContainerCount is the number of containers discovered
	ContainerCount int `json:"containerCount"`

	// ErrorMessage contains error details if status is failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// ProcessID is the OS process ID (PID)
	ProcessID int `json:"processId,omitempty"`

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
