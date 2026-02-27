package fileops

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DiskUsage contains disk space information for a volume.
type DiskUsage struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Used      uint64 `json:"used"`
	UsedPct   int    `json:"usedPct"`
}

// GetDiskUsage returns disk space information for the given path's volume.
func GetDiskUsage(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("statfs %s: %w", path, err)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	pct := 0
	if total > 0 {
		pct = int((used * 100) / total)
	}

	return &DiskUsage{
		Total:   total,
		Free:    free,
		Used:    used,
		UsedPct: pct,
	}, nil
}

// GetFolderSize returns the total size in bytes of a directory using du.
// Uses a 5-second timeout to avoid blocking on very large directories.
func GetFolderSize(path string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "du", "-sk", path).Output()
	if err != nil {
		return 0, fmt.Errorf("du %s: %w", path, err)
	}

	parts := strings.Fields(string(out))
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected du output for %s", path)
	}

	// du -sk outputs size in 1024-byte blocks.
	kb, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse du output: %w", err)
	}

	return kb * 1024, nil
}
