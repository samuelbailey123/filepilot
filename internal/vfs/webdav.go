package vfs

import (
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/studio-b12/gowebdav"
)

// WebDAVConfig holds the configuration for a WebDAV connection.
type WebDAVConfig struct {
	URL      string // Base URL (e.g., https://server.com/dav/)
	User     string
	Password string
	Timeout  time.Duration
}

// WebDAV implements the FileSystem interface for WebDAV servers.
type WebDAV struct {
	mu     sync.Mutex
	client *gowebdav.Client
	config WebDAVConfig
}

// NewWebDAV creates a new WebDAV filesystem by connecting to the server.
func NewWebDAV(config WebDAVConfig) (*WebDAV, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("WebDAV URL is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	client := gowebdav.NewClient(config.URL, config.User, config.Password)
	client.SetTimeout(config.Timeout)

	// Verify connectivity.
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("WebDAV connect %s: %w", config.URL, err)
	}

	return &WebDAV{client: client, config: config}, nil
}

// Close disconnects from the WebDAV server. WebDAV is HTTP-based
// so there is no persistent connection to close.
func (w *WebDAV) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client = nil
	return nil
}

// IsConnected reports whether the WebDAV client is available.
func (w *WebDAV) IsConnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client != nil
}

// List returns directory contents at the given path.
func (w *WebDAV) List(dirPath string) ([]FileEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return nil, fmt.Errorf("WebDAV not connected")
	}

	dirPath = normWebDAVPath(dirPath)
	infos, err := w.client.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV list %s: %w", dirPath, err)
	}

	result := make([]FileEntry, 0, len(infos))
	for _, info := range infos {
		result = append(result, FileEntry{
			Path:    path.Join(dirPath, info.Name()),
			Name:    info.Name(),
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	return result, nil
}

// Stat returns metadata for a single file or directory.
func (w *WebDAV) Stat(filePath string) (*FileEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return nil, fmt.Errorf("WebDAV not connected")
	}

	filePath = normWebDAVPath(filePath)
	info, err := w.client.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV stat %s: %w", filePath, err)
	}

	return &FileEntry{
		Path:    filePath,
		Name:    info.Name(),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}, nil
}

// Read opens a file for reading.
func (w *WebDAV) Read(filePath string) (io.ReadCloser, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return nil, fmt.Errorf("WebDAV not connected")
	}

	filePath = normWebDAVPath(filePath)
	stream, err := w.client.ReadStream(filePath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV read %s: %w", filePath, err)
	}
	return stream, nil
}

// Write writes content to a file, creating or overwriting it.
func (w *WebDAV) Write(filePath string, r io.Reader) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return fmt.Errorf("WebDAV not connected")
	}

	filePath = normWebDAVPath(filePath)
	return w.client.WriteStream(filePath, r, 0644)
}

// Mkdir creates a directory at the given path.
func (w *WebDAV) Mkdir(dirPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return fmt.Errorf("WebDAV not connected")
	}

	dirPath = normWebDAVPath(dirPath)
	return w.client.MkdirAll(dirPath, 0755)
}

// Remove deletes a file or directory.
func (w *WebDAV) Remove(filePath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return fmt.Errorf("WebDAV not connected")
	}

	filePath = normWebDAVPath(filePath)
	return w.client.Remove(filePath)
}

// Rename renames or moves a file.
func (w *WebDAV) Rename(oldPath, newPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return fmt.Errorf("WebDAV not connected")
	}

	return w.client.Rename(normWebDAVPath(oldPath), normWebDAVPath(newPath), true)
}

// Copy copies a file from src to dst.
func (w *WebDAV) Copy(src, dst string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.client == nil {
		return fmt.Errorf("WebDAV not connected")
	}

	return w.client.Copy(normWebDAVPath(src), normWebDAVPath(dst), true)
}

// Move moves a file from src to dst (same as Rename for WebDAV).
func (w *WebDAV) Move(src, dst string) error {
	return w.Rename(src, dst)
}

// normWebDAVPath ensures the path starts with / and has no trailing slash.
func normWebDAVPath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}
