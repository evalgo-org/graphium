package models

// Host represents a physical or virtual machine that runs containers.
// It follows the Schema.org ComputerSystem type with infrastructure-specific fields.
//
// JSON-LD Context: https://schema.org
// Type: ComputerSystem
//
// The Host model includes:
//   - Basic identification (@id, name)
//   - Network configuration (ipAddress)
//   - Hardware specifications (cpu, memory)
//   - Status and location (status, datacenter)
//
// Hosts form the foundation of the container infrastructure graph. Each host
// can run multiple containers (linked via the Container.HostedOn field).
//
// Example JSON representation:
//
//	{
//	  "@context": "https://schema.org",
//	  "@type": "ComputerSystem",
//	  "@id": "host-01",
//	  "name": "web-server-01",
//	  "ipAddress": "192.168.1.10",
//	  "processorCount": 8,
//	  "memorySize": 16777216,
//	  "status": "active",
//	  "location": "us-west-2"
//	}
type Host struct {
	// Context is the JSON-LD @context URL (typically https://schema.org)
	Context string `json:"@context" jsonld:"@context"`

	// Type is the JSON-LD @type (ComputerSystem for hosts)
	Type string `json:"@type" jsonld:"@type"`

	// ID is the unique host identifier (maps to CouchDB _id)
	ID string `json:"@id" jsonld:"@id" couchdb:"_id"`

	// Rev is the CouchDB document revision for optimistic locking
	Rev string `json:"_rev,omitempty" couchdb:"_rev"`

	// Name is the human-readable host name (required, indexed)
	Name string `json:"name" jsonld:"name" couchdb:"required,index"`

	// IPAddress is the host's IP address (required, indexed)
	IPAddress string `json:"ipAddress" jsonld:"ipAddress" couchdb:"required,index"`

	// CPU is the number of CPU cores available
	CPU int `json:"cpu" jsonld:"processorCount"`

	// Memory is the total memory in bytes
	Memory int64 `json:"memory" jsonld:"memorySize"`

	// Status is the host operational status (active, maintenance, offline)
	Status string `json:"status" jsonld:"status" couchdb:"index"`

	// Datacenter is the physical or logical location of the host
	Datacenter string `json:"location" jsonld:"location" couchdb:"index"`
}
