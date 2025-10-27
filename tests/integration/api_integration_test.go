// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"evalgo.org/graphium/internal/api"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// TestContainerCRUD tests the complete CRUD lifecycle for containers
func TestContainerCRUD(t *testing.T) {
	// Setup test server and storage
	cfg := getTestConfig()
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server := api.New(cfg, store)

	// Test CREATE
	t.Run("Create Container", func(t *testing.T) {
		container := models.Container{
			ID:       "test-container-001",
			Name:     "test-nginx",
			Image:    "nginx:latest",
			Status:   "running",
			HostedOn: "test-host",
		}

		body, _ := json.Marshal(container)
		req := httptest.NewRequest("POST", "/api/v1/containers", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var created models.Container
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if created.ID != container.ID {
			t.Errorf("Expected ID %s, got %s", container.ID, created.ID)
		}
	})

	// Test READ
	t.Run("Get Container", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/containers/test-container-001", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var container models.Container
		if err := json.Unmarshal(rec.Body.Bytes(), &container); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if container.Name != "test-nginx" {
			t.Errorf("Expected name 'test-nginx', got %s", container.Name)
		}
	})

	// Test UPDATE
	t.Run("Update Container", func(t *testing.T) {
		// First get the container to get its revision
		req := httptest.NewRequest("GET", "/api/v1/containers/test-container-001", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		var existing models.Container
		json.Unmarshal(rec.Body.Bytes(), &existing)

		// Update the container
		existing.Status = "stopped"
		body, _ := json.Marshal(existing)
		req = httptest.NewRequest("PUT", "/api/v1/containers/test-container-001", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var updated models.Container
		json.Unmarshal(rec.Body.Bytes(), &updated)

		if updated.Status != "stopped" {
			t.Errorf("Expected status 'stopped', got %s", updated.Status)
		}
	})

	// Test LIST with pagination
	t.Run("List Containers with Pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/containers?limit=10&offset=0", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response api.PaginatedContainersResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Limit != 10 {
			t.Errorf("Expected limit 10, got %d", response.Limit)
		}
	})

	// Test DELETE
	t.Run("Delete Container", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/containers/test-container-001", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify deletion
		req = httptest.NewRequest("GET", "/api/v1/containers/test-container-001", nil)
		rec = httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 after deletion, got %d", rec.Code)
		}
	})
}

// TestHostCRUD tests the complete CRUD lifecycle for hosts
func TestHostCRUD(t *testing.T) {
	cfg := getTestConfig()
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server := api.New(cfg, store)

	// Test CREATE
	t.Run("Create Host", func(t *testing.T) {
		host := models.Host{
			ID:        "test-host-001",
			Name:      "test-server",
			IPAddress: "192.168.1.100",
			Status:    "active",
		}

		body, _ := json.Marshal(host)
		req := httptest.NewRequest("POST", "/api/v1/hosts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// Test READ
	t.Run("Get Host", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/hosts/test-host-001", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var host models.Host
		json.Unmarshal(rec.Body.Bytes(), &host)

		if host.IPAddress != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100, got %s", host.IPAddress)
		}
	})

	// Cleanup
	t.Run("Delete Host", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/hosts/test-host-001", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

// TestAPIValidation tests API validation middleware
func TestAPIValidation(t *testing.T) {
	cfg := getTestConfig()
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server := api.New(cfg, store)

	t.Run("Invalid Content-Type", func(t *testing.T) {
		body := []byte(`{"name":"test"}`)
		req := httptest.NewRequest("POST", "/api/v1/containers", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Missing Required Fields", func(t *testing.T) {
		container := map[string]string{} // Empty container
		body, _ := json.Marshal(container)
		req := httptest.NewRequest("POST", "/api/v1/containers", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Invalid ID Format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/containers/ab", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Invalid Query Parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/containers?status=invalid_status", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})
}

// TestBulkOperations tests bulk create operations
func TestBulkOperations(t *testing.T) {
	cfg := getTestConfig()
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server := api.New(cfg, store)

	t.Run("Bulk Create Containers", func(t *testing.T) {
		containers := []*models.Container{
			{
				ID:    "bulk-container-001",
				Name:  "bulk-nginx-1",
				Image: "nginx:latest",
			},
			{
				ID:    "bulk-container-002",
				Name:  "bulk-nginx-2",
				Image: "nginx:latest",
			},
		}

		body, _ := json.Marshal(containers)
		req := httptest.NewRequest("POST", "/api/v1/containers/bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response api.BulkResponse
		json.Unmarshal(rec.Body.Bytes(), &response)

		if response.Success < 2 {
			t.Errorf("Expected at least 2 successful creates, got %d", response.Success)
		}
	})
}

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	cfg := getTestConfig()
	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server := api.New(cfg, store)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &response)

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
}

// getTestConfig returns a test configuration
func getTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8095,
		},
		CouchDB: config.CouchDBConfig{
			URL:      "http://localhost:5985",
			Database: "graphium_test",
			Username: "admin",
			Password: "testpass",
		},
		Security: config.SecurityConfig{
			AllowedOrigins: []string{"*"},
			RateLimit:      100,
		},
	}
}

// Helper function to add a ServeHTTP method to the Server
// Note: In actual implementation, you might need to expose the echo instance
// or create a proper HTTP handler interface
