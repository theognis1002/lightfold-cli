package state

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
	"time"
)

// Runtime represents a language runtime (imported from pkg/runtime to avoid circular import)
type Runtime string

// ServerState tracks all apps deployed to a single server
type ServerState struct {
	ServerIP          string        `json:"server_ip"`
	Provider          string        `json:"provider"`           // "digitalocean", "vultr", "hetzner", "byos"
	ServerID          string        `json:"server_id"`          // Droplet/instance ID (empty for BYOS)
	ProxyType         string        `json:"proxy_type"`         // "caddy" or "nginx"
	RootDomain        string        `json:"root_domain"`        // Optional: example.com
	DeployedApps      []DeployedApp `json:"deployed_apps"`      // All apps on this server
	InstalledRuntimes []Runtime     `json:"installed_runtimes"` // Runtimes installed on server
	NextPort          int           `json:"next_port"`          // Next available port
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// DeployedApp represents an app deployed to a server
type DeployedApp struct {
	TargetName string    `json:"target_name"` // Lightfold target name
	AppName    string    `json:"app_name"`    // App identifier (usually same as target)
	Port       int       `json:"port"`        // Assigned port
	Domain     string    `json:"domain"`      // Full domain (app.example.com or empty)
	Framework  string    `json:"framework"`   // Next.js, Django, etc.
	LastDeploy time.Time `json:"last_deploy"`
}

// GetServersPath returns the directory where server state files are stored
func GetServersPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(config.LocalConfigDir, "servers")
	}
	return filepath.Join(homeDir, config.LocalConfigDir, "servers")
}

// GetServerStatePath returns the path to a specific server's state file
func GetServerStatePath(serverIP string) string {
	// Sanitize IP for filename (use filepath.Base to remove any path separators)
	sanitized := filepath.Base(serverIP)
	return filepath.Join(GetServersPath(), sanitized+".json")
}

// ServerStateExists checks if a server state file exists
func ServerStateExists(serverIP string) bool {
	statePath := GetServerStatePath(serverIP)
	_, err := os.Stat(statePath)
	return err == nil
}

// GetServerState loads server state from disk
func GetServerState(serverIP string) (*ServerState, error) {
	statePath := GetServerStatePath(serverIP)

	// Ensure directory exists
	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, config.PermDirectory); err != nil {
		return nil, fmt.Errorf("failed to create servers directory: %w", err)
	}

	// Return empty state if file doesn't exist
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return &ServerState{
			ServerIP:          serverIP,
			DeployedApps:      []DeployedApp{},
			InstalledRuntimes: []Runtime{},
			NextPort:          PortRangeStart,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}, nil
	}

	// Load existing state
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server state file: %w", err)
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse server state file: %w", err)
	}

	// Ensure DeployedApps is initialized
	if state.DeployedApps == nil {
		state.DeployedApps = []DeployedApp{}
	}

	// Ensure InstalledRuntimes is initialized
	if state.InstalledRuntimes == nil {
		state.InstalledRuntimes = []Runtime{}
	}

	// Initialize NextPort if not set
	if state.NextPort == 0 {
		state.NextPort = PortRangeStart
	}

	return &state, nil
}

// SaveServerState persists server state to disk
func SaveServerState(state *ServerState) error {
	if state == nil {
		return fmt.Errorf("cannot save nil server state")
	}

	statePath := GetServerStatePath(state.ServerIP)

	// Ensure directory exists
	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, config.PermDirectory); err != nil {
		return fmt.Errorf("failed to create servers directory: %w", err)
	}

	// Update timestamp
	state.UpdatedAt = time.Now()

	// Marshal and write
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal server state: %w", err)
	}

	if err := os.WriteFile(statePath, data, config.PermConfigFile); err != nil {
		return fmt.Errorf("failed to write server state file: %w", err)
	}

	return nil
}

// DeleteServerState removes a server state file
func DeleteServerState(serverIP string) error {
	statePath := GetServerStatePath(serverIP)

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("failed to delete server state file: %w", err)
	}

	return nil
}

// RegisterApp adds or updates an app in the server's deployed apps list
func RegisterApp(serverIP string, app DeployedApp) error {
	state, err := GetServerState(serverIP)
	if err != nil {
		return err
	}

	// Check if app already exists
	found := false
	for i, existingApp := range state.DeployedApps {
		if existingApp.TargetName == app.TargetName {
			// Update existing app
			state.DeployedApps[i] = app
			found = true
			break
		}
	}

	// Add new app if not found
	if !found {
		state.DeployedApps = append(state.DeployedApps, app)
	}

	return SaveServerState(state)
}

// UnregisterApp removes an app from the server's deployed apps list
func UnregisterApp(serverIP, targetName string) error {
	state, err := GetServerState(serverIP)
	if err != nil {
		return err
	}

	// Find and remove app
	newApps := []DeployedApp{}
	for _, app := range state.DeployedApps {
		if app.TargetName != targetName {
			newApps = append(newApps, app)
		}
	}

	state.DeployedApps = newApps

	// Delete server state if no apps remain
	if len(state.DeployedApps) == 0 {
		return DeleteServerState(serverIP)
	}

	return SaveServerState(state)
}

// GetAppFromServer retrieves a specific app's info from server state
func GetAppFromServer(serverIP, targetName string) (*DeployedApp, error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return nil, err
	}

	for _, app := range state.DeployedApps {
		if app.TargetName == targetName {
			return &app, nil
		}
	}

	return nil, fmt.Errorf("app not found on server: %s", targetName)
}

// ListAppsOnServer returns all apps deployed to a server
func ListAppsOnServer(serverIP string) ([]DeployedApp, error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return nil, err
	}

	return state.DeployedApps, nil
}

// UpdateAppDeployment updates the last deploy time for an app
func UpdateAppDeployment(serverIP, targetName string, timestamp time.Time) error {
	state, err := GetServerState(serverIP)
	if err != nil {
		return err
	}

	for i, app := range state.DeployedApps {
		if app.TargetName == targetName {
			state.DeployedApps[i].LastDeploy = timestamp
			return SaveServerState(state)
		}
	}

	return fmt.Errorf("app not found on server: %s", targetName)
}

// RegisterRuntime adds a runtime to the server's installed runtimes list
func RegisterRuntime(serverIP string, runtime Runtime) error {
	state, err := GetServerState(serverIP)
	if err != nil {
		return err
	}

	// Check if runtime already exists
	for _, rt := range state.InstalledRuntimes {
		if rt == runtime {
			// Already registered, nothing to do
			return nil
		}
	}

	// Add new runtime
	state.InstalledRuntimes = append(state.InstalledRuntimes, runtime)

	return SaveServerState(state)
}

// ListAllServers returns a list of all server IPs with state files
func ListAllServers() ([]string, error) {
	serversPath := GetServersPath()

	// Ensure directory exists
	if err := os.MkdirAll(serversPath, config.PermDirectory); err != nil {
		return nil, fmt.Errorf("failed to create servers directory: %w", err)
	}

	// Read directory
	entries, err := os.ReadDir(serversPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read servers directory: %w", err)
	}

	// Extract server IPs from filenames
	var servers []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			// Remove .json extension to get sanitized IP
			servers = append(servers, name[:len(name)-5])
		}
	}

	return servers, nil
}
