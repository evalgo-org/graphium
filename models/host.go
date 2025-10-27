package models

type Host struct {
	Context    string `json:"@context" jsonld:"@context"`
	Type       string `json:"@type" jsonld:"@type"`
	ID         string `json:"@id" jsonld:"@id" couchdb:"_id"`
	Rev        string `json:"_rev,omitempty" couchdb:"_rev"`
	Name       string `json:"name" jsonld:"name" couchdb:"required,index"`
	IPAddress  string `json:"ipAddress" jsonld:"ipAddress" couchdb:"required,index"`
	CPU        int    `json:"cpu" jsonld:"processorCount"`
	Memory     int64  `json:"memory" jsonld:"memorySize"`
	Status     string `json:"status" jsonld:"status" couchdb:"index"`
	Datacenter string `json:"location" jsonld:"location" couchdb:"index"`
}
