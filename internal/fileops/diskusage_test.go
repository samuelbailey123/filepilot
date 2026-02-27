package fileops

import (
	"os"
	"testing"
)

// TestGetDiskUsage verifies that GetDiskUsage returns sensible values for the root volume.
func TestGetDiskUsage(t *testing.T) {
	usage, err := GetDiskUsage("/")
	if err != nil {
		t.Fatalf("GetDiskUsage(/): %v", err)
	}
	if usage.Total == 0 {
		t.Error("expected non-zero total disk space")
	}
	if usage.Free == 0 {
		t.Error("expected non-zero free disk space")
	}
	if usage.Used == 0 {
		t.Error("expected non-zero used disk space")
	}
	if usage.UsedPct < 0 || usage.UsedPct > 100 {
		t.Errorf("expected UsedPct in [0,100], got %d", usage.UsedPct)
	}
}

// TestGetFolderSize verifies that GetFolderSize returns a non-negative value for a temp directory.
func TestGetFolderSize(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	size, err := GetFolderSize(dir)
	if err != nil {
		t.Fatalf("GetFolderSize(%s): %v", dir, err)
	}
	if size < 0 {
		t.Errorf("expected non-negative folder size, got %d", size)
	}
}

// TestGetFolderSizeNonexistent verifies that GetFolderSize returns an error for a missing path.
func TestGetFolderSizeNonexistent(t *testing.T) {
	_, err := GetFolderSize("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
