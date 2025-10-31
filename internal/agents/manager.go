package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/models"
)

// Manager manages agent processes for the Graphium server.
// It starts, stops, and monitors agent processes that connect to remote Docker hosts.
type Manager struct {
	storage   *storage.Storage
	config    *config.Config
	agents    map[string]*AgentProcess // agentID -> process
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	executable string // Path to graphium executable
}

// AgentProcess represents a running agent process.
type AgentProcess struct {
	Config    *models.AgentConfig
	State     *models.AgentState
	Cmd       *exec.Cmd
	StartedAt time.Time
	LogFile   *os.File // Log file for this agent's output
	mu        sync.Mutex
}

// NewManager creates a new agent manager.
func NewManager(storage *storage.Storage, cfg *config.Config) (*Manager, error) {
	// Get path to current executable
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		storage:    storage,
		config:     cfg,
		agents:     make(map[string]*AgentProcess),
		ctx:        ctx,
		cancel:     cancel,
		executable: executable,
	}, nil
}

// Start starts the agent manager and auto-starts enabled agents.
func (m *Manager) Start() error {
	log.Println("Starting agent manager...")

	// Load all agent configurations
	configs, err := m.storage.ListAgentConfigs(nil)
	if err != nil {
		return fmt.Errorf("failed to load agent configs: %w", err)
	}

	// Auto-start enabled agents
	for _, cfg := range configs {
		if cfg.Enabled && cfg.AutoStart {
			log.Printf("Auto-starting agent: %s (%s)", cfg.Name, cfg.HostID)
			if err := m.StartAgent(cfg.ID); err != nil {
				log.Printf("Warning: Failed to auto-start agent %s: %v", cfg.ID, err)
			}
		}
	}

	// Start monitoring goroutine
	go m.monitorAgents()

	log.Printf("Agent manager started with %d agents", len(m.agents))
	return nil
}

// Stop stops all agents and shuts down the manager.
func (m *Manager) Stop() error {
	log.Println("Stopping agent manager...")

	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all running agents
	for id, agent := range m.agents {
		log.Printf("Stopping agent: %s", id)
		if err := m.stopAgentProcess(agent); err != nil {
			log.Printf("Warning: Failed to stop agent %s: %v", id, err)
		}
	}

	log.Println("Agent manager stopped")
	return nil
}

// StartAgent starts an agent by its config ID.
func (m *Manager) StartAgent(configID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if agent, exists := m.agents[configID]; exists {
		if agent.State.Status == "running" {
			return fmt.Errorf("agent already running")
		}
	}

	// Load config
	cfg, err := m.storage.GetAgentConfig(configID)
	if err != nil {
		return fmt.Errorf("failed to load agent config: %w", err)
	}

	if !cfg.Enabled {
		return fmt.Errorf("agent is disabled")
	}

	// Generate agent token
	agentToken, err := auth.GenerateAgentToken(
		m.config.Security.AgentTokenSecret,
		cfg.HostID,
		365*24*time.Hour, // 1 year token for managed agents
	)
	if err != nil {
		return fmt.Errorf("failed to generate agent token: %w", err)
	}

	// Build command
	cmd := exec.CommandContext(m.ctx, m.executable, "agent",
		"--api-url", fmt.Sprintf("http://localhost:%d", m.config.Server.Port),
		"--host-id", cfg.HostID,
		"--datacenter", cfg.Datacenter,
		"--docker-socket", cfg.DockerSocket,
	)

	// Set environment
	cmd.Env = append(os.Environ(), "TOKEN="+agentToken)

	// If SSH key path is configured, set it for Docker SSH connections
	if cfg.SSHKeyPath != "" {
		cmd.Env = append(cmd.Env, "DOCKER_SSH_IDENTITY="+cfg.SSHKeyPath)
		log.Printf("Agent %s: Using SSH key: %s", cfg.Name, cfg.SSHKeyPath)
	}

	// Create logs directory if it doesn't exist
	logsPath := m.config.Agents.LogsPath
	if logsPath == "" {
		logsPath = "./logs"
	}
	if err := os.MkdirAll(logsPath, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Open log file for this agent
	logFilePath := filepath.Join(logsPath, fmt.Sprintf("%s.log", cfg.HostID))
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Redirect output to agent-specific log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	log.Printf("Agent %s: Logging to %s", cfg.Name, logFilePath)

	// Start the process
	if err := cmd.Start(); err != nil {
		logFile.Close() // Clean up log file if process fails to start
		return fmt.Errorf("failed to start agent process: %w", err)
	}

	// Create agent process tracking
	now := time.Now()
	agent := &AgentProcess{
		Config:    cfg,
		Cmd:       cmd,
		StartedAt: now,
		LogFile:   logFile,
		State: &models.AgentState{
			ConfigID:  cfg.ID,
			Status:    "running",
			StartedAt: &now,
			ProcessID: cmd.Process.Pid,
		},
	}

	m.agents[configID] = agent

	// Monitor process in background
	go m.watchProcess(agent)

	log.Printf("Started agent: %s (PID: %d)", cfg.Name, cmd.Process.Pid)
	return nil
}

// StopAgent stops a running agent by its config ID.
func (m *Manager) StopAgent(configID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[configID]
	if !exists {
		return fmt.Errorf("agent not running")
	}

	return m.stopAgentProcess(agent)
}

// stopAgentProcess stops an agent process (caller must hold lock).
func (m *Manager) stopAgentProcess(agent *AgentProcess) error {
	agent.mu.Lock()
	defer agent.mu.Unlock()

	if agent.State.Status == "stopped" {
		return nil
	}

	agent.State.Status = "stopping"

	if agent.Cmd != nil && agent.Cmd.Process != nil {
		// Send interrupt signal
		if err := agent.Cmd.Process.Signal(os.Interrupt); err != nil {
			// Force kill if interrupt fails
			if err := agent.Cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		}

		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- agent.Cmd.Wait()
		}()

		select {
		case <-time.After(10 * time.Second):
			// Force kill after timeout
			_ = agent.Cmd.Process.Kill()
		case <-done:
			// Process exited
		}
	}

	// Close log file
	if agent.LogFile != nil {
		if err := agent.LogFile.Close(); err != nil {
			log.Printf("Warning: Failed to close log file for agent %s: %v", agent.Config.Name, err)
		}
		agent.LogFile = nil
	}

	now := time.Now()
	agent.State.Status = "stopped"
	agent.State.StoppedAt = &now

	log.Printf("Stopped agent: %s", agent.Config.Name)
	return nil
}

// RestartAgent restarts an agent by its config ID.
func (m *Manager) RestartAgent(configID string) error {
	if err := m.StopAgent(configID); err != nil && err.Error() != "agent not running" {
		return err
	}

	// Wait a moment for cleanup
	time.Sleep(500 * time.Millisecond)

	return m.StartAgent(configID)
}

// GetAgentState returns the current state of an agent.
func (m *Manager) GetAgentState(configID string) (*models.AgentState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[configID]
	if !exists {
		return &models.AgentState{
			ConfigID: configID,
			Status:   "stopped",
		}, nil
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	// Return a copy
	state := *agent.State
	return &state, nil
}

// ListAgentStates returns the states of all agents.
func (m *Manager) ListAgentStates() ([]*models.AgentState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Load all configs to include stopped agents
	configs, err := m.storage.ListAgentConfigs(nil)
	if err != nil {
		return nil, err
	}

	states := make([]*models.AgentState, 0, len(configs))

	for _, cfg := range configs {
		if agent, exists := m.agents[cfg.ID]; exists {
			agent.mu.Lock()
			state := *agent.State
			agent.mu.Unlock()
			states = append(states, &state)
		} else {
			// Not running
			states = append(states, &models.AgentState{
				ConfigID: cfg.ID,
				Status:   "stopped",
			})
		}
	}

	return states, nil
}

// watchProcess monitors an agent process and updates state when it exits.
func (m *Manager) watchProcess(agent *AgentProcess) {
	err := agent.Cmd.Wait()

	agent.mu.Lock()
	now := time.Now()
	agent.State.StoppedAt = &now

	if err != nil {
		agent.State.Status = "failed"
		agent.State.ErrorMessage = err.Error()
		log.Printf("Agent %s failed: %v", agent.Config.Name, err)
	} else {
		agent.State.Status = "stopped"
		log.Printf("Agent %s stopped", agent.Config.Name)
	}
	agent.mu.Unlock()
}

// monitorAgents periodically checks agent health.
func (m *Manager) monitorAgents() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAgentHealth()
		}
	}
}

// checkAgentHealth checks if agents are still running.
func (m *Manager) checkAgentHealth() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, agent := range m.agents {
		agent.mu.Lock()
		if agent.State.Status == "running" && agent.Cmd != nil && agent.Cmd.Process != nil {
			// Check if process is still alive by sending signal 0
			// Signal 0 doesn't actually signal the process, but returns an error if it doesn't exist
			if err := agent.Cmd.Process.Signal(syscall.Signal(0)); err != nil {
				// Process is dead
				now := time.Now()
				agent.State.Status = "failed"
				agent.State.StoppedAt = &now
				agent.State.ErrorMessage = "Process died unexpectedly"
				log.Printf("Agent %s died unexpectedly", agent.Config.Name)
			}
		}
		agent.mu.Unlock()
	}
}
