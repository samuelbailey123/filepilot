package vfs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ConnectionConfig stores saved connection details.
type ConnectionConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"` // sftp, ftp, webdav
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	// Password is NOT stored here — it goes in macOS Keychain.
	// PrivateKeyPath is stored for SFTP key-based auth.
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`
	BasePath       string `json:"basePath,omitempty"` // Initial remote path.
}

// ConnManager manages saved connections and active remote VFS sessions.
type ConnManager struct {
	mu          sync.RWMutex
	active      map[string]FileSystem // connID -> active VFS
	configDir   string                // ~/.config/filepilot
	connections []ConnectionConfig    // Saved connections (loaded from disk).
}

// NewConnManager creates a connection manager that persists connections
// to the given config directory.
func NewConnManager(configDir string) *ConnManager {
	cm := &ConnManager{
		active:    make(map[string]FileSystem),
		configDir: configDir,
	}
	cm.loadConnections()
	return cm
}

// RemotePathPrefix is the prefix for remote filesystem paths.
// Remote paths follow the format: remote://{connID}/path/to/file
const RemotePathPrefix = "remote://"

// IsRemotePath reports whether the given path is a remote VFS path.
func IsRemotePath(p string) bool {
	return strings.HasPrefix(p, RemotePathPrefix)
}

// ParseRemotePath splits a remote path into connection ID and remote path.
// Input: "remote://{connID}/path/to/file" -> connID, "/path/to/file"
func ParseRemotePath(p string) (connID string, remotePath string, err error) {
	if !IsRemotePath(p) {
		return "", "", fmt.Errorf("not a remote path: %s", p)
	}
	rest := strings.TrimPrefix(p, RemotePathPrefix)
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return rest, "/", nil
	}
	return rest[:idx], rest[idx:], nil
}

// BuildRemotePath constructs a remote path from connection ID and remote path.
func BuildRemotePath(connID, remotePath string) string {
	if remotePath == "" || remotePath == "/" {
		return RemotePathPrefix + connID + "/"
	}
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}
	return RemotePathPrefix + connID + remotePath
}

// SaveConnection persists a connection config to disk.
func (cm *ConnManager) SaveConnection(cfg ConnectionConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cfg.ID == "" {
		return fmt.Errorf("connection ID is required")
	}

	// Update or add.
	found := false
	for i, c := range cm.connections {
		if c.ID == cfg.ID {
			cm.connections[i] = cfg
			found = true
			break
		}
	}
	if !found {
		cm.connections = append(cm.connections, cfg)
	}

	return cm.saveConnections()
}

// DeleteConnection removes a saved connection.
func (cm *ConnManager) DeleteConnection(connID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Disconnect if active.
	if fs, ok := cm.active[connID]; ok {
		closeFS(fs)
		delete(cm.active, connID)
	}

	// Remove from saved list.
	filtered := cm.connections[:0]
	for _, c := range cm.connections {
		if c.ID != connID {
			filtered = append(filtered, c)
		}
	}
	cm.connections = filtered

	return cm.saveConnections()
}

// ListConnections returns all saved connections.
func (cm *ConnManager) ListConnections() []ConnectionConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]ConnectionConfig, len(cm.connections))
	copy(result, cm.connections)
	return result
}

// GetConnection returns a saved connection by ID.
func (cm *ConnManager) GetConnection(connID string) (ConnectionConfig, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, c := range cm.connections {
		if c.ID == connID {
			return c, true
		}
	}
	return ConnectionConfig{}, false
}

// SetActive registers an active VFS for a connection ID.
func (cm *ConnManager) SetActive(connID string, fs FileSystem) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.active[connID] = fs
}

// GetActive returns the active VFS for a connection ID.
func (cm *ConnManager) GetActive(connID string) (FileSystem, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	fs, ok := cm.active[connID]
	return fs, ok
}

// Disconnect closes an active connection.
func (cm *ConnManager) Disconnect(connID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if fs, ok := cm.active[connID]; ok {
		closeFS(fs)
		delete(cm.active, connID)
	}
	return nil
}

// DisconnectAll closes all active connections.
func (cm *ConnManager) DisconnectAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for id, fs := range cm.active {
		closeFS(fs)
		delete(cm.active, id)
	}
}

// IsActive reports whether a connection is currently active.
func (cm *ConnManager) IsActive(connID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.active[connID]
	return ok
}

// configPath returns the path to the connections config file.
func (cm *ConnManager) configPath() string {
	return filepath.Join(cm.configDir, "connections.json")
}

// loadConnections reads saved connections from disk.
func (cm *ConnManager) loadConnections() {
	data, err := os.ReadFile(cm.configPath())
	if err != nil {
		return // File doesn't exist yet.
	}
	json.Unmarshal(data, &cm.connections)
}

// saveConnections writes connections to disk.
func (cm *ConnManager) saveConnections() error {
	if err := os.MkdirAll(cm.configDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cm.connections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.configPath(), data, 0644)
}

// closeFS closes a FileSystem if it implements io.Closer.
func closeFS(fs FileSystem) {
	type closer interface {
		Close() error
	}
	if c, ok := fs.(closer); ok {
		c.Close()
	}
}
