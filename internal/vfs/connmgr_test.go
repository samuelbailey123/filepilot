package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemotePath(t *testing.T) {
	tests := []struct {
		input      string
		wantID     string
		wantPath   string
		wantErr    bool
	}{
		{"remote://myconn/home/user", "myconn", "/home/user", false},
		{"remote://myconn/", "myconn", "/", false},
		{"remote://myconn", "myconn", "/", false},
		{"/local/path", "", "", true},
		{"", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			id, rpath, err := ParseRemotePath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Errorf("connID = %q, want %q", id, tc.wantID)
			}
			if rpath != tc.wantPath {
				t.Errorf("remotePath = %q, want %q", rpath, tc.wantPath)
			}
		})
	}
}

func TestBuildRemotePath(t *testing.T) {
	tests := []struct {
		connID string
		rpath  string
		want   string
	}{
		{"myconn", "/home/user", "remote://myconn/home/user"},
		{"myconn", "/", "remote://myconn/"},
		{"myconn", "", "remote://myconn/"},
		{"myconn", "relative", "remote://myconn/relative"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := BuildRemotePath(tc.connID, tc.rpath)
			if got != tc.want {
				t.Errorf("BuildRemotePath(%q, %q) = %q, want %q", tc.connID, tc.rpath, got, tc.want)
			}
		})
	}
}

func TestIsRemotePath(t *testing.T) {
	if !IsRemotePath("remote://test/path") {
		t.Error("expected remote://test/path to be remote")
	}
	if IsRemotePath("/local/path") {
		t.Error("expected /local/path to not be remote")
	}
}

func TestConnManager_SaveAndList(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	cfg := ConnectionConfig{
		ID:       "test-sftp",
		Name:     "Test Server",
		Protocol: "sftp",
		Host:     "example.com",
		Port:     22,
		User:     "admin",
	}

	if err := cm.SaveConnection(cfg); err != nil {
		t.Fatalf("SaveConnection: %v", err)
	}

	conns := cm.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].ID != "test-sftp" {
		t.Errorf("ID = %q, want test-sftp", conns[0].ID)
	}
	if conns[0].Host != "example.com" {
		t.Errorf("Host = %q, want example.com", conns[0].Host)
	}

	// Verify file was written.
	data, err := os.ReadFile(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatalf("read connections file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("connections file is empty")
	}
}

func TestConnManager_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	cfg := ConnectionConfig{ID: "test", Name: "V1", Protocol: "sftp", Host: "a.com"}
	cm.SaveConnection(cfg)

	cfg.Name = "V2"
	cfg.Host = "b.com"
	cm.SaveConnection(cfg)

	conns := cm.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection after update, got %d", len(conns))
	}
	if conns[0].Name != "V2" || conns[0].Host != "b.com" {
		t.Errorf("connection not updated: %+v", conns[0])
	}
}

func TestConnManager_Delete(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	cm.SaveConnection(ConnectionConfig{ID: "a", Name: "A", Protocol: "sftp", Host: "a.com"})
	cm.SaveConnection(ConnectionConfig{ID: "b", Name: "B", Protocol: "ftp", Host: "b.com"})

	if err := cm.DeleteConnection("a"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	conns := cm.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection after delete, got %d", len(conns))
	}
	if conns[0].ID != "b" {
		t.Errorf("remaining connection ID = %q, want b", conns[0].ID)
	}
}

func TestConnManager_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	cm.SaveConnection(ConnectionConfig{ID: "persist", Name: "Persist Test", Protocol: "webdav", Host: "dav.example.com"})

	// Create a new ConnManager that reads from the same directory.
	cm2 := NewConnManager(dir)
	conns := cm2.ListConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection after reload, got %d", len(conns))
	}
	if conns[0].ID != "persist" {
		t.Errorf("reloaded connection ID = %q, want persist", conns[0].ID)
	}
}

func TestConnManager_GetConnection(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	cm.SaveConnection(ConnectionConfig{ID: "find-me", Name: "Find Me", Protocol: "sftp", Host: "x.com"})

	cfg, ok := cm.GetConnection("find-me")
	if !ok {
		t.Fatal("GetConnection returned false for existing connection")
	}
	if cfg.Host != "x.com" {
		t.Errorf("Host = %q, want x.com", cfg.Host)
	}

	_, ok = cm.GetConnection("nonexistent")
	if ok {
		t.Fatal("GetConnection returned true for nonexistent connection")
	}
}

func TestConnManager_SaveConnectionValidation(t *testing.T) {
	dir := t.TempDir()
	cm := NewConnManager(dir)

	err := cm.SaveConnection(ConnectionConfig{ID: "", Name: "No ID"})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}
