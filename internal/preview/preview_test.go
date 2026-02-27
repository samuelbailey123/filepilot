package preview

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateDirectory verifies that Generate returns type "directory" for a directory.
func TestGenerateDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	preview, err := Generate(dir)
	if err != nil {
		t.Fatalf("Generate(dir): %v", err)
	}
	if preview.Type != "directory" {
		t.Errorf("expected type 'directory', got %q", preview.Type)
	}
	if preview.Name != filepath.Base(dir) {
		t.Errorf("expected name %q, got %q", filepath.Base(dir), preview.Name)
	}
}

// TestGenerateTextFile verifies that Generate returns type "code" for a text file.
func TestGenerateTextFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	preview, err := Generate(path)
	if err != nil {
		t.Fatalf("Generate(txt): %v", err)
	}
	if preview.Type != "code" {
		t.Errorf("expected type 'code', got %q", preview.Type)
	}
	if preview.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", preview.Content)
	}
}

// TestGenerateUnknownBinary verifies that Generate returns type "hex" for a binary file.
func TestGenerateUnknownBinary(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "data.unknown")
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	preview, err := Generate(path)
	if err != nil {
		t.Fatalf("Generate(binary): %v", err)
	}
	if preview.Type != "hex" {
		t.Errorf("expected type 'hex', got %q", preview.Type)
	}
}
