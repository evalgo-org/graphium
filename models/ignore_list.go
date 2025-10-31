package models

import "time"

// IgnoreListEntry represents a container that should be ignored by agents.
// Containers in the ignore list will not be synced or monitored.
type IgnoreListEntry struct {
	// Context is the JSON-LD @context
	Context string `json:"@context"`

	// Type is the JSON-LD @type
	Type string `json:"@type"`

	// ID is the document ID (ignore-{containerID})
	ID string `json:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// ContainerID is the ID of the container to ignore
	ContainerID string `json:"containerId"`

	// HostID is the host where the container is located
	HostID string `json:"hostId"`

	// Reason describes why this container is being ignored
	Reason string `json:"reason"`

	// CreatedBy indicates who added this entry
	CreatedBy string `json:"createdBy"`

	// CreatedAt is when this entry was created
	CreatedAt time.Time `json:"dateCreated"`
}
