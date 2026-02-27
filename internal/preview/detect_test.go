package preview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHasBinaryContent verifies detection of text vs binary files.
func TestHasBinaryContent(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Text file: should not be detected as binary.
	textPath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(textPath, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("write text file: %v", err)
	}
	binary, err := hasBinaryContent(textPath)
	if err != nil {
		t.Fatalf("hasBinaryContent(text): %v", err)
	}
	if binary {
		t.Error("expected text file to not be detected as binary")
	}

	// Binary file: should be detected as binary (contains null bytes).
	binPath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(binPath, []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}, 0644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	binary, err = hasBinaryContent(binPath)
	if err != nil {
		t.Fatalf("hasBinaryContent(binary): %v", err)
	}
	if !binary {
		t.Error("expected binary file to be detected as binary")
	}
}

// TestHexDump verifies the hex dump output format.
func TestHexDump(t *testing.T) {
	dir, err := os.MkdirTemp("", "filepilot-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.bin")
	data := []byte("Hello, World!")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	dump, err := hexDump(path)
	if err != nil {
		t.Fatalf("hexDump: %v", err)
	}

	// Should start with offset 00000000.
	if !strings.HasPrefix(dump, "00000000") {
		t.Errorf("expected dump to start with offset, got: %s", dump[:20])
	}

	// Should contain the ASCII representation.
	if !strings.Contains(dump, "Hello, World!") {
		t.Error("expected ASCII column to contain the text")
	}
}
