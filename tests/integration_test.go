// +build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"evalgo.org/graphium/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAPIURL    = "http://localhost:8080"
	testCouchDB   = "http://localhost:5984"
	testDatabase  = "graphium_test"
	testTimeout   = 30 * time.Second
)

// TestIntegration_FullWorkflow tests the complete workflow from container creation to querying
func TestIntegration_FullWorkflow(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: Create a host
	host := &models.Host{
		Context:    "https://schema.org",
		Type:       "ComputerSystem",
		ID:         fmt.Sprintf("test-host-%d", time.Now().Unix()),
		Name:       "test-host",
		IPAddress:  "192.168.1.100",
		CPU:        4,
		Memory:     8000000000,
		Status:     "active",
		Datacenter: "test-dc",
	}

	hostJSON, err := json.Marshal(host)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(ctx, "POST", testAPIURL+"/api/v1/hosts", bytes.NewBuffer(hostJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Failed to create host")

	// Step 2: Create containers
	container1 := &models.Container{
		Context:  "https://schema.org",
		Type:     "SoftwareApplication",
		ID:       fmt.Sprintf("test-container-1-%d", time.Now().Unix()),
		Name:     "nginx-web",
		Image:    "nginx:latest",
		Status:   "running",
		HostedOn: host.ID,
	}

	containerJSON, err := json.Marshal(container1)
	require.NoError(t, err)

	req, err = http.NewRequestWithContext(ctx, "POST", testAPIURL+"/api/v1/containers", bytes.NewBuffer(containerJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Failed to create container")

	// Step 3: Query containers
	req, err = http.NewRequestWithContext(ctx, "GET", testAPIURL+"/api/v1/containers", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 4: Get container by ID
	req, err = http.NewRequestWithContext(ctx, "GET", testAPIURL+"/api/v1/containers/"+container1.ID, nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var retrievedContainer models.Container
	err = json.NewDecoder(resp.Body).Decode(&retrievedContainer)
	require.NoError(t, err)
	assert.Equal(t, container1.Name, retrievedContainer.Name)

	// Step 5: Query containers by host
	req, err = http.NewRequestWithContext(ctx, "GET", testAPIURL+"/api/v1/query/containers/by-host/"+host.ID, nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 6: Get statistics
	req, err = http.NewRequestWithContext(ctx, "GET", testAPIURL+"/api/v1/stats", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 7: Delete container
	req, err = http.NewRequestWithContext(ctx, "DELETE", testAPIURL+"/api/v1/containers/"+container1.ID, nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 8: Delete host
	req, err = http.NewRequestWithContext(ctx, "DELETE", testAPIURL+"/api/v1/hosts/"+host.ID, nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestIntegration_Validation tests the validation endpoints
func TestIntegration_Validation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	// Test valid container validation
	validContainer := `{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test-container",
		"name": "test",
		"executableName": "nginx:latest",
		"hostedOn": "host-01"
	}`

	req, err := http.NewRequestWithContext(ctx, "POST", testAPIURL+"/api/v1/validate/container", bytes.NewBufferString(validContainer))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test invalid container validation
	invalidContainer := `{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test-container"
	}`

	req, err = http.NewRequestWithContext(ctx, "POST", testAPIURL+"/api/v1/validate/container", bytes.NewBufferString(invalidContainer))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
