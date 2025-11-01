package agent

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"evalgo.org/graphium/models"
	evecommon "eve.evalgo.org/common"
)

// AgentDeployer handles container lifecycle operations for tasks.
// It provides methods to deploy, delete, start, stop, and manage containers
// on the local Docker daemon.
type AgentDeployer struct {
	docker  *dockerclient.Client
	hostID  string
	agentID string
}

// NewDeployer creates a new agent deployer instance.
func NewDeployer(docker *dockerclient.Client, hostID string, agentID string) *AgentDeployer {
	return &AgentDeployer{
		docker:  docker,
		hostID:  hostID,
		agentID: agentID,
	}
}

// DeployContainer creates and starts a container from the specification.
func (d *AgentDeployer) DeployContainer(ctx context.Context, payload *models.DeployContainerPayload) (*models.TaskResult, error) {
	spec := &payload.ContainerSpec

	// Pull image if needed
	pullPolicy := payload.PullPolicy
	if pullPolicy == "" {
		pullPolicy = "if-not-present"
	}

	if err := d.pullImage(ctx, spec.Image, pullPolicy); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Convert container spec to Docker config
	containerConfig, hostConfig, networkConfig, err := d.specToDockerConfig(spec, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert spec to Docker config: %w", err)
	}

	// Create container
	containerName := spec.Name
	if containerName == "" {
		containerName = fmt.Sprintf("graphium-%s-%d", spec.Image, time.Now().Unix())
	}

	resp, err := d.docker.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID

	// Start container
	if err := d.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		// Clean up created container if start fails
		_ = d.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Return success result
	result := &models.TaskResult{
		Success:     true,
		ContainerID: containerID,
		Message:     fmt.Sprintf("Container %s deployed successfully", containerName),
		Data: map[string]interface{}{
			"containerName": containerName,
			"image":         spec.Image,
			"warnings":      resp.Warnings,
		},
	}

	return result, nil
}

// DeleteContainer removes a container.
func (d *AgentDeployer) DeleteContainer(ctx context.Context, payload *models.DeleteContainerPayload) (*models.TaskResult, error) {
	containerID := payload.ContainerID
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	// Stop container if running (unless force is true)
	if !payload.Force {
		timeout := payload.StopTimeout
		if timeout == 0 {
			timeout = 10
		}

		if err := evecommon.ContainerStop(ctx, d.docker, containerID, timeout); err != nil {
			// Ignore error if container is already stopped
			if !strings.Contains(err.Error(), "is already stopped") &&
				!strings.Contains(err.Error(), "No such container") {
				return nil, fmt.Errorf("failed to stop container: %w", err)
			}
		}
	}

	// Remove container
	removeOptions := container.RemoveOptions{
		Force:         payload.Force,
		RemoveVolumes: payload.RemoveVolumes,
	}

	if err := d.docker.ContainerRemove(ctx, containerID, removeOptions); err != nil {
		return nil, fmt.Errorf("failed to remove container: %w", err)
	}

	// Return success result
	result := &models.TaskResult{
		Success:     true,
		ContainerID: containerID,
		Message:     fmt.Sprintf("Container %s deleted successfully", payload.ContainerName),
	}

	return result, nil
}

// StopContainer stops a running container.
func (d *AgentDeployer) StopContainer(ctx context.Context, payload *models.ControlContainerPayload) (*models.TaskResult, error) {
	containerID := payload.ContainerID
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	timeout := payload.Timeout
	if timeout == 0 {
		timeout = 10
	}

	if err := evecommon.ContainerStop(ctx, d.docker, containerID, timeout); err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	result := &models.TaskResult{
		Success:     true,
		ContainerID: containerID,
		Message:     fmt.Sprintf("Container %s stopped successfully", payload.ContainerName),
	}

	return result, nil
}

// StartContainer starts a stopped container.
func (d *AgentDeployer) StartContainer(ctx context.Context, payload *models.ControlContainerPayload) (*models.TaskResult, error) {
	containerID := payload.ContainerID
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	if err := d.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	result := &models.TaskResult{
		Success:     true,
		ContainerID: containerID,
		Message:     fmt.Sprintf("Container %s started successfully", payload.ContainerName),
	}

	return result, nil
}

// RestartContainer restarts a container.
func (d *AgentDeployer) RestartContainer(ctx context.Context, payload *models.ControlContainerPayload) (*models.TaskResult, error) {
	containerID := payload.ContainerID
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	timeout := payload.Timeout
	if timeout == 0 {
		timeout = 10
	}

	// Stop first
	if err := evecommon.ContainerStop(ctx, d.docker, containerID, timeout); err != nil {
		// Ignore if already stopped
		if !strings.Contains(err.Error(), "is already stopped") {
			return nil, fmt.Errorf("failed to stop container: %w", err)
		}
	}

	// Then start
	if err := d.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	result := &models.TaskResult{
		Success:     true,
		ContainerID: containerID,
		Message:     fmt.Sprintf("Container %s restarted successfully", payload.ContainerName),
	}

	return result, nil
}

// pullImage pulls a Docker image if needed based on pull policy.
func (d *AgentDeployer) pullImage(ctx context.Context, imageName string, pullPolicy string) error {
	switch pullPolicy {
	case "never":
		// Don't pull, assume image exists locally
		return nil

	case "if-not-present":
		// Check if image exists locally
		_, _, err := d.docker.ImageInspectWithRaw(ctx, imageName)
		if err == nil {
			// Image exists, no need to pull
			return nil
		}
		// Image doesn't exist, pull it
		fallthrough

	case "always":
		// Always pull the image
		reader, err := d.docker.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
		defer reader.Close()

		// Consume pull output to ensure pull completes
		_, err = io.Copy(io.Discard, reader)
		return err

	default:
		return fmt.Errorf("invalid pull policy: %s", pullPolicy)
	}
}

// specToDockerConfig converts a Graphium ContainerSpec to Docker API configs.
func (d *AgentDeployer) specToDockerConfig(spec *models.ContainerSpec, payload *models.DeployContainerPayload) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
	// Container config
	containerConfig := &container.Config{
		Image:      spec.Image,
		Env:        d.convertEnvVars(spec.Environment),
		Cmd:        spec.Command,
		WorkingDir: spec.WorkingDir,
		User:       spec.User,
		Labels:     payload.Labels,
	}

	// Exposed ports
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)

	for _, port := range spec.Ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = "tcp"
		}

		natPort, err := nat.NewPort(strings.ToLower(protocol), strconv.Itoa(port.ContainerPort))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("invalid port: %w", err)
		}

		exposedPorts[natPort] = struct{}{}

		if port.HostPort > 0 {
			hostIP := port.HostIP
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			portBindings[natPort] = []nat.PortBinding{
				{
					HostIP:   hostIP,
					HostPort: strconv.Itoa(port.HostPort),
				},
			}
		}
	}

	containerConfig.ExposedPorts = exposedPorts

	// Host config
	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		RestartPolicy: d.convertRestartPolicy(spec.RestartPolicy),
		Binds:         d.convertVolumeMounts(spec.VolumeMounts),
	}

	// Network config
	networkConfig := &network.NetworkingConfig{}

	return containerConfig, hostConfig, networkConfig, nil
}

// convertEnvVars converts Graphium environment variables to Docker format.
func (d *AgentDeployer) convertEnvVars(envVars []models.EnvironmentVariable) []string {
	result := make([]string, 0, len(envVars))
	for _, env := range envVars {
		result = append(result, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}
	return result
}

// convertRestartPolicy converts Graphium restart policy to Docker format.
func (d *AgentDeployer) convertRestartPolicy(policy string) container.RestartPolicy {
	switch policy {
	case "always":
		return container.RestartPolicy{Name: "always"}
	case "unless-stopped":
		return container.RestartPolicy{Name: "unless-stopped"}
	case "on-failure":
		return container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3}
	case "no", "":
		return container.RestartPolicy{Name: "no"}
	default:
		return container.RestartPolicy{Name: "no"}
	}
}

// convertVolumeMounts converts Graphium volume mounts to Docker format.
func (d *AgentDeployer) convertVolumeMounts(mounts []models.VolumeMount) []string {
	result := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		bindStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
		if mount.ReadOnly {
			bindStr += ":ro"
		}
		result = append(result, bindStr)
	}
	return result
}
