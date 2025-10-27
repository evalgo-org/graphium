// Package storage provides the storage layer for Graphium using CouchDB.
// This package wraps the eve.evalgo.org/db library to provide Graphium-specific
// functionality for managing containers, hosts, and their relationships.
package storage

import (
	"encoding/json"
	"fmt"

	"eve.evalgo.org/db"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/models"
)

// Storage provides the main storage interface for Graphium.
// It wraps the CouchDB service from eve library and provides
// type-safe operations for Graphium entities.
type Storage struct {
	service *db.CouchDBService
	config  *config.Config
}

// New creates a new Storage instance from the application configuration.
// It initializes the CouchDB connection and ensures the database exists.
func New(cfg *config.Config) (*Storage, error) {
	// Create CouchDB configuration from app config
	couchConfig := db.CouchDBConfig{
		URL:             cfg.CouchDB.URL,
		Database:        cfg.CouchDB.Database,
		Username:        cfg.CouchDB.Username,
		Password:        cfg.CouchDB.Password,
		CreateIfMissing: true,
	}

	// Initialize CouchDB service
	service, err := db.NewCouchDBServiceFromConfig(couchConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create CouchDB service: %w", err)
	}

	storage := &Storage{
		service: service,
		config:  cfg,
	}

	// Initialize database schema (indexes and views)
	if err := storage.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return storage, nil
}

// initializeSchema creates indexes and views needed for Graphium queries.
func (s *Storage) initializeSchema() error {
	// Create indexes for common queries
	indexes := []db.Index{
		{
			Name:   "containers-status-host",
			Fields: []string{"@type", "status", "hostedOn"},
			Type:   "json",
		},
		{
			Name:   "hosts-datacenter-status",
			Fields: []string{"@type", "location", "status"},
			Type:   "json",
		},
		{
			Name:   "containers-name",
			Fields: []string{"@type", "name"},
			Type:   "json",
		},
		{
			Name:   "hosts-name",
			Fields: []string{"@type", "name"},
			Type:   "json",
		},
	}

	for _, index := range indexes {
		if err := s.service.CreateIndex(index); err != nil {
			// Log warning but don't fail - index might already exist
			fmt.Printf("Warning: failed to create index %s: %v\n", index.Name, err)
		}
	}

	// Create views for graph queries
	if err := s.createViews(); err != nil {
		return fmt.Errorf("failed to create views: %w", err)
	}

	return nil
}

// createViews creates CouchDB MapReduce views for graph traversal and queries.
func (s *Storage) createViews() error {
	designDoc := db.DesignDoc{
		ID:       "_design/graphium",
		Language: "javascript",
		Views: map[string]db.View{
			// View: containers_by_host - Find all containers on a specific host
			"containers_by_host": {
				Map: `function(doc) {
					if (doc['@type'] === 'SoftwareApplication' && doc.hostedOn) {
						emit(doc.hostedOn, doc);
					}
				}`,
			},
			// View: hosts_by_datacenter - Find all hosts in a datacenter
			"hosts_by_datacenter": {
				Map: `function(doc) {
					if (doc['@type'] === 'ComputerSystem' && doc.location) {
						emit(doc.location, doc);
					}
				}`,
			},
			// View: containers_by_status - Find containers by status
			"containers_by_status": {
				Map: `function(doc) {
					if (doc['@type'] === 'SoftwareApplication' && doc.status) {
						emit(doc.status, doc);
					}
				}`,
			},
			// View: containers_by_image - Find containers by image name
			"containers_by_image": {
				Map: `function(doc) {
					if (doc['@type'] === 'SoftwareApplication' && doc.executableName) {
						emit(doc.executableName, doc);
					}
				}`,
			},
			// View: container_count_by_host - Count containers per host
			"container_count_by_host": {
				Map: `function(doc) {
					if (doc['@type'] === 'SoftwareApplication' && doc.hostedOn) {
						emit(doc.hostedOn, 1);
					}
				}`,
				Reduce: "_sum",
			},
			// View: host_status_summary - Aggregate host statuses
			"host_status_summary": {
				Map: `function(doc) {
					if (doc['@type'] === 'ComputerServer' && doc.status) {
						emit(doc.status, 1);
					}
				}`,
				Reduce: "_sum",
			},
		},
	}

	return s.service.CreateDesignDoc(designDoc)
}

// Close closes the storage connection.
func (s *Storage) Close() error {
	return s.service.Close()
}

// SaveContainer saves a container to the database.
func (s *Storage) SaveContainer(container *models.Container) error {
	// Set JSON-LD context and type if not set
	if container.Context == "" {
		container.Context = "https://schema.org"
	}
	if container.Type == "" {
		container.Type = "SoftwareApplication"
	}

	_, err := s.service.SaveGenericDocument(container)
	return err
}

// GetContainer retrieves a container by ID.
func (s *Storage) GetContainer(id string) (*models.Container, error) {
	var container models.Container
	err := s.service.GetGenericDocument(id, &container)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// DeleteContainer deletes a container by ID and revision.
func (s *Storage) DeleteContainer(id, rev string) error {
	return s.service.DeleteDocument(id, rev)
}

// ListContainers retrieves all containers matching the given filters.
func (s *Storage) ListContainers(filters map[string]interface{}) ([]*models.Container, error) {
	// Build query with filters
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "SoftwareApplication")

	// Apply additional filters
	for field, value := range filters {
		qb = qb.And().Where(field, "$eq", value)
	}

	query := qb.Build()

	// Execute query
	containers, err := db.FindTyped[models.Container](s.service, query)
	if err != nil {
		return nil, err
	}

	// Deduplicate containers by @id (Docker container ID)
	// CouchDB may have multiple documents for the same container due to sync issues
	containerMap := make(map[string]*models.Container)
	for i := range containers {
		// Keep the latest version (last one wins)
		containerMap[containers[i].ID] = &containers[i]
	}

	// Convert map to slice
	result := make([]*models.Container, 0, len(containerMap))
	for _, container := range containerMap {
		result = append(result, container)
	}

	return result, nil
}

// GetContainersByHost retrieves all containers running on a specific host.
func (s *Storage) GetContainersByHost(hostID string) ([]*models.Container, error) {
	result, err := s.service.QueryView("graphium", "containers_by_host", db.ViewOptions{
		Key:         hostID,
		IncludeDocs: true,
	})

	if err != nil {
		return nil, err
	}

	// Deduplicate containers by @id (Docker container ID)
	// CouchDB may have multiple documents for the same container due to sync issues
	containerMap := make(map[string]*models.Container)
	for _, row := range result.Rows {
		var container models.Container
		if err := json.Unmarshal(row.Doc, &container); err != nil {
			continue // Skip invalid documents
		}
		// Keep the latest version (last one wins)
		containerMap[container.ID] = &container
	}

	// Convert map to slice
	containers := make([]*models.Container, 0, len(containerMap))
	for _, container := range containerMap {
		containers = append(containers, container)
	}

	return containers, nil
}

// GetContainersByStatus retrieves all containers with a specific status.
func (s *Storage) GetContainersByStatus(status string) ([]*models.Container, error) {
	result, err := s.service.QueryView("graphium", "containers_by_status", db.ViewOptions{
		Key:         status,
		IncludeDocs: true,
	})

	if err != nil {
		return nil, err
	}

	containers := make([]*models.Container, 0, len(result.Rows))
	for _, row := range result.Rows {
		var container models.Container
		if err := json.Unmarshal(row.Doc, &container); err != nil {
			continue
		}
		containers = append(containers, &container)
	}

	return containers, nil
}

// SaveHost saves a host to the database.
func (s *Storage) SaveHost(host *models.Host) error {
	// Set JSON-LD context and type if not set
	if host.Context == "" {
		host.Context = "https://schema.org"
	}
	if host.Type == "" {
		host.Type = "ComputerServer"
	}

	_, err := s.service.SaveGenericDocument(host)
	return err
}

// GetHost retrieves a host by ID.
func (s *Storage) GetHost(id string) (*models.Host, error) {
	var host models.Host
	err := s.service.GetGenericDocument(id, &host)
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// DeleteHost deletes a host by ID and revision.
func (s *Storage) DeleteHost(id, rev string) error {
	return s.service.DeleteDocument(id, rev)
}

// ListHosts retrieves all hosts matching the given filters.
func (s *Storage) ListHosts(filters map[string]interface{}) ([]*models.Host, error) {
	// Build query with filters - accept both ComputerServer and ComputerSystem types
	// Use direct MangoQuery since QueryBuilder may not support $in properly
	selector := map[string]interface{}{
		"@type": map[string]interface{}{
			"$in": []string{"ComputerServer", "ComputerSystem"},
		},
	}

	// Apply additional filters to the selector
	for field, value := range filters {
		selector[field] = map[string]interface{}{"$eq": value}
	}

	query := db.MangoQuery{
		Selector: selector,
	}

	// Execute query
	hosts, err := db.FindTyped[models.Host](s.service, query)
	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	result := make([]*models.Host, len(hosts))
	for i := range hosts {
		result[i] = &hosts[i]
	}

	return result, nil
}

// GetHostsByDatacenter retrieves all hosts in a specific datacenter.
func (s *Storage) GetHostsByDatacenter(datacenter string) ([]*models.Host, error) {
	result, err := s.service.QueryView("graphium", "hosts_by_datacenter", db.ViewOptions{
		Key:         datacenter,
		IncludeDocs: true,
	})

	if err != nil {
		return nil, err
	}

	hosts := make([]*models.Host, 0, len(result.Rows))
	for _, row := range result.Rows {
		var host models.Host
		if err := json.Unmarshal(row.Doc, &host); err != nil {
			continue
		}
		hosts = append(hosts, &host)
	}

	return hosts, nil
}

// BulkSaveContainers saves multiple containers in a single operation.
func (s *Storage) BulkSaveContainers(containers []*models.Container) ([]db.BulkResult, error) {
	docs := make([]interface{}, len(containers))
	for i, c := range containers {
		// Set defaults
		if c.Context == "" {
			c.Context = "https://schema.org"
		}
		if c.Type == "" {
			c.Type = "SoftwareApplication"
		}
		docs[i] = c
	}

	return s.service.BulkSaveDocuments(docs)
}

// BulkSaveHosts saves multiple hosts in a single operation.
func (s *Storage) BulkSaveHosts(hosts []*models.Host) ([]db.BulkResult, error) {
	docs := make([]interface{}, len(hosts))
	for i, h := range hosts {
		// Set defaults
		if h.Context == "" {
			h.Context = "https://schema.org"
		}
		if h.Type == "" {
			h.Type = "ComputerServer"
		}
		docs[i] = h
	}

	return s.service.BulkSaveDocuments(docs)
}

// GetContainerDependents finds all containers that reference a given container.
func (s *Storage) GetContainerDependents(containerID string) ([]*models.Container, error) {
	dependents, err := s.service.GetDependents(containerID, "dependsOn")
	if err != nil {
		return nil, err
	}

	containers := make([]*models.Container, 0, len(dependents))
	for _, depData := range dependents {
		var container models.Container
		if err := json.Unmarshal(depData, &container); err != nil {
			continue
		}
		containers = append(containers, &container)
	}

	return containers, nil
}

// GetHostContainerCount returns the number of containers on each host.
func (s *Storage) GetHostContainerCount() (map[string]int, error) {
	// Get all active containers using Find query (only returns non-deleted documents)
	containers, err := s.ListContainers(nil)
	if err != nil {
		return nil, err
	}

	// Count containers per host
	counts := make(map[string]int)
	for _, container := range containers {
		if container.HostedOn != "" {
			counts[container.HostedOn]++
		}
	}

	return counts, nil
}

// GetDatabaseInfo returns database statistics.
func (s *Storage) GetDatabaseInfo() (*db.DatabaseInfo, error) {
	return s.service.GetDatabaseInfo()
}
