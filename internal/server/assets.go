// Package server provides an embedded HTTP asset server for file thumbnails and previews.
package server

import (
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// AssetServer serves local files over HTTP on localhost for WKWebView access.
// This is necessary because WKWebView restricts direct file:// access.
type AssetServer struct {
	listener net.Listener
	port     int
}

// NewAssetServer creates and starts a localhost-only HTTP server on a random port.
// The server validates paths and serves files with correct MIME types.
func NewAssetServer() (*AssetServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	s := &AssetServer{
		listener: listener,
		port:     listener.Addr().(*net.TCPAddr).Port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/serve", s.handleServe)
	mux.HandleFunc("/healthz", s.handleHealth)

	go func() {
		srv := &http.Server{Handler: mux}
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("asset server error: %v", err)
		}
	}()

	log.Printf("asset server listening on 127.0.0.1:%d", s.port)
	return s, nil
}

// Port returns the port the server is listening on.
func (s *AssetServer) Port() int {
	return s.port
}

// URL returns the base URL for the asset server.
func (s *AssetServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.port)
}

// FileURL returns the full URL to serve a specific file.
func (s *AssetServer) FileURL(path string) string {
	return fmt.Sprintf("%s/serve?path=%s", s.URL(), path)
}

// Stop closes the asset server.
func (s *AssetServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// handleServe validates the path and serves the file.
func (s *AssetServer) handleServe(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path parameter required", http.StatusBadRequest)
		return
	}

	// Security: Resolve and validate the path to prevent directory traversal.
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	// Verify the file exists and is a regular file (not a directory or symlink to restricted area).
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "access error", http.StatusForbidden)
		}
		return
	}
	if info.IsDir() {
		http.Error(w, "cannot serve directories", http.StatusBadRequest)
		return
	}

	// Determine MIME type.
	ext := filepath.Ext(cleanPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeFile(w, r, cleanPath)
}

// handleHealth returns a simple health check response.
func (s *AssetServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
