package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Service represents a service registration in the registry
type ServiceRegistration struct {
	Context    string                 `json:"@context"`
	Type       string                 `json:"@type"`
	Identifier string                 `json:"identifier"`
	Name       string                 `json:"name"`
	URL        string                 `json:"url"`
	Properties map[string]interface{} `json:"additionalProperty,omitempty"`
}

// registerWithRegistry registers this agent as a semantic service
func (a *Agent) registerWithRegistry(ctx context.Context) error {
	registryURL := os.Getenv("REGISTRYSERVICE_API_URL")
	if registryURL == "" {
		registryURL = "http://localhost:8096" // Default registry service URL
	}

	// Determine agent URL
	agentURL := os.Getenv("AGENT_URL")
	if agentURL == "" {
		// Try to construct from hostname and port
		hostname := os.Getenv("HOSTNAME")
		if hostname == "" {
			hostname = a.hostID
		}
		if a.httpPort > 0 {
			agentURL = fmt.Sprintf("http://%s:%d", hostname, a.httpPort)
		} else {
			log.Printf("Warning: Agent HTTP port not configured, skipping registry registration")
			return nil
		}
	}

	registration := ServiceRegistration{
		Context:    "https://schema.org",
		Type:       "SoftwareApplication",
		Identifier: fmt.Sprintf("graphium-agent-%s", a.hostID),
		Name:       fmt.Sprintf("Graphium Agent - %s", a.hostID),
		URL:        agentURL,
		Properties: map[string]interface{}{
			"version":       "1.0.0",
			"datacenter":    a.datacenter,
			"hostId":        a.hostID,
			"capabilities": []string{
				"docker-deploy",
				"docker-control",
				"docker-delete",
				"docker-check",
			},
			"semanticEndpoint": fmt.Sprintf("%s/v1/api/semantic/action", agentURL),
			"healthEndpoint":   fmt.Sprintf("%s/health", agentURL),
		},
	}

	// Marshal registration
	payload, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}

	// Send registration to registry service
	registrationURL := fmt.Sprintf("%s/v1/api/services", registryURL)
	req, err := http.NewRequestWithContext(ctx, "POST", registrationURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	log.Printf("Successfully registered agent %s with registry at %s", a.hostID, registryURL)
	return nil
}

// startRegistryHeartbeat sends periodic heartbeats to the registry service
func (a *Agent) startRegistryHeartbeat(ctx context.Context) {
	heartbeatInterval := 30 * time.Second // Send heartbeat every 30 seconds
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.sendRegistryHeartbeat(ctx); err != nil {
				log.Printf("Failed to send registry heartbeat: %v", err)
			}

		case <-ctx.Done():
			log.Println("Registry heartbeat stopped")
			// Deregister on shutdown
			if err := a.deregisterFromRegistry(context.Background()); err != nil {
				log.Printf("Failed to deregister from registry: %v", err)
			}
			return
		}
	}
}

// sendRegistryHeartbeat sends a heartbeat to update service status
func (a *Agent) sendRegistryHeartbeat(ctx context.Context) error {
	registryURL := os.Getenv("REGISTRYSERVICE_API_URL")
	if registryURL == "" {
		registryURL = "http://localhost:8096"
	}

	serviceID := fmt.Sprintf("graphium-agent-%s", a.hostID)
	heartbeatURL := fmt.Sprintf("%s/v1/api/services/%s/heartbeat", registryURL, serviceID)

	heartbeat := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "healthy",
		"metrics": map[string]interface{}{
			"uptime":       time.Since(a.startTime).Seconds(),
			"syncCount":    a.syncCount,
			"failedSyncs":  a.failedSyncs,
			"eventsCount":  a.eventsCount,
		},
	}

	payload, err := json.Marshal(heartbeat)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", heartbeatURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If service not found (404), re-register
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Agent not found in registry, re-registering...")
		return a.registerWithRegistry(ctx)
	}

	return nil
}

// deregisterFromRegistry removes this agent from the registry
func (a *Agent) deregisterFromRegistry(ctx context.Context) error {
	registryURL := os.Getenv("REGISTRYSERVICE_API_URL")
	if registryURL == "" {
		registryURL = "http://localhost:8096"
	}

	serviceID := fmt.Sprintf("graphium-agent-%s", a.hostID)
	deregisterURL := fmt.Sprintf("%s/v1/api/services/%s", registryURL, serviceID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", deregisterURL, nil)
	if err != nil {
		return err
	}

	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("Deregistered agent %s from registry", a.hostID)
	return nil
}
