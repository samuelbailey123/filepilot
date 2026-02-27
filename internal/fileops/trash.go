package fileops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// RestoreFromTrash moves a file from the macOS Trash back to its original
// location. It first attempts a direct os.Rename from ~/.Trash/<name> to the
// original path, which is fast and works across most cases. If the rename
// fails or the file is not found in ~/.Trash, it falls back to osascript so
// that Finder can handle any edge cases such as iCloud-backed or sandboxed
// trash locations.
func (o *Ops) RestoreFromTrash(originalPath string) error {
	name := filepath.Base(originalPath)
	trashPath := filepath.Join(os.Getenv("HOME"), ".Trash", name)
	dir := filepath.Dir(originalPath)

	// Fast path: rename directly from ~/.Trash back to original location.
	if _, err := os.Stat(trashPath); err == nil {
		if renameErr := os.Rename(trashPath, originalPath); renameErr == nil {
			return nil
		}
	}

	// Fallback: ask Finder to move the item back using osascript.
	script := fmt.Sprintf(
		`tell application "Finder" to move (POSIX file %q as alias) to POSIX file %q`,
		trashPath,
		dir,
	)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore from trash: %s: %w", string(output), err)
	}
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

// RequestICloudDownload triggers the download of an iCloud placeholder file
// using macOS brctl (Bird Control). This is a non-blocking call — the actual
// download happens asynchronously in the background.
func RequestICloudDownload(path string) error {
	cmd := exec.Command("brctl", "download", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brctl download: %s: %w", string(output), err)
	}
	return nil
}
