package vfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Local implements the FileSystem interface for the local filesystem.
type Local struct{}

// NewLocal creates a new local filesystem instance.
func NewLocal() *Local {
	return &Local{}
}

// List returns directory contents at the given path.
// Uses os.Lstat so that symlinks are reported as symlinks rather than
// being silently followed, which lets callers distinguish them.
func (l *Local) List(path string) ([]FileEntry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", path, err)
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		entryPath := filepath.Join(path, de.Name())

		// Lstat does not follow symlinks, so we can detect them.
		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		symlinkTarget := ""
		isDir := info.IsDir()

		if isSymlink {
			target, readErr := os.Readlink(entryPath)
			if readErr == nil {
				symlinkTarget = target
			}
			// Follow the symlink to determine whether the target is a directory.
			if targetInfo, statErr := os.Stat(entryPath); statErr == nil {
				isDir = targetInfo.IsDir()
			}
		}

		entries = append(entries, FileEntry{
			Path:          entryPath,
			Name:          de.Name(),
			Size:          info.Size(),
			IsDir:         isDir,
			ModTime:       info.ModTime(),
			Permissions:   int(info.Mode().Perm()),
			IsSymlink:     isSymlink,
			SymlinkTarget: symlinkTarget,
		})
	}
	return entries, nil
}

// Stat returns metadata for a single file or directory.
// Uses os.Lstat first so that symlinks are identified; then resolves the
// target with os.Stat to determine whether the destination is a directory.
func (l *Local) Stat(path string) (*FileEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	symlinkTarget := ""
	isDir := info.IsDir()

	if isSymlink {
		target, readErr := os.Readlink(path)
		if readErr == nil {
			symlinkTarget = target
		}
		if targetInfo, statErr := os.Stat(path); statErr == nil {
			isDir = targetInfo.IsDir()
		}
	}

	return &FileEntry{
		Path:          path,
		Name:          filepath.Base(path),
		Size:          info.Size(),
		IsDir:         isDir,
		ModTime:       info.ModTime(),
		Permissions:   int(info.Mode().Perm()),
		IsSymlink:     isSymlink,
		SymlinkTarget: symlinkTarget,
	}, nil
}

// Read opens a file for reading.
func (l *Local) Read(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, nil
}

// Write writes content to a file, creating or overwriting it.
func (l *Local) Write(path string, r io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	return f.Close()
}

// Mkdir creates a directory at the given path, including parents.
func (l *Local) Mkdir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Remove deletes a file or directory (recursive for directories).
func (l *Local) Remove(path string) error {
	return os.RemoveAll(path)
}

// Rename renames or moves a file.
func (l *Local) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Copy copies a file or directory from src to dst.
func (l *Local) Copy(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if info.IsDir() {
		return l.copyDir(src, dst)
	}
	return l.copyFile(src, dst)
}

// Move moves a file from src to dst.
func (l *Local) Move(src, dst string) error {
	return os.Rename(src, dst)
}

// copyFile copies a single file preserving permissions.
func (l *Local) copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}
	return out.Close()
}

// copyDir recursively copies a directory tree.
func (l *Local) copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := l.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := l.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
