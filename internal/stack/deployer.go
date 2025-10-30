package stack

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/go-connections/nat"

	"evalgo.org/eve/common"
	"evalgo.org/graphium/models"
)

// Database is an interface for storing deployment state.
type Database interface {
	// Create creates a new document
	Create(ctx context.Context, doc interface{}) error

	// Update updates an existing document
	Update(ctx context.Context, doc interface{}) error
}

// Deployer handles the deployment of stacks to Docker hosts.
type Deployer struct {
	// DB is the database interface for storing deployment state
	DB Database

	// HostResolver resolves host references
	HostResolver HostResolver

	// DockerClientFactory creates Docker clients for hosts
	DockerClientFactory DockerClientFactory
}

// DockerClientFactory creates Docker clients for different hosts.
type DockerClientFactory interface {
	// GetClient returns a Docker client for the given host ID
	GetClient(ctx context.Context, hostID string) (common.DockerClient, error)
}

// DeployOptions contains options for deployment.
type DeployOptions struct {
	// Timeout for operations (default: 5 minutes)
	Timeout time.Duration

	// RollbackOnError automatically rolls back on any error
	RollbackOnError bool

	// StackName is the name of the stack (will prefix all containers)
	StackName string

	// PullImages pulls images before deployment
	PullImages bool
}

// NewDeployer creates a new deployer.
func NewDeployer(database Database, resolver HostResolver, clientFactory DockerClientFactory) *Deployer {
	return &Deployer{
		DB:                  database,
		HostResolver:        resolver,
		DockerClientFactory: clientFactory,
	}
}

// Deploy deploys a stack according to the deployment plan.
func (d *Deployer) Deploy(ctx context.Context, plan *models.DeploymentPlan, opts DeployOptions) (*models.DeploymentState, error) {
	// Set default timeout
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	// Create deployment context with timeout
	deployCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Initialize deployment state
	state := &models.DeploymentState{
		ID:         fmt.Sprintf("deployment-%s-%d", opts.StackName, time.Now().Unix()),
		Type:       "DeploymentState",
		StackID:    opts.StackName,
		Status:     "deploying",
		Phase:      "initialization",
		Progress:   0,
		Placements: make(map[string]*models.ContainerPlacement),
		Events:     []models.DeploymentEvent{},
		StartedAt:  time.Now(),
	}

	// Add initialization event
	d.addEvent(state, "info", "initialization", "", "Starting deployment")

	// Save initial state
	if err := d.DB.Create(deployCtx, state); err != nil {
		return state, fmt.Errorf("failed to save deployment state: %w", err)
	}

	// Defer rollback on error if requested
	if opts.RollbackOnError {
		defer func() {
			if state.Status == "failed" {
				d.rollback(ctx, state)
			}
		}()
	}

	// Step 1: Create network if needed
	if err := d.deployNetwork(deployCtx, plan, state, opts); err != nil {
		return d.failDeployment(deployCtx, state, "network creation failed", err)
	}

	// Step 2: Create volumes if needed
	if err := d.deployVolumes(deployCtx, plan, state, opts); err != nil {
		return d.failDeployment(deployCtx, state, "volume creation failed", err)
	}

	// Step 3: Deploy containers in waves
	if err := d.deployContainersInWaves(deployCtx, plan, state, opts); err != nil {
		return d.failDeployment(deployCtx, state, "container deployment failed", err)
	}

	// Mark deployment as complete
	now := time.Now()
	state.Status = "running"
	state.Phase = "completed"
	state.Progress = 100
	state.CompletedAt = &now

	d.addEvent(state, "info", "completed", "", "Deployment completed successfully")

	// Save final state
	if err := d.DB.Update(deployCtx, state); err != nil {
		return state, fmt.Errorf("failed to save final state: %w", err)
	}

	return state, nil
}

// deployNetwork creates the Docker network if specified.
func (d *Deployer) deployNetwork(ctx context.Context, plan *models.DeploymentPlan, state *models.DeploymentState, opts DeployOptions) error {
	if plan.Network == nil {
		return nil
	}

	state.Phase = "network-creation"
	d.addEvent(state, "info", "network-creation", "", fmt.Sprintf("Creating network %s", plan.Network.Name))

	// For single-host, create on the target host
	// For multi-host, this would create overlay network (Phase 2)
	hostID := d.getPrimaryHost(plan)
	if hostID == "" {
		return fmt.Errorf("no target host for network creation")
	}

	client, err := d.DockerClientFactory.GetClient(ctx, hostID)
	if err != nil {
		return fmt.Errorf("failed to get Docker client for host %s: %w", hostID, err)
	}

	// Check if network already exists (if external)
	if plan.Network.External {
		networkInfo, err := client.NetworkInspect(ctx, plan.Network.Name, network.InspectOptions{})
		if err != nil {
			return fmt.Errorf("external network %s not found: %w", plan.Network.Name, err)
		}
		// Extract IPAM config for subnet/gateway if available
		var subnet, gateway string
		if len(networkInfo.IPAM.Config) > 0 {
			subnet = networkInfo.IPAM.Config[0].Subnet
			gateway = networkInfo.IPAM.Config[0].Gateway
		}
		state.NetworkInfo = &models.DeployedNetworkInfo{
			NetworkID:   networkInfo.ID,
			NetworkName: networkInfo.Name,
			Driver:      networkInfo.Driver,
			Subnet:      subnet,
			Gateway:     gateway,
			Scope:       networkInfo.Scope,
		}
		return nil
	}

	// Create the network
	createOpts := network.CreateOptions{
		Driver: plan.Network.Driver,
		Labels: plan.Network.Labels,
	}

	// Set IPAM config if subnet/gateway specified
	if plan.Network.Subnet != "" || plan.Network.Gateway != "" {
		createOpts.IPAM = &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet:  plan.Network.Subnet,
					Gateway: plan.Network.Gateway,
					IPRange: plan.Network.IPRange,
				},
			},
		}
	}

	// Set driver options
	if len(plan.Network.Options) > 0 {
		createOpts.Options = plan.Network.Options
	}

	resp, err := client.NetworkCreate(ctx, plan.Network.Name, createOpts)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Get network info
	networkInfo, err := client.NetworkInspect(ctx, resp.ID, network.InspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to inspect created network: %w", err)
	}

	// Extract IPAM config
	var subnet, gateway string
	if len(networkInfo.IPAM.Config) > 0 {
		subnet = networkInfo.IPAM.Config[0].Subnet
		gateway = networkInfo.IPAM.Config[0].Gateway
	}

	state.NetworkInfo = &models.DeployedNetworkInfo{
		NetworkID:   networkInfo.ID,
		NetworkName: networkInfo.Name,
		Driver:      networkInfo.Driver,
		Subnet:      subnet,
		Gateway:     gateway,
		Scope:       networkInfo.Scope,
	}

	d.addEvent(state, "info", "network-creation", "",
		fmt.Sprintf("Network %s created with ID %s", plan.Network.Name, resp.ID))

	return d.DB.Update(ctx, state)
}

// deployVolumes creates any named volumes needed.
func (d *Deployer) deployVolumes(ctx context.Context, plan *models.DeploymentPlan, state *models.DeploymentState, opts DeployOptions) error {
	state.Phase = "volume-creation"
	state.VolumeInfo = make(map[string]*models.VolumeInfo)

	// Collect all named volumes
	volumes := make(map[string]*models.VolumeMount)
	for _, spec := range plan.ContainerSpecs {
		for _, vol := range spec.VolumeMounts {
			if vol.Type == "volume" && vol.Source != "" {
				volumes[vol.Source] = &vol
			}
		}
	}

	if len(volumes) == 0 {
		return nil
	}

	d.addEvent(state, "info", "volume-creation", "",
		fmt.Sprintf("Creating %d volume(s)", len(volumes)))

	// Create volumes on their target hosts
	for volName, vol := range volumes {
		// Determine which host needs this volume
		hostID := d.getVolumeHost(plan, volName)
		if hostID == "" {
			d.addEvent(state, "warning", "volume-creation", "",
				fmt.Sprintf("No host specified for volume %s, skipping", volName))
			continue
		}

		client, err := d.DockerClientFactory.GetClient(ctx, hostID)
		if err != nil {
			return fmt.Errorf("failed to get Docker client for host %s: %w", hostID, err)
		}

		// Determine driver and labels
		driver := "local"
		var labels map[string]string
		var driverOpts map[string]string

		if vol.VolumeOptions != nil {
			labels = vol.VolumeOptions.Labels
			if vol.VolumeOptions.DriverConfig != nil {
				if vol.VolumeOptions.DriverConfig.Name != "" {
					driver = vol.VolumeOptions.DriverConfig.Name
				}
				driverOpts = vol.VolumeOptions.DriverConfig.Options
			}
		}

		// Create volume
		volResp, err := client.VolumeCreate(ctx, volume.CreateOptions{
			Name:       volName,
			Driver:     driver,
			DriverOpts: driverOpts,
			Labels:     labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create volume %s: %w", volName, err)
		}

		now := time.Now()
		state.VolumeInfo[volName] = &models.VolumeInfo{
			VolumeName: volName,
			Driver:     volResp.Driver,
			Scope:      volResp.Scope,
			CreatedAt:  &now,
		}

		d.addEvent(state, "info", "volume-creation", "",
			fmt.Sprintf("Volume %s created with name %s", volName, volResp.Name))
	}

	return d.DB.Update(ctx, state)
}

// deployContainersInWaves deploys containers in dependency-ordered waves.
func (d *Deployer) deployContainersInWaves(ctx context.Context, plan *models.DeploymentPlan, state *models.DeploymentState, opts DeployOptions) error {
	state.Phase = "container-deployment"

	// Get containers organized by wave
	waves := d.getContainerWaves(plan)
	totalContainers := len(plan.ContainerSpecs)
	deployed := 0

	for waveNum, wave := range waves {
		d.addEvent(state, "info", "container-deployment", "",
			fmt.Sprintf("Starting deployment wave %d/%d with %d container(s)",
				waveNum+1, len(waves), len(wave)))

		// Deploy all containers in this wave in parallel
		for _, spec := range wave {
			if err := d.deployContainer(ctx, plan, &spec, state, opts); err != nil {
				return fmt.Errorf("failed to deploy container %s: %w", spec.Name, err)
			}
			deployed++
			state.Progress = (deployed * 100) / totalContainers
			// d.DB.Update(ctx, state) // Commented out to avoid CouchDB revision conflicts
		}

		// Wait for wave to be healthy before proceeding
		if err := d.waitForWaveHealth(ctx, wave, state, opts); err != nil {
			return fmt.Errorf("wave %d failed health check: %w", waveNum+1, err)
		}
	}

	return nil
}

// deployContainer deploys a single container.
func (d *Deployer) deployContainer(ctx context.Context, plan *models.DeploymentPlan, spec *models.ContainerSpec, state *models.DeploymentState, opts DeployOptions) error {
	containerName := fmt.Sprintf("%s-%s", opts.StackName, spec.Name)

	d.addEvent(state, "info", "container-deployment", containerName,
		fmt.Sprintf("Deploying container %s with image %s", containerName, spec.Image))

	// Get target host
	hostID := plan.HostMap[spec.ID]
	if hostID == "" {
		// No host assigned, automatically select one
		hosts, err := d.HostResolver.ListHosts()
		if err != nil {
			return fmt.Errorf("failed to list hosts for automatic placement: %w", err)
		}
		if len(hosts) == 0 {
			return fmt.Errorf("no hosts available for container %s", spec.Name)
		}
		// Use the first available host (TODO: implement smarter placement strategy)
		hostID = hosts[0].Host.ID
		d.addEvent(state, "info", "container-deployment", containerName,
			fmt.Sprintf("Auto-selected host %s for container %s", hostID, spec.Name))
	}

	// Get Docker client
	client, err := d.DockerClientFactory.GetClient(ctx, hostID)
	if err != nil {
		return fmt.Errorf("failed to get Docker client: %w", err)
	}

	// Build container configuration
	containerConfig := d.buildContainerConfig(spec)
	hostConfig := d.buildHostConfig(spec)
	networkConfig := d.buildNetworkConfig(plan, spec)

	// Create container (platform nil for default)
	resp, err := client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Get container info
	info, err := client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get host IP
	hostInfo, err := d.HostResolver.ResolveHost(hostID)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}

	// Extract port mappings from inspect response
	ports := make(map[int]int)
	if info.NetworkSettings != nil {
		for port, bindings := range info.NetworkSettings.Ports {
			if len(bindings) > 0 {
				containerPort := port.Int()
				var hostPort int
				fmt.Sscanf(bindings[0].HostPort, "%d", &hostPort)
				ports[containerPort] = hostPort
			}
		}
	}

	// Store placement
	now := time.Now()
	state.Placements[containerName] = &models.ContainerPlacement{
		ContainerID:   info.ID,
		ContainerName: containerName,
		HostID:        hostID,
		IPAddress:     hostInfo.Host.IPAddress,
		Ports:         ports,
		Status:        info.State.Status,
		StartedAt:     &now,
	}

	d.addEvent(state, "info", "container-deployment", containerName,
		fmt.Sprintf("Container deployed successfully with ID %s", resp.ID))

	return nil
}

// buildContainerConfig builds the Docker container.Config from ContainerSpec.
func (d *Deployer) buildContainerConfig(spec *models.ContainerSpec) *container.Config {
	config := &container.Config{
		Image: spec.Image,
		Env:   []string{},
		Labels: spec.Labels,
	}

	// Environment variables
	for k, v := range spec.Environment {
		config.Env = append(config.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Command and args
	if len(spec.Command) > 0 {
		config.Cmd = spec.Command
	}
	if len(spec.Args) > 0 {
		config.Cmd = append(config.Cmd, spec.Args...)
	}

	// Working directory
	if spec.WorkingDir != "" {
		config.WorkingDir = spec.WorkingDir
	}

	// User
	if spec.User != "" {
		config.User = spec.User
	}

	// Exposed ports
	if len(spec.Ports) > 0 {
		config.ExposedPorts = make(nat.PortSet)
		for _, p := range spec.Ports {
			port := nat.Port(fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
			config.ExposedPorts[port] = struct{}{}
		}
	}

	return config
}

// buildHostConfig builds the Docker container.HostConfig from ContainerSpec.
func (d *Deployer) buildHostConfig(spec *models.ContainerSpec) *container.HostConfig {
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{},
		PortBindings:  make(nat.PortMap),
		Mounts:        []mount.Mount{},
	}

	// Restart policy
	if spec.RestartPolicy != "" {
		hostConfig.RestartPolicy.Name = container.RestartPolicyMode(spec.RestartPolicy)
	}

	// Port bindings
	for _, p := range spec.Ports {
		port := nat.Port(fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
		binding := nat.PortBinding{
			HostPort: fmt.Sprintf("%d", p.HostPort),
		}
		if p.HostIP != "" {
			binding.HostIP = p.HostIP
		}
		hostConfig.PortBindings[port] = []nat.PortBinding{binding}
	}

	// Volume mounts
	for _, v := range spec.VolumeMounts {
		mnt := mount.Mount{
			Type:     mount.Type(v.Type),
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		}

		// Bind options
		if v.BindOptions != nil {
			mnt.BindOptions = &mount.BindOptions{
				Propagation:  mount.Propagation(v.BindOptions.Propagation),
				NonRecursive: v.BindOptions.NonRecursive,
			}
		}

		hostConfig.Mounts = append(hostConfig.Mounts, mnt)
	}

	// Resource constraints
	if spec.Resources != nil {
		if spec.Resources.Limits != nil {
			if spec.Resources.Limits.CPUs > 0 {
				hostConfig.NanoCPUs = int64(spec.Resources.Limits.CPUs * 1e9)
			}
			if spec.Resources.Limits.Memory > 0 {
				hostConfig.Memory = spec.Resources.Limits.Memory
			}
			if spec.Resources.Limits.MemorySwap > 0 {
				hostConfig.MemorySwap = spec.Resources.Limits.MemorySwap
			}
			if spec.Resources.Limits.Pids > 0 {
				pidsLimit := spec.Resources.Limits.Pids
				hostConfig.PidsLimit = &pidsLimit
			}
		}
		// Note: Reservations are not directly supported in standalone Docker
		// They would be used in Docker Swarm mode
	}

	return hostConfig
}

// buildNetworkConfig builds the Docker network.NetworkingConfig.
func (d *Deployer) buildNetworkConfig(plan *models.DeploymentPlan, spec *models.ContainerSpec) *network.NetworkingConfig {
	if plan.Network == nil {
		return nil
	}

	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			plan.Network.Name: {},
		},
	}
}

// getContainerWaves returns containers organized by deployment wave.
func (d *Deployer) getContainerWaves(plan *models.DeploymentPlan) [][]models.ContainerSpec {
	if len(plan.DependencyGraph) == 0 {
		return [][]models.ContainerSpec{plan.ContainerSpecs}
	}

	// Build name to spec map
	specMap := make(map[string]models.ContainerSpec)
	for _, spec := range plan.ContainerSpecs {
		specMap[spec.Name] = spec
	}

	// Build waves
	waves := make([][]models.ContainerSpec, 0, len(plan.DependencyGraph))
	for _, waveNames := range plan.DependencyGraph {
		wave := make([]models.ContainerSpec, 0, len(waveNames))
		for _, name := range waveNames {
			if spec, ok := specMap[name]; ok {
				wave = append(wave, spec)
			}
		}
		waves = append(waves, wave)
	}

	return waves
}

// waitForWaveHealth waits for all containers in a wave to be healthy.
func (d *Deployer) waitForWaveHealth(ctx context.Context, wave []models.ContainerSpec, state *models.DeploymentState, opts DeployOptions) error {
	// Simple wait for now - just give containers time to start
	// In a full implementation, this would check health checks
	time.Sleep(2 * time.Second)
	return nil
}

// getPrimaryHost gets the primary host for network/volume creation.
func (d *Deployer) getPrimaryHost(plan *models.DeploymentPlan) string {
	// Use stack's default host if specified
	if plan.StackNode.LocatedInHost != nil {
		return plan.StackNode.LocatedInHost.ID
	}

	// Otherwise use first container's host
	if len(plan.ContainerSpecs) > 0 {
		firstContainerID := plan.ContainerSpecs[0].ID
		return plan.HostMap[firstContainerID]
	}

	return ""
}

// getVolumeHost determines which host should have a volume.
func (d *Deployer) getVolumeHost(plan *models.DeploymentPlan, volumeName string) string {
	// Find first container that uses this volume
	for _, spec := range plan.ContainerSpecs {
		for _, vol := range spec.VolumeMounts {
			if vol.Type == "volume" && vol.Source == volumeName {
				return plan.HostMap[spec.ID]
			}
		}
	}
	return ""
}

// addEvent adds an event to the deployment state.
func (d *Deployer) addEvent(state *models.DeploymentState, eventType, phase, container, message string) {
	state.Events = append(state.Events, models.DeploymentEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		Phase:     phase,
		Container: container,
		Message:   message,
	})
}

// failDeployment marks a deployment as failed.
func (d *Deployer) failDeployment(ctx context.Context, state *models.DeploymentState, phase string, err error) (*models.DeploymentState, error) {
	now := time.Now()
	state.Status = "failed"
	state.Phase = phase
	state.CompletedAt = &now
	state.ErrorMessage = err.Error()

	d.addEvent(state, "error", phase, "", err.Error())
	d.DB.Update(ctx, state)

	return state, err
}

// rollback rolls back a failed deployment.
func (d *Deployer) rollback(ctx context.Context, state *models.DeploymentState) {
	now := time.Now()
	state.RollbackState = &models.RollbackState{
		Status:            "rolling-back",
		StartedAt:         now,
		RemovedContainers: []string{},
	}

	d.addEvent(state, "info", "rollback", "", "Starting rollback")

	// Remove all deployed containers
	for name, placement := range state.Placements {
		client, err := d.DockerClientFactory.GetClient(ctx, placement.HostID)
		if err != nil {
			d.addEvent(state, "error", "rollback", name,
				fmt.Sprintf("Failed to get client for rollback: %v", err))
			continue
		}

		if err := client.ContainerRemove(ctx, placement.ContainerID, container.RemoveOptions{Force: true}); err != nil {
			d.addEvent(state, "error", "rollback", name,
				fmt.Sprintf("Failed to remove container: %v", err))
		} else {
			state.RollbackState.RemovedContainers = append(state.RollbackState.RemovedContainers, name)
			d.addEvent(state, "info", "rollback", name, "Container removed")
		}
	}

	completedAt := time.Now()
	state.RollbackState.Status = "rolled-back"
	state.RollbackState.CompletedAt = &completedAt

	d.addEvent(state, "info", "rollback", "", "Rollback completed")
	d.DB.Update(ctx, state)
}
