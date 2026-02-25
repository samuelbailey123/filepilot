package fileops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// OpType describes the type of file operation for undo tracking.
type OpType string

const (
	OpMove   OpType = "move"
	OpCopy   OpType = "copy"
	OpRename OpType = "rename"
	OpDelete OpType = "delete"
)

// UndoEntry records a reversible file operation.
type UndoEntry struct {
	Type     OpType `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	TrashRef string `json:"trashRef,omitempty"`
}

// Ops provides file operations with an undo stack.
type Ops struct {
	undoStack []UndoEntry
	maxUndo   int
	mu        sync.Mutex
}

// NewOps creates a new file operations handler.
func NewOps() *Ops {
	return &Ops{
		undoStack: make([]UndoEntry, 0, 100),
		maxUndo:   100,
	}
}

// Move moves a file or directory from src to dst.
func (o *Ops) Move(src, dst string) error {
	if err := validatePaths(src, dst); err != nil {
		return err
	}

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("move %s to %s: %w", src, dst, err)
	}

	o.pushUndo(UndoEntry{Type: OpMove, From: src, To: dst})
	return nil
}

// Copy copies a file or directory from src to dst.
func (o *Ops) Copy(src, dst string) error {
	if err := validatePaths(src, dst); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if info.IsDir() {
		if err := copyDir(src, dst); err != nil {
			return err
		}
	} else {
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	o.pushUndo(UndoEntry{Type: OpCopy, From: src, To: dst})
	return nil
}

// Rename renames a file or directory.
func (o *Ops) Rename(path, newName string) (string, error) {
	if newName == "" {
		return "", fmt.Errorf("new name cannot be empty")
	}

	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, newName)

	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("destination already exists: %s", newPath)
	}

	if err := os.Rename(path, newPath); err != nil {
		return "", fmt.Errorf("rename: %w", err)
	}

	o.pushUndo(UndoEntry{Type: OpRename, From: path, To: newPath})
	return newPath, nil
}

// Undo reverses the last file operation.
func (o *Ops) Undo() (*UndoEntry, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.undoStack) == 0 {
		return nil, fmt.Errorf("nothing to undo")
	}

	entry := o.undoStack[len(o.undoStack)-1]
	o.undoStack = o.undoStack[:len(o.undoStack)-1]

	switch entry.Type {
	case OpMove, OpRename:
		if err := os.Rename(entry.To, entry.From); err != nil {
			return &entry, fmt.Errorf("undo move: %w", err)
		}
	case OpCopy:
		if err := os.RemoveAll(entry.To); err != nil {
			return &entry, fmt.Errorf("undo copy: %w", err)
		}
	case OpDelete:
		// Trash undo is handled via trash.go RestoreFromTrash.
		return &entry, fmt.Errorf("use RestoreFromTrash for delete undo")
	}

	return &entry, nil
}

// UndoCount returns the number of undo-able operations.
func (o *Ops) UndoCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.undoStack)
}

// CreateDir creates a new directory at the given path.
func (o *Ops) CreateDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// CreateFile creates a new empty file at the given path.
func (o *Ops) CreateFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	return f.Close()
}

// pushUndo adds an entry to the undo stack.
func (o *Ops) pushUndo(entry UndoEntry) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.undoStack = append(o.undoStack, entry)
	if len(o.undoStack) > o.maxUndo {
		o.undoStack = o.undoStack[1:]
	}
}

// validatePaths checks that source exists and destination doesn't.
func validatePaths(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source does not exist: %s", src)
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination already exists: %s", dst)
	}
	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
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
func copyDir(src, dst string) error {
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
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
