package models

import "time"

// StackDefinition represents a complete stack deployment definition using JSON-LD @graph structure.
// This is the new format that supports full declarative infrastructure as code.
//
// Example structure:
//
//	{
//	  "@context": [...],
//	  "@graph": [
//	    {
//	      "@id": "https://example.com/stacks/my-stack",
//	      "@type": ["datacenter:Stack", "SoftwareApplication"],
//	      "name": "my-stack",
//	      "hasPart": [...]
//	    }
//	  ]
//	}
type StackDefinition struct {
	// Context is the JSON-LD @context (can be string, array, or object)
	Context interface{} `json:"@context"`

	// Graph is the array of JSON-LD graph nodes
	Graph []GraphNode `json:"@graph"`
}

// GraphNode represents a node in the JSON-LD graph.
// It can represent a Stack, Container, Host, Rack, or Datacenter.
type GraphNode struct {
	// ID is the unique identifier (@id in JSON-LD)
	ID string `json:"@id"`

	// Type is the JSON-LD @type (can be string or array)
	Type interface{} `json:"@type"`

	// Name is the human-readable name
	Name string `json:"name,omitempty"`

	// Description is the human-readable description
	Description string `json:"description,omitempty"`

	// Stack-specific fields
	LocatedInHost *Reference      `json:"locatedInHost,omitempty"`
	Network       *NetworkSpec    `json:"network,omitempty"`
	HasPart       []ContainerSpec `json:"hasPart,omitempty"`

	// Host-specific fields
	UPosition     string     `json:"uPosition,omitempty"`
	LocatedInRack *Reference `json:"locatedInRack,omitempty"`

	// Rack-specific fields
	RackPosition        string     `json:"rackPosition,omitempty"`
	LocatedInDatacenter *Reference `json:"locatedInDatacenter,omitempty"`

	// Additional metadata
	DateCreated  *time.Time `json:"dateCreated,omitempty"`
	DateModified *time.Time `json:"dateModified,omitempty"`
	Creator      string     `json:"creator,omitempty"`
}

// ContainerSpec defines a complete container specification within a stack.
type ContainerSpec struct {
	// ID is the unique identifier for this container
	ID string `json:"@id"`

	// Type is the JSON-LD @type (e.g., ["datacenter:Container", "SoftwareApplication"])
	Type interface{} `json:"@type"`

	// Name is the container name (will be prefixed with stack name)
	Name string `json:"name"`

	// ApplicationCategory describes the container role (e.g., "DatabaseApplication")
	ApplicationCategory string `json:"applicationCategory,omitempty"`

	// Image is the Docker image (e.g., "postgres:15", "nginx:alpine")
	Image string `json:"image"`

	// Environment contains environment variables
	Environment map[string]string `json:"environment,omitempty"`

	// Ports defines port mappings
	Ports []PortMapping `json:"ports,omitempty"`

	// VolumeMounts defines volume mounts
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`

	// HealthCheck defines the health check configuration
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`

	// LocatedInHost specifies the target host (for multi-host deployments)
	LocatedInHost *Reference `json:"locatedInHost,omitempty"`

	// DependsOn lists container dependencies (for startup ordering)
	DependsOn []string `json:"dependsOn,omitempty"`

	// RestartPolicy defines the restart behavior (no, always, on-failure, unless-stopped)
	RestartPolicy string `json:"restartPolicy,omitempty"`

	// Command overrides the default container command
	Command []string `json:"command,omitempty"`

	// Args provides arguments to the command
	Args []string `json:"args,omitempty"`

	// WorkingDir sets the working directory
	WorkingDir string `json:"workingDir,omitempty"`

	// User specifies the user to run as
	User string `json:"user,omitempty"`

	// Labels are custom container labels
	Labels map[string]string `json:"labels,omitempty"`

	// Resources defines resource constraints
	Resources *ResourceConstraints `json:"resources,omitempty"`
}

// NetworkSpec defines network configuration for the stack.
type NetworkSpec struct {
	// Name is the network name
	Name string `json:"name"`

	// Driver is the network driver (bridge, overlay, host, macvlan)
	Driver string `json:"driver"`

	// CreateIfNotExists creates the network if it doesn't exist
	CreateIfNotExists bool `json:"createIfNotExists"`

	// External indicates this is an externally managed network
	External bool `json:"external,omitempty"`

	// Subnet is the network subnet (e.g., "172.18.0.0/16")
	Subnet string `json:"subnet,omitempty"`

	// Gateway is the network gateway IP
	Gateway string `json:"gateway,omitempty"`

	// IPRange is the IP address range for containers
	IPRange string `json:"ipRange,omitempty"`

	// Options are driver-specific options
	Options map[string]string `json:"options,omitempty"`

	// Labels are custom network labels
	Labels map[string]string `json:"labels,omitempty"`
}

// PortMapping defines how container ports are mapped to host ports.
type PortMapping struct {
	// ContainerPort is the port inside the container
	ContainerPort int `json:"containerPort"`

	// HostPort is the port on the host (0 for dynamic allocation)
	HostPort int `json:"hostPort"`

	// Protocol is the port protocol (tcp, udp, sctp)
	Protocol string `json:"protocol,omitempty"`

	// HostIP binds to a specific host IP (empty for all interfaces)
	HostIP string `json:"hostIP,omitempty"`
}

// VolumeMount defines a volume or bind mount.
type VolumeMount struct {
	// Source is the volume name or host path
	Source string `json:"source"`

	// Target is the container mount path
	Target string `json:"target"`

	// Type is the mount type (volume, bind, tmpfs, npipe)
	Type string `json:"type"`

	// ReadOnly makes the mount read-only
	ReadOnly bool `json:"readOnly,omitempty"`

	// VolumeOptions are options for volume mounts
	VolumeOptions *VolumeOptions `json:"volumeOptions,omitempty"`

	// BindOptions are options for bind mounts
	BindOptions *BindOptions `json:"bindOptions,omitempty"`
}

// VolumeOptions contains options for volume mounts.
type VolumeOptions struct {
	// NoCopy disables copying data from container to volume
	NoCopy bool `json:"noCopy,omitempty"`

	// Labels are custom volume labels
	Labels map[string]string `json:"labels,omitempty"`

	// DriverConfig specifies the volume driver
	DriverConfig *VolumeDriverConfig `json:"driverConfig,omitempty"`
}

// VolumeDriverConfig specifies volume driver configuration.
type VolumeDriverConfig struct {
	// Name is the driver name
	Name string `json:"name,omitempty"`

	// Options are driver-specific options
	Options map[string]string `json:"options,omitempty"`
}

// BindOptions contains options for bind mounts.
type BindOptions struct {
	// Propagation is the bind propagation mode (rprivate, private, rshared, shared, rslave, slave)
	Propagation string `json:"propagation,omitempty"`

	// NonRecursive disables recursive bind mounting
	NonRecursive bool `json:"nonRecursive,omitempty"`
}

// HealthCheck defines container health check configuration.
type HealthCheck struct {
	// Type is the health check type (http, tcp, exec, grpc)
	Type string `json:"type"`

	// Path is the HTTP path for http checks (e.g., "/health")
	Path string `json:"path,omitempty"`

	// Port is the port to check
	Port int `json:"port,omitempty"`

	// Command is the command to execute for exec checks
	Command []string `json:"command,omitempty"`

	// Interval is the time between health checks in seconds
	Interval int `json:"interval"`

	// Timeout is the health check timeout in seconds
	Timeout int `json:"timeout"`

	// Retries is the number of consecutive failures before unhealthy
	Retries int `json:"retries"`

	// StartPeriod is the initialization time before health checks start (seconds)
	StartPeriod int `json:"startPeriod"`

	// Headers are HTTP headers for http checks
	Headers map[string]string `json:"headers,omitempty"`
}

// ResourceConstraints defines CPU and memory limits.
type ResourceConstraints struct {
	// Limits defines maximum resource usage
	Limits *ResourceLimits `json:"limits,omitempty"`

	// Reservations defines guaranteed resource allocation
	Reservations *ResourceReservations `json:"reservations,omitempty"`
}

// ResourceLimits defines maximum resource usage.
type ResourceLimits struct {
	// CPUs is the maximum CPU cores (e.g., 0.5, 2.0)
	CPUs float64 `json:"cpus,omitempty"`

	// Memory is the maximum memory in bytes
	Memory int64 `json:"memory,omitempty"`

	// MemorySwap is the maximum memory + swap in bytes (-1 for unlimited)
	MemorySwap int64 `json:"memorySwap,omitempty"`

	// Pids is the maximum number of PIDs
	Pids int64 `json:"pids,omitempty"`
}

// ResourceReservations defines guaranteed resource allocation.
type ResourceReservations struct {
	// CPUs is the guaranteed CPU cores
	CPUs float64 `json:"cpus,omitempty"`

	// Memory is the guaranteed memory in bytes
	Memory int64 `json:"memory,omitempty"`
}

// Reference represents a JSON-LD @id reference to another node.
type Reference struct {
	ID string `json:"@id"`
}

// DeploymentPlan represents the parsed and resolved deployment plan.
type DeploymentPlan struct {
	// StackNode is the main stack graph node
	StackNode *GraphNode

	// ContainerSpecs are all container specifications
	ContainerSpecs []ContainerSpec

	// HostMap maps container @id to host @id
	HostMap map[string]string

	// Network is the network specification
	Network *NetworkSpec

	// Topology contains the infrastructure topology
	Topology *Topology

	// DependencyGraph is the container startup order
	DependencyGraph [][]string // Each inner slice is a deployment wave
}

// Topology represents the infrastructure topology.
type Topology struct {
	// Hosts maps host @id to host graph node
	Hosts map[string]*GraphNode

	// Racks maps rack @id to rack graph node
	Racks map[string]*GraphNode

	// Datacenters maps datacenter @id to datacenter graph node
	Datacenters map[string]*GraphNode
}

// DeploymentState tracks the real-time state of a stack deployment.
type DeploymentState struct {
	// ID is the deployment ID (maps to CouchDB _id)
	ID string `json:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Type is the JSON-LD @type
	Type string `json:"@type"`

	// StackID is the stack identifier
	StackID string `json:"stackId"`

	// Status is the deployment status (deploying, running, stopping, stopped, failed, rolling-back)
	Status string `json:"status"`

	// Phase is the current deployment phase
	Phase string `json:"phase,omitempty"`

	// Progress is the deployment progress (0-100)
	Progress int `json:"progress,omitempty"`

	// Placements maps container names to their placements
	Placements map[string]*ContainerPlacement `json:"placements"`

	// NetworkInfo contains network configuration details
	NetworkInfo *DeployedNetworkInfo `json:"networkInfo,omitempty"`

	// VolumeInfo contains volume information
	VolumeInfo map[string]*VolumeInfo `json:"volumeInfo,omitempty"`

	// Events tracks deployment events
	Events []DeploymentEvent `json:"events,omitempty"`

	// StartedAt is when deployment started
	StartedAt time.Time `json:"startedAt"`

	// CompletedAt is when deployment completed
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// ErrorMessage contains error details if deployment failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// RollbackState tracks rollback if needed
	RollbackState *RollbackState `json:"rollbackState,omitempty"`
}

// NOTE: ContainerPlacement is defined in stack.go and is shared between old and new models

// DeployedNetworkInfo contains information about the deployed network.
type DeployedNetworkInfo struct {
	// NetworkID is the Docker network ID
	NetworkID string `json:"networkId"`

	// NetworkName is the network name
	NetworkName string `json:"networkName"`

	// Driver is the network driver
	Driver string `json:"driver"`

	// Subnet is the network subnet
	Subnet string `json:"subnet,omitempty"`

	// Gateway is the network gateway
	Gateway string `json:"gateway,omitempty"`

	// Scope is the network scope (local, swarm, global)
	Scope string `json:"scope,omitempty"`
}

// VolumeInfo contains information about a deployed volume.
type VolumeInfo struct {
	// VolumeName is the volume name
	VolumeName string `json:"volumeName"`

	// Driver is the volume driver
	Driver string `json:"driver"`

	// Mountpoint is the volume mount point on the host
	Mountpoint string `json:"mountpoint,omitempty"`

	// Scope is the volume scope
	Scope string `json:"scope,omitempty"`

	// CreatedAt is when the volume was created
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

// DeploymentEvent tracks events during deployment.
type DeploymentEvent struct {
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Type is the event type (info, warning, error)
	Type string `json:"type"`

	// Phase is the deployment phase when event occurred
	Phase string `json:"phase,omitempty"`

	// Container is the container name (if applicable)
	Container string `json:"container,omitempty"`

	// Message is the event message
	Message string `json:"message"`

	// Details contains additional event details
	Details map[string]interface{} `json:"details,omitempty"`
}

// RollbackState tracks rollback progress if deployment fails.
type RollbackState struct {
	// Status is the rollback status (rolling-back, rolled-back, rollback-failed)
	Status string `json:"status"`

	// StartedAt is when rollback started
	StartedAt time.Time `json:"startedAt"`

	// CompletedAt is when rollback completed
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// RemovedContainers lists containers removed during rollback
	RemovedContainers []string `json:"removedContainers,omitempty"`

	// ErrorMessage contains error details if rollback failed
	ErrorMessage string `json:"errorMessage,omitempty"`
}
