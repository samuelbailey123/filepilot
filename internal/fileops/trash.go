package fileops

import (
	"fmt"
	"os/exec"
)

// MoveToTrash moves a file to the macOS Trash using osascript.
func (o *Ops) MoveToTrash(path string) error {
	script := fmt.Sprintf(
		`tell application "Finder" to delete POSIX file %q`,
		path,
	)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("move to trash: %s: %w", string(output), err)
	}

	o.pushUndo(UndoEntry{Type: OpDelete, From: path, TrashRef: path})
	return nil
}

// RevealInFinder opens the parent directory in Finder and selects the file.
func RevealInFinder(path string) error {
	cmd := exec.Command("open", "-R", path)
	return cmd.Run()
}

// OpenWithDefault opens a file with the default application.
func OpenWithDefault(path string) error {
	cmd := exec.Command("open", path)
	return cmd.Run()
}

// OpenWithApp opens a file with a specific application.
func OpenWithApp(path, app string) error {
	cmd := exec.Command("open", "-a", app, path)
	return cmd.Run()
}

// GetFileInfo returns extended file information using mdls (Spotlight metadata).
func GetFileInfo(path string) (string, error) {
	cmd := exec.Command("mdls", path)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("mdls: %w", err)
	}
	return string(output), nil
}
