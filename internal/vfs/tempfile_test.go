package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTempFileManager_DownloadAndUpload(t *testing.T) {
	// Create a source directory with a file.
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(srcFile, []byte("hello remote"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use local VFS as a mock remote filesystem.
	localFS := NewLocal()

	// Create temp file manager.
	tempDir := t.TempDir()
	mgr, err := NewTempFileManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Download the file.
	localPath, err := mgr.Download(localFS, "test-conn", srcFile)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	// Verify the local file exists and has correct content.
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read local file: %v", err)
	}
	if string(data) != "hello remote" {
		t.Errorf("content = %q, want %q", string(data), "hello remote")
	}

	// Download again should return cached path.
	localPath2, err := mgr.Download(localFS, "test-conn", srcFile)
	if err != nil {
		t.Fatalf("Download cached: %v", err)
	}
	if localPath2 != localPath {
		t.Error("expected cached path to match")
	}

	// Modify the local temp file.
	if err := os.WriteFile(localPath, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Upload back to the remote.
	if err := mgr.Upload(localFS, "test-conn", srcFile); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Verify the remote file was updated.
	remoteData, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(remoteData) != "modified" {
		t.Errorf("remote content = %q, want %q", string(remoteData), "modified")
	}
}

func TestTempFileManager_GetLocalPath(t *testing.T) {
	tempDir := t.TempDir()
	mgr, err := NewTempFileManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Non-existent file returns empty.
	got := mgr.GetLocalPath("conn", "/nonexistent")
	if got != "" {
		t.Errorf("expected empty path, got %q", got)
	}
}

func TestTempFileManager_Invalidate(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "file.txt")
	os.WriteFile(srcFile, []byte("data"), 0644)

	tempDir := t.TempDir()
	mgr, err := NewTempFileManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	localFS := NewLocal()
	localPath, _ := mgr.Download(localFS, "conn", srcFile)

	// Invalidate.
	mgr.Invalidate("conn", srcFile)

	// Local file should be removed.
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Error("expected local file to be removed after invalidate")
	}

	// GetLocalPath should return empty.
	if mgr.GetLocalPath("conn", srcFile) != "" {
		t.Error("expected empty path after invalidate")
	}
}

func TestTempFileManager_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	baseDir := filepath.Join(tempDir, "managed")
	mgr, err := NewTempFileManager(baseDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create some content.
	os.MkdirAll(filepath.Join(baseDir, "conn"), 0755)
	os.WriteFile(filepath.Join(baseDir, "conn", "file.txt"), []byte("test"), 0644)

	mgr.Cleanup()

	// Base dir should be gone.
	if _, err := os.Stat(baseDir); !os.IsNotExist(err) {
		t.Error("expected base dir to be removed after cleanup")
	}
}
