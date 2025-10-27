// Package models defines the core data models for Graphium using JSON-LD (Linked Data).
// These models follow Schema.org vocabulary for semantic container orchestration.
//
// All models use JSON-LD annotations to enable semantic querying and graph traversal
// across the container infrastructure. The models map to both CouchDB storage and
// REST API representations.
package models

//go:generate go run ../tools/generate.go

// Container represents a containerized application running on a host system.
// It follows the Schema.org SoftwareApplication type with additional container-specific fields.
//
// JSON-LD Context: https://schema.org
// Type: SoftwareApplication
//
// The Container model includes:
//   - Basic identification (@id, name)
//   - Container runtime details (image, status)
//   - Host relationship (hostedOn)
//   - Network configuration (ports)
//   - Environment variables (env)
//
// Example JSON representation:
//
//	{
//	  "@context": "https://schema.org",
//	  "@type": "SoftwareApplication",
//	  "@id": "container-abc123",
//	  "name": "web-server",
//	  "executableName": "nginx:latest",
//	  "status": "running",
//	  "hostedOn": "host-01",
//	  "ports": [
//	    {"hostPort": 8080, "containerPort": 80, "protocol": "tcp"}
//	  ]
//	}
type Container struct {
	// Context is the JSON-LD @context URL (typically https://schema.org)
	Context string `json:"@context" jsonld:"@context"`

	// Type is the JSON-LD @type (SoftwareApplication for containers)
	Type string `json:"@type" jsonld:"@type"`

	// ID is the unique container identifier (maps to CouchDB _id)
	ID string `json:"@id" jsonld:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision for optimistic locking
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Name is the human-readable container name (required, indexed)
	Name string `json:"name" jsonld:"name" couchdb:"required,index"`

	// Image is the container image name (executableName in Schema.org)
	Image string `json:"executableName" jsonld:"executableName" couchdb:"required"`

	// Status is the container runtime status (running, stopped, paused, etc.)
	Status string `json:"status" jsonld:"status" couchdb:"index"`

	// HostedOn is the ID of the host running this container (creates graph relationship)
	HostedOn string `json:"hostedOn" jsonld:"hostedOn" couchdb:"relation,index"`

	// Ports are the network port mappings for this container
	Ports []Port `json:"ports,omitempty" jsonld:"ports"`

	// Env contains environment variables passed to the container
	Env map[string]string `json:"environment,omitempty" jsonld:"environment"`

	// Created is the ISO 8601 timestamp when the container was created
	Created string `json:"dateCreated,omitempty" jsonld:"dateCreated"`
}

// Port represents a network port mapping between host and container.
// It maps host ports to container ports with a specific protocol (tcp/udp).
type Port struct {
	// HostPort is the port number on the host machine
	HostPort int `json:"hostPort" jsonld:"hostPort"`

	// ContainerPort is the port number inside the container
	ContainerPort int `json:"containerPort" jsonld:"containerPort"`

	// Protocol is the network protocol (tcp, udp, sctp)
	Protocol string `json:"protocol" jsonld:"protocol"`
}
