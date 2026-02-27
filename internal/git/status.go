// Package git provides Git repository detection and file status parsing.
package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FileStatus describes the git status of a file in the working tree.
type FileStatus string

const (
	// FileStatusModified indicates the file has unstaged or staged modifications.
	FileStatusModified FileStatus = "modified"

	// FileStatusAdded indicates the file is newly staged for the first time.
	FileStatusAdded FileStatus = "added"

	// FileStatusDeleted indicates the file has been deleted.
	FileStatusDeleted FileStatus = "deleted"

	// FileStatusRenamed indicates the file has been renamed.
	FileStatusRenamed FileStatus = "renamed"

	// FileStatusUntracked indicates the file is not tracked by git.
	FileStatusUntracked FileStatus = "untracked"

	// FileStatusIgnored indicates the file is ignored via .gitignore rules.
	FileStatusIgnored FileStatus = "ignored"

	// FileStatusStaged indicates the file has been staged (added to the index).
	FileStatusStaged FileStatus = "staged"
)

// gitTimeout is the maximum time allowed for any git subprocess call.
const gitTimeout = 5 * time.Second

// IsGitRepo reports whether dirPath is inside a git repository. It walks up
// the directory tree looking for a .git entry, stopping at the filesystem root.
func IsGitRepo(dirPath string) bool {
	current, err := filepath.Abs(dirPath)
	if err != nil {
		return false
	}

	for {
		candidate := filepath.Join(current, ".git")
		if _, err := os.Stat(candidate); err == nil {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without finding .git.
			return false
		}
		current = parent
	}
}

// FindRepoRoot returns the absolute path to the root of the git repository
// that contains dirPath. It delegates to `git rev-parse --show-toplevel` so
// that worktrees, submodules, and non-standard layouts are all handled
// correctly.
func FindRepoRoot(dirPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dirPath

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel in %s: %w", dirPath, err)
	}

	root := strings.TrimRight(string(out), "\n\r")
	return root, nil
}

// GetStatus runs `git status --porcelain -uall` inside repoRoot and returns a
// map of relative file path to FileStatus. Files that do not appear in the
// porcelain output (i.e. clean, committed files) are not included in the map.
func GetStatus(repoRoot string) (map[string]FileStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "-uall")
	cmd.Dir = repoRoot

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status in %s: %w", repoRoot, err)
	}

	return parsePorcelain(out), nil
}

// parsePorcelain converts the raw bytes from `git status --porcelain` into a
// map of relative path -> FileStatus.
//
// The porcelain v1 format is two-character status codes followed by a space
// and the file path. For renames the format is:
//
//	R  new-name -> old-name
//
// We report the new (destination) path as the map key in that case.
func parsePorcelain(data []byte) map[string]FileStatus {
	result := make(map[string]FileStatus)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			// Minimum valid porcelain line: XY followed by space and at least one char.
			continue
		}

		xy := line[:2]
		// Column 3 is a space separator; the path starts at column 3.
		path := strings.TrimSpace(line[3:])

		// Renamed entries include " -> " separating the new and old paths.
		// We key by the new (right-hand) path.
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[0]
		}

		status := porcelainToStatus(xy)
		if status == "" {
			continue
		}

		result[path] = status
	}

	return result
}

// porcelainToStatus maps a two-character git porcelain XY code to a FileStatus
// value. Returns an empty string if the code is not recognised.
//
// X is the index (staged) status; Y is the worktree (unstaged) status.
func porcelainToStatus(xy string) FileStatus {
	switch xy {
	case "M ", " M", "MM":
		return FileStatusModified
	case "A ":
		return FileStatusStaged
	case "AM":
		return FileStatusStaged
	case "D ", " D":
		return FileStatusDeleted
	case "R ", "RM":
		return FileStatusRenamed
	case "??":
		return FileStatusUntracked
	case "!!":
		return FileStatusIgnored
	default:
		// Handle any remaining index-modified variants not explicitly listed.
		if len(xy) == 2 {
			x, y := xy[0], xy[1]
			if x == 'A' && y == ' ' {
				return FileStatusStaged
			}
			if x == 'M' || y == 'M' {
				return FileStatusModified
			}
			if x == 'D' || y == 'D' {
				return FileStatusDeleted
			}
		}
		return ""
	}
}
