// Package vfs defines the FileSystem interface and implementations for local, SFTP, and S3 filesystems.
package vfs

import (
	"io"
	"time"
)

// FileEntry represents a single file or directory in a filesystem.
type FileEntry struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	IsDir         bool      `json:"isDir"`
	ModTime       time.Time `json:"modTime"`
	Permissions   int       `json:"permissions"`
	IsSymlink     bool      `json:"isSymlink"`
	SymlinkTarget string    `json:"symlinkTarget"`
}

// FileSystem defines the interface for filesystem operations.
// Implementations include local filesystem, SFTP, FTP, S3, WebDAV, and SMB.
type FileSystem interface {
	// List returns directory contents at the given path.
	List(path string) ([]FileEntry, error)

	// Stat returns metadata for a single file or directory.
	Stat(path string) (*FileEntry, error)

	// Read opens a file for reading. Caller must close the returned reader.
	Read(path string) (io.ReadCloser, error)

	// Write writes content to a file, creating or overwriting it.
	Write(path string, r io.Reader) error

	// Mkdir creates a directory at the given path, including parents.
	Mkdir(path string) error

	// Remove deletes a file or empty directory.
	Remove(path string) error

	// Rename renames or moves a file within the same filesystem.
	Rename(oldPath, newPath string) error

	// Copy copies a file from src to dst within the same filesystem.
	Copy(src, dst string) error

	// Move moves a file from src to dst within the same filesystem.
	Move(src, dst string) error
}
