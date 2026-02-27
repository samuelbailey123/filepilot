package vfs

import (
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

// FTPConfig holds the configuration for an FTP connection.
type FTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Timeout  time.Duration
}

// FTP implements the FileSystem interface for FTP servers.
type FTP struct {
	mu     sync.Mutex
	conn   *ftp.ServerConn
	config FTPConfig
}

// NewFTP creates a new FTP filesystem by connecting to the server.
func NewFTP(config FTPConfig) (*FTP, error) {
	if config.Host == "" {
		return nil, fmt.Errorf("FTP host is required")
	}
	if config.Port == 0 {
		config.Port = 21
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(config.Timeout))
	if err != nil {
		return nil, fmt.Errorf("FTP dial %s: %w", addr, err)
	}

	user := config.User
	if user == "" {
		user = "anonymous"
	}
	pass := config.Password
	if pass == "" && user == "anonymous" {
		pass = "filepilot@"
	}

	if err := conn.Login(user, pass); err != nil {
		conn.Quit()
		return nil, fmt.Errorf("FTP login: %w", err)
	}

	return &FTP{conn: conn, config: config}, nil
}

// Close disconnects from the FTP server.
func (f *FTP) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn != nil {
		err := f.conn.Quit()
		f.conn = nil
		return err
	}
	return nil
}

// IsConnected reports whether the FTP client is connected.
func (f *FTP) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conn != nil
}

// List returns directory contents at the given path.
func (f *FTP) List(dirPath string) ([]FileEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return nil, fmt.Errorf("FTP not connected")
	}

	dirPath = normFTPPath(dirPath)
	entries, err := f.conn.List(dirPath)
	if err != nil {
		return nil, fmt.Errorf("FTP list %s: %w", dirPath, err)
	}

	result := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		result = append(result, FileEntry{
			Path:    path.Join(dirPath, e.Name),
			Name:    e.Name,
			Size:    int64(e.Size),
			IsDir:   e.Type == ftp.EntryTypeFolder,
			ModTime: e.Time,
		})
	}
	return result, nil
}

// Stat returns metadata for a single file or directory.
func (f *FTP) Stat(filePath string) (*FileEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return nil, fmt.Errorf("FTP not connected")
	}

	filePath = normFTPPath(filePath)
	entry, err := f.conn.GetEntry(filePath)
	if err != nil {
		return nil, fmt.Errorf("FTP stat %s: %w", filePath, err)
	}

	return &FileEntry{
		Path:    filePath,
		Name:    path.Base(filePath),
		Size:    int64(entry.Size),
		IsDir:   entry.Type == ftp.EntryTypeFolder,
		ModTime: entry.Time,
	}, nil
}

// Read opens a file for reading.
func (f *FTP) Read(filePath string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return nil, fmt.Errorf("FTP not connected")
	}

	filePath = normFTPPath(filePath)
	resp, err := f.conn.Retr(filePath)
	if err != nil {
		return nil, fmt.Errorf("FTP read %s: %w", filePath, err)
	}
	return resp, nil
}

// Write writes content to a file, creating or overwriting it.
func (f *FTP) Write(filePath string, r io.Reader) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return fmt.Errorf("FTP not connected")
	}

	filePath = normFTPPath(filePath)
	return f.conn.Stor(filePath, r)
}

// Mkdir creates a directory at the given path.
func (f *FTP) Mkdir(dirPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return fmt.Errorf("FTP not connected")
	}

	dirPath = normFTPPath(dirPath)
	return f.conn.MakeDir(dirPath)
}

// Remove deletes a file or directory.
func (f *FTP) Remove(filePath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return fmt.Errorf("FTP not connected")
	}

	filePath = normFTPPath(filePath)

	// Try file delete first, fall back to directory removal.
	err := f.conn.Delete(filePath)
	if err != nil {
		return f.conn.RemoveDir(filePath)
	}
	return nil
}

// Rename renames or moves a file.
func (f *FTP) Rename(oldPath, newPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return fmt.Errorf("FTP not connected")
	}

	return f.conn.Rename(normFTPPath(oldPath), normFTPPath(newPath))
}

// Copy copies a file from src to dst by downloading and re-uploading.
func (f *FTP) Copy(src, dst string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return fmt.Errorf("FTP not connected")
	}

	src = normFTPPath(src)
	dst = normFTPPath(dst)

	resp, err := f.conn.Retr(src)
	if err != nil {
		return fmt.Errorf("FTP read %s: %w", src, err)
	}
	defer resp.Close()

	return f.conn.Stor(dst, resp)
}

// Move moves a file from src to dst (same as Rename for FTP).
func (f *FTP) Move(src, dst string) error {
	return f.Rename(src, dst)
}

// normFTPPath ensures the path starts with / and has no trailing slash.
func normFTPPath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}
