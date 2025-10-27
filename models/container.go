package models

//go:generate go run ../tools/generate.go

type Container struct {
	Context  string            `json:"@context" jsonld:"@context"`
	Type     string            `json:"@type" jsonld:"@type"`
	ID       string            `json:"@id" jsonld:"@id" couchdb:"_id"`
	Rev      string            `json:"_rev,omitempty" couchdb:"_rev"`
	Name     string            `json:"name" jsonld:"name" couchdb:"required,index"`
	Image    string            `json:"executableName" jsonld:"executableName" couchdb:"required"`
	Status   string            `json:"status" jsonld:"status" couchdb:"index"`
	HostedOn string            `json:"hostedOn" jsonld:"hostedOn" couchdb:"relation,index"`
	Ports    []Port            `json:"ports,omitempty" jsonld:"ports"`
	Env      map[string]string `json:"environment,omitempty" jsonld:"environment"`
	Created  string            `json:"dateCreated,omitempty" jsonld:"dateCreated"`
}

type Port struct {
	HostPort      int    `json:"hostPort" jsonld:"hostPort"`
	ContainerPort int    `json:"containerPort" jsonld:"containerPort"`
	Protocol      string `json:"protocol" jsonld:"protocol"`
}
