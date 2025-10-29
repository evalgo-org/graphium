package models

import "time"

// Stack represents a multi-container application deployment.
// It follows the Schema.org ItemList type with container orchestration extensions.
//
// JSON-LD Context: https://schema.org
// Type: ItemList
//
// Stacks can be deployed in single-host or distributed (multi-host) mode.
// In distributed mode, containers can be placed on different hosts based on
// placement strategies (auto, manual, datacenter, spread).
//
// Example JSON representation:
//
//	{
//	  "@context": "https://schema.org",
//	  "@type": "ItemList",
//	  "@id": "my-app-stack",
//	  "name": "my-app",
//	  "description": "My application stack",
//	  "status": "running",
//	  "deployment": {
//	    "mode": "multi-host",
//	    "placementStrategy": "spread"
//	  },
//	  "dateCreated": "2025-10-29T10:00:00Z"
//	}
type Stack struct {
	// Context is the JSON-LD @context URL
	Context string `json:"@context" jsonld:"@context"`

	// Type is the JSON-LD @type (ItemList for stacks)
	Type string `json:"@type" jsonld:"@type"`

	// ID is the unique stack identifier (maps to CouchDB _id)
	ID string `json:"@id" jsonld:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Name is the stack name (required, indexed, DNS-compatible)
	Name string `json:"name" jsonld:"name" couchdb:"required,index"`

	// Description is the human-readable stack description
	Description string `json:"description,omitempty" jsonld:"description"`

	// Status is the stack operational status
	// Values: pending, deploying, running, stopping, stopped, error
	Status string `json:"status" jsonld:"status" couchdb:"index"`

	// Datacenter is the primary datacenter for this stack (optional)
	Datacenter string `json:"location,omitempty" jsonld:"location" couchdb:"index"`

	// Deployment contains deployment configuration
	Deployment DeploymentConfig `json:"deployment,omitempty"`

	// Containers is a list of container references in this stack
	Containers []string `json:"containers,omitempty" jsonld:"itemListElement"`

	// DefinitionPath is the path to the stack definition file
	DefinitionPath string `json:"definitionPath,omitempty"`

	// DeploymentID is the Docker deployment identifier
	DeploymentID string `json:"deploymentId,omitempty"`

	// CreatedAt is the stack creation timestamp
	CreatedAt time.Time `json:"dateCreated" jsonld:"dateCreated" couchdb:"index"`

	// UpdatedAt is the last update timestamp
	UpdatedAt time.Time `json:"dateModified" jsonld:"dateModified"`

	// DeployedAt is the deployment timestamp
	DeployedAt *time.Time `json:"deployedAt,omitempty"`

	// Owner is the user who created the stack
	Owner string `json:"owner,omitempty" jsonld:"creator"`

	// Labels are custom key-value labels
	Labels map[string]string `json:"labels,omitempty"`

	// ErrorMessage contains error details if status is "error"
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// DeploymentConfig defines how a stack should be deployed.
type DeploymentConfig struct {
	// Mode is the deployment mode: "single-host" or "multi-host"
	Mode string `json:"mode"`

	// PlacementStrategy defines how containers are placed on hosts
	// Values: "auto", "manual", "datacenter", "spread"
	PlacementStrategy string `json:"placementStrategy,omitempty"`

	// HostConstraints define placement rules per container
	HostConstraints []HostConstraint `json:"hostConstraints,omitempty"`

	// NetworkMode defines cross-host networking
	// Values: "host-port" (exposed ports), "overlay" (Docker overlay network)
	NetworkMode string `json:"networkMode,omitempty"`
}

// HostConstraint defines placement rules for a container.
type HostConstraint struct {
	// ContainerName is the name of the container to constrain
	ContainerName string `json:"containerName"`

	// TargetHostID is the specific host ID (for manual placement)
	TargetHostID string `json:"targetHost,omitempty"`

	// RequiredDatacenter requires the container to be in this datacenter
	RequiredDatacenter string `json:"requiredDatacenter,omitempty"`

	// MinCPU is the minimum CPU cores required
	MinCPU int `json:"minCpu,omitempty"`

	// MinMemory is the minimum memory in bytes required
	MinMemory int64 `json:"minMemory,omitempty"`

	// Labels are custom labels that the host must have
	Labels map[string]string `json:"labels,omitempty"`
}

// StackDeployment represents the runtime state of a deployed stack.
type StackDeployment struct {
	// StackID is the reference to the Stack model
	StackID string `json:"stackId"`

	// Placements maps container names to their host placements
	Placements map[string]ContainerPlacement `json:"placements"`

	// NetworkConfig contains cross-host networking details
	NetworkConfig NetworkConfig `json:"networkConfig"`

	// StartedAt is when the deployment started
	StartedAt time.Time `json:"startedAt"`

	// CompletedAt is when the deployment completed
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Status is the deployment status
	Status string `json:"status"`

	// ErrorMessage contains error details if deployment failed
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ContainerPlacement represents where a container was placed.
type ContainerPlacement struct {
	// ContainerID is the Docker container ID
	ContainerID string `json:"containerId"`

	// ContainerName is the container name
	ContainerName string `json:"containerName"`

	// HostID is the host where the container is running
	HostID string `json:"hostId"`

	// IPAddress is the host IP address
	IPAddress string `json:"ipAddress"`

	// Ports maps container ports to exposed host ports
	Ports map[int]int `json:"ports"`

	// Status is the container status
	Status string `json:"status"`

	// StartedAt is when the container started
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// NetworkConfig contains cross-host networking configuration.
type NetworkConfig struct {
	// Mode is the networking mode (host-port or overlay)
	Mode string `json:"mode"`

	// OverlayNetworkID is the Docker overlay network ID (if mode is overlay)
	OverlayNetworkID string `json:"overlayNetworkId,omitempty"`

	// ServiceEndpoints maps container names to their connection endpoints
	// Format: {"postgres": "192.168.1.10:5432", "redis": "192.168.1.11:6379"}
	ServiceEndpoints map[string]string `json:"serviceEndpoints"`

	// EnvironmentVariables contains injected environment variables for cross-host connections
	EnvironmentVariables map[string]map[string]string `json:"environmentVariables"`
}

// HostInfo contains host metadata for placement decisions.
type HostInfo struct {
	// Host is the host model
	Host *Host `json:"host"`

	// DockerSocket is the Docker socket URL
	DockerSocket string `json:"dockerSocket"`

	// CurrentLoad contains current resource usage
	CurrentLoad ResourceLoad `json:"currentLoad"`

	// AvailableResources contains available resources
	AvailableResources Resources `json:"availableResources"`

	// Labels are custom host labels
	Labels map[string]string `json:"labels,omitempty"`
}

// ResourceLoad represents current resource usage.
type ResourceLoad struct {
	// CPUUsage is CPU usage percentage (0-100)
	CPUUsage float64 `json:"cpuUsage"`

	// MemoryUsage is memory usage in bytes
	MemoryUsage int64 `json:"memoryUsage"`

	// ContainerCount is the number of running containers
	ContainerCount int `json:"containerCount"`
}

// Resources represents available resources.
type Resources struct {
	// CPU is available CPU cores
	CPU int `json:"cpu"`

	// Memory is available memory in bytes
	Memory int64 `json:"memory"`
}
