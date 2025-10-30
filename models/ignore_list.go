package models

import "time"

// IgnoreListEntry represents a container that should be ignored by the agent.
// This prevents the agent from re-syncing containers that were intentionally
// deleted by users via the UI.
type IgnoreListEntry struct {
	// Context is the JSON-LD context
	Context string `json:"@context,omitempty" couchdb:"index"`

	// Type is the JSON-LD type
	Type string `json:"@type,omitempty" couchdb:"index"`

	// ID is the unique identifier for this entry (container ID)
	ID string `json:"_id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// ContainerID is the Docker container ID to ignore
	ContainerID string `json:"container_id" couchdb:"required,index"`

	// HostID is the host where this container was located
	HostID string `json:"host_id,omitempty" couchdb:"index"`

	// Reason explains why this container is ignored
	Reason string `json:"reason,omitempty"`

	// CreatedAt is when this entry was added
	CreatedAt time.Time `json:"created_at" couchdb:"index"`

	// ExpiresAt is optional expiration time (for temporary ignores)
	ExpiresAt *time.Time `json:"expires_at,omitempty" couchdb:"index"`

	// CreatedBy is the user who added this entry
	CreatedBy string `json:"created_by,omitempty"`
}
