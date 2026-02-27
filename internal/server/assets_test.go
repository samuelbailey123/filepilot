package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestAssetServerServesFile verifies the asset server can serve a temp file.
func TestAssetServerServesFile(t *testing.T) {
	srv, err := NewAssetServer()
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	defer srv.Stop()

	// Create a temp file with known content.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello from asset server"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Request the file through the asset server.
	resp, err := http.Get(srv.FileURL(path))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(body) != content {
		t.Errorf("expected %q, got %q", content, string(body))
	}
}

// TestAssetServerRejectsDirectory verifies that directories are rejected.
func TestAssetServerRejectsDirectory(t *testing.T) {
	srv, err := NewAssetServer()
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	defer srv.Stop()

	dir := t.TempDir()
	resp, err := http.Get(srv.FileURL(dir))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for directory, got %d", resp.StatusCode)
	}
}

// TestAssetServerRejectsMissingPath verifies that missing paths return 400.
func TestAssetServerRejectsMissingPath(t *testing.T) {
	srv, err := NewAssetServer()
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	defer srv.Stop()

	resp, err := http.Get(srv.URL() + "/serve")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path, got %d", resp.StatusCode)
	}
}

// TestAssetServerNotFound verifies that non-existent files return 404.
func TestAssetServerNotFound(t *testing.T) {
	srv, err := NewAssetServer()
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	defer srv.Stop()

	resp, err := http.Get(srv.FileURL("/nonexistent/file.txt"))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestAssetServerHealthCheck verifies the health endpoint.
func TestAssetServerHealthCheck(t *testing.T) {
	srv, err := NewAssetServer()
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	defer srv.Stop()

	resp, err := http.Get(srv.URL() + "/healthz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
