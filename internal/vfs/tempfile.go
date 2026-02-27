package vfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TempFileManager manages temporary local copies of remote files.
// Remote files are downloaded to a local temp directory for preview,
// editing, and serving via the asset server.
type TempFileManager struct {
	mu      sync.Mutex
	baseDir string
	files   map[string]tempFileEntry // remotePath -> local temp info
}

type tempFileEntry struct {
	LocalPath  string
	RemotePath string
	ConnID     string
}

// NewTempFileManager creates a temp file manager rooted at the given directory.
func NewTempFileManager(baseDir string) (*TempFileManager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	return &TempFileManager{
		baseDir: baseDir,
		files:   make(map[string]tempFileEntry),
	}, nil
}

// Download fetches a remote file to a local temp path for preview/editing.
// Returns the local file path. If already downloaded, returns the cached path.
func (m *TempFileManager) Download(fs FileSystem, connID, remotePath string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullKey := connID + ":" + remotePath

	// Return cached path if already downloaded.
	if entry, ok := m.files[fullKey]; ok {
		if _, err := os.Stat(entry.LocalPath); err == nil {
			return entry.LocalPath, nil
		}
		// File was deleted; re-download.
		delete(m.files, fullKey)
	}

	// Create a directory structure under the temp dir matching the remote path.
	connDir := filepath.Join(m.baseDir, sanitizeDirName(connID))
	localDir := filepath.Join(connDir, filepath.Dir(remotePath))
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return "", fmt.Errorf("create local dir: %w", err)
	}

	localPath := filepath.Join(connDir, remotePath)

	// Download the file.
	reader, err := fs.Read(remotePath)
	if err != nil {
		return "", fmt.Errorf("read remote file: %w", err)
	}
	defer reader.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("create local file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(localPath)
		return "", fmt.Errorf("download file: %w", err)
	}

	m.files[fullKey] = tempFileEntry{
		LocalPath:  localPath,
		RemotePath: remotePath,
		ConnID:     connID,
	}

	return localPath, nil
}

// Upload pushes a local temp file back to the remote filesystem.
func (m *TempFileManager) Upload(fs FileSystem, connID, remotePath string) error {
	m.mu.Lock()
	entry, ok := m.files[connID+":"+remotePath]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("no temp file for %s:%s", connID, remotePath)
	}

	f, err := os.Open(entry.LocalPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	return fs.Write(remotePath, f)
}

// GetLocalPath returns the local temp path for a remote file, or empty string.
func (m *TempFileManager) GetLocalPath(connID, remotePath string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.files[connID+":"+remotePath]
	if !ok {
		return ""
	}
	return entry.LocalPath
}

// Cleanup removes all temp files and clears the cache.
func (m *TempFileManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	os.RemoveAll(m.baseDir)
	m.files = make(map[string]tempFileEntry)
}

// Invalidate removes a specific cached file.
func (m *TempFileManager) Invalidate(connID, remotePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := connID + ":" + remotePath
	if entry, ok := m.files[key]; ok {
		os.Remove(entry.LocalPath)
		delete(m.files, key)
	}
}

// sanitizeDirName makes a string safe to use as a directory name.
func sanitizeDirName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '\x00' || r == ' ' {
			return '_'
		}
		return r
	}, s)
}
