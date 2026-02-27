package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"filepilot/internal/fileops"
	"filepilot/internal/index"
	"filepilot/internal/preview"
	"filepilot/internal/search"
	"filepilot/internal/server"
	fpsync "filepilot/internal/sync"
	"filepilot/internal/vfs"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the frontend.
type App struct {
	ctx         context.Context
	idx         *index.Index
	scanner     *index.Scanner
	watcher     *index.Watcher
	search      *search.Engine
	ops         *fileops.Ops
	assetServer *server.AssetServer
	localFS     vfs.FileSystem
	connMgr     *vfs.ConnManager
	tempFiles   *vfs.TempFileManager
	dbPath      string
	homeDir     string
}

// NewApp creates a new App instance.
func NewApp() *App {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/Users"
	}
	dbPath := filepath.Join(home, ".config", "filepilot", "index.db")

	configDir := filepath.Join(home, ".config", "filepilot")
	tempDir := filepath.Join(os.TempDir(), "filepilot-remote")
	tempMgr, err := vfs.NewTempFileManager(tempDir)
	if err != nil {
		log.Printf("failed to create temp file manager: %v", err)
	}

	return &App{
		ops:       fileops.NewOps(),
		localFS:   vfs.NewLocal(),
		connMgr:   vfs.NewConnManager(configDir),
		tempFiles: tempMgr,
		dbPath:    dbPath,
		homeDir:   home,
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	idx, err := index.NewIndex(a.dbPath)
	if err != nil {
		log.Printf("failed to open index: %v", err)
		return
	}
	a.idx = idx

	// Search uses the same Index DB connection — no separate connection needed.
	a.search = search.NewEngine(idx.DB())

	a.scanner = index.NewScanner(a.idx)

	// Start background scan.
	go a.backgroundScan()

	// Start filesystem watcher.
	w, err := index.NewWatcher(a.idx)
	if err != nil {
		log.Printf("failed to create watcher: %v", err)
		return
	}
	a.watcher = w
	a.watcher.SetOnChange(func(path string) {
		runtime.EventsEmit(a.ctx, "fs:changed", path)
	})

	a.watcher.Watch(a.homeDir, "/Volumes")

	// Start embedded asset server for serving local files to WKWebView.
	as, err := server.NewAssetServer()
	if err != nil {
		log.Printf("failed to start asset server: %v", err)
	} else {
		a.assetServer = as
	}
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.tempFiles != nil {
		a.tempFiles.Cleanup()
	}
	if a.connMgr != nil {
		a.connMgr.DisconnectAll()
	}
	if a.scanner != nil {
		a.scanner.Stop()
	}
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.assetServer != nil {
		a.assetServer.Stop()
	}
	if a.idx != nil {
		a.idx.Close()
	}
}

// backgroundScan runs the initial filesystem scan in the background.
func (a *App) backgroundScan() {
	if a.idx == nil {
		return
	}

	// Check if we have any indexed files; skip full scan if already populated.
	files, dirs, _, _ := a.idx.Stats()
	if files+dirs > 1000 {
		log.Printf("index already has %d files and %d dirs, skipping full scan", files, dirs)
		runtime.EventsEmit(a.ctx, "scan:complete", map[string]int64{"files": files, "dirs": dirs})
		return
	}

	runtime.EventsEmit(a.ctx, "scan:started", nil)

	roots := []string{a.homeDir}
	if entries, err := os.ReadDir("/Volumes"); err == nil {
		for _, e := range entries {
			roots = append(roots, filepath.Join("/Volumes", e.Name()))
		}
	}

	if err := a.scanner.Scan(roots...); err != nil {
		log.Printf("scan error: %v", err)
		runtime.EventsEmit(a.ctx, "scan:error", err.Error())
		return
	}

	p := a.scanner.Progress()
	runtime.EventsEmit(a.ctx, "scan:complete", map[string]int64{
		"scanned": p.Scanned,
		"indexed": p.Indexed,
	})
}

// --- Bound methods (called from frontend) ---

// GetHomeDir returns the user's home directory path.
func (a *App) GetHomeDir() string {
	return a.homeDir
}

// ListDir returns directory contents for the given path.
// Routes remote:// paths through the connection manager VFS.
func (a *App) ListDir(dirPath string) ([]index.FileEntry, error) {
	if dirPath == "" {
		dirPath = a.homeDir
	}

	// Route remote paths through VFS connection manager.
	if vfs.IsRemotePath(dirPath) {
		return a.listDirRemote(dirPath)
	}

	// Always read from filesystem directly for reliable, up-to-date results.
	return a.listDirDirect(dirPath)
}

// listDirRemote reads a remote directory via the connection manager.
func (a *App) listDirRemote(dirPath string) ([]index.FileEntry, error) {
	connID, remotePath, err := vfs.ParseRemotePath(dirPath)
	if err != nil {
		return nil, err
	}

	fs, ok := a.connMgr.GetActive(connID)
	if !ok {
		return nil, fmt.Errorf("connection %q is not active", connID)
	}

	vfsEntries, err := fs.List(remotePath)
	if err != nil {
		return nil, err
	}

	entries := make([]index.FileEntry, 0, len(vfsEntries))
	for _, ve := range vfsEntries {
		entries = append(entries, index.FileEntry{
			Path:        vfs.BuildRemotePath(connID, ve.Path),
			Name:        ve.Name,
			ParentPath:  dirPath,
			Extension:   strings.TrimPrefix(filepath.Ext(ve.Name), "."),
			Size:        ve.Size,
			IsDir:       ve.IsDir,
			ModTime:     ve.ModTime.Unix(),
			Permissions: ve.Permissions,
			Hidden:      strings.HasPrefix(ve.Name, "."),
		})
	}
	return entries, nil
}

// listDirDirect reads the filesystem via the VFS abstraction.
func (a *App) listDirDirect(dirPath string) ([]index.FileEntry, error) {
	vfsEntries, err := a.localFS.List(dirPath)
	if err != nil {
		return nil, err
	}

	entries := make([]index.FileEntry, 0, len(vfsEntries))
	for _, ve := range vfsEntries {
		entries = append(entries, index.FileEntry{
			Path:          ve.Path,
			Name:          ve.Name,
			ParentPath:    dirPath,
			Extension:     strings.TrimPrefix(filepath.Ext(ve.Name), "."),
			Size:          ve.Size,
			IsDir:         ve.IsDir,
			ModTime:       ve.ModTime.Unix(),
			Permissions:   ve.Permissions,
			Hidden:        strings.HasPrefix(ve.Name, "."),
			IsSymlink:     ve.IsSymlink,
			SymlinkTarget: ve.SymlinkTarget,
		})
	}
	return entries, nil
}

// GetEntry returns a single file entry.
func (a *App) GetEntry(path string) (*index.FileEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	return &index.FileEntry{
		Path:        path,
		Name:        name,
		ParentPath:  filepath.Dir(path),
		Extension:   strings.TrimPrefix(filepath.Ext(name), "."),
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		ModTime:     info.ModTime().Unix(),
		Permissions: int(info.Mode().Perm()),
		Hidden:      strings.HasPrefix(name, "."),
	}, nil
}

// Search performs a full-text search across the file index.
func (a *App) Search(query string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.Search(query, limit)
}

// SearchInDir performs a search scoped to a directory subtree.
func (a *App) SearchInDir(query string, dirPath string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.SearchInDir(query, dirPath, limit)
}

// SubstringSearch performs a substring (LIKE) search across the file index.
func (a *App) SubstringSearch(query string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.SubstringSearch(query, limit)
}

// SubstringSearchInDir performs a substring search scoped to a directory.
func (a *App) SubstringSearchInDir(query string, dirPath string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.SubstringSearchInDir(query, dirPath, limit)
}

// RegexSearch performs a regex-based search across the file index.
func (a *App) RegexSearch(pattern string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.RegexSearch(pattern, limit)
}

// RegexSearchInDir performs a regex search scoped to a directory.
func (a *App) RegexSearchInDir(pattern string, dirPath string, limit int) ([]search.Result, error) {
	if a.search == nil {
		return nil, nil
	}
	return a.search.RegexSearchInDir(pattern, dirPath, limit)
}

// ServeFileURL returns a localhost URL for accessing a local file via the asset server.
// Used by the frontend for PDF rendering and other content that requires HTTP access.
func (a *App) ServeFileURL(path string) string {
	if a.assetServer == nil {
		return ""
	}
	return a.assetServer.FileURL(path)
}

// GetPreview generates a file preview.
func (a *App) GetPreview(path string) (*preview.FilePreview, error) {
	return preview.Generate(path)
}

// RequestICloudDownload triggers an iCloud download for a placeholder file
// using macOS brctl. Returns nil on success or if not applicable.
func (a *App) RequestICloudDownload(path string) error {
	return fileops.RequestICloudDownload(path)
}

// MoveFile moves a file or directory.
func (a *App) MoveFile(src, dst string) error {
	return a.ops.Move(src, dst)
}

// CopyFile copies a file or directory.
func (a *App) CopyFile(src, dst string) error {
	return a.ops.Copy(src, dst)
}

// RenameFile renames a file or directory.
func (a *App) RenameFile(path, newName string) (string, error) {
	return a.ops.Rename(path, newName)
}

// DeleteFile moves a file to the macOS Trash.
func (a *App) DeleteFile(path string) error {
	return a.ops.MoveToTrash(path)
}

// Undo reverses the last file operation.
func (a *App) Undo() (string, error) {
	entry, err := a.ops.Undo()
	if err != nil {
		return "", err
	}
	return entry.From, nil
}

// UndoCount returns the number of undo-able operations.
func (a *App) UndoCount() int {
	return a.ops.UndoCount()
}

// OpenFile opens a file with the default application.
func (a *App) OpenFile(path string) error {
	return fileops.OpenWithDefault(path)
}

// RevealInFinder reveals a file in Finder.
func (a *App) RevealInFinder(path string) error {
	return fileops.RevealInFinder(path)
}

// CreateNewDir creates a new directory.
func (a *App) CreateNewDir(path string) error {
	return a.ops.CreateDir(path)
}

// CreateNewFile creates a new empty file.
func (a *App) CreateNewFile(path string) error {
	return a.ops.CreateFile(path)
}

// GetFavorites returns the user's favorite locations.
func (a *App) GetFavorites() ([]index.Favorite, error) {
	if a.idx == nil {
		return nil, nil
	}
	return a.idx.ListFavorites()
}

// AddFavorite adds a path to favorites.
func (a *App) AddFavorite(path, label string) error {
	if a.idx == nil {
		return nil
	}
	return a.idx.AddFavorite(path, label)
}

// RemoveFavorite removes a path from favorites.
func (a *App) RemoveFavorite(path string) error {
	if a.idx == nil {
		return nil
	}
	return a.idx.RemoveFavorite(path)
}

// GetVolumes returns mounted volumes.
func (a *App) GetVolumes() ([]map[string]string, error) {
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return nil, err
	}

	var volumes []map[string]string
	for _, e := range entries {
		volumes = append(volumes, map[string]string{
			"name": e.Name(),
			"path": filepath.Join("/Volumes", e.Name()),
		})
	}
	return volumes, nil
}

// EjectVolume safely ejects a mounted volume using diskutil.
// Refuses to eject the root volume or the boot volume.
func (a *App) EjectVolume(mountPath string) error {
	if mountPath == "" {
		return fmt.Errorf("mount path is required")
	}

	// Refuse to eject root or boot volume.
	cleanPath := filepath.Clean(mountPath)
	if cleanPath == "/" || cleanPath == "/System/Volumes/Data" {
		return fmt.Errorf("cannot eject the boot volume")
	}

	// Only allow ejecting volumes under /Volumes.
	if !strings.HasPrefix(cleanPath, "/Volumes/") {
		return fmt.Errorf("can only eject volumes mounted under /Volumes")
	}

	// Verify the volume exists and is mounted.
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return fmt.Errorf("volume not found: %s", cleanPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "diskutil", "eject", cleanPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eject failed: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// GetDiskUsage returns disk space information for the given path's volume.
func (a *App) GetDiskUsage(path string) (*fileops.DiskUsage, error) {
	return fileops.GetDiskUsage(path)
}

// GetFolderSize returns the total size of a directory in bytes.
func (a *App) GetFolderSize(path string) (int64, error) {
	return fileops.GetFolderSize(path)
}

// GetScanProgress returns the current scan progress.
func (a *App) GetScanProgress() index.ScanProgress {
	if a.scanner == nil {
		return index.ScanProgress{}
	}
	return a.scanner.Progress()
}

// GetStats returns index statistics.
func (a *App) GetStats() map[string]int64 {
	if a.idx == nil {
		return nil
	}
	files, dirs, dbSize, _ := a.idx.Stats()
	return map[string]int64{
		"files":  files,
		"dirs":   dirs,
		"dbSize": dbSize,
	}
}

// FormatSize formats bytes into a human-readable string.
func (a *App) FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// SaveKeymap persists the user's keymap overrides to disk.
// The keymap is stored as JSON in ~/.config/filepilot/keymap.json.
func (a *App) SaveKeymap(keymapJSON string) error {
	dir := filepath.Join(a.homeDir, ".config", "filepilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	path := filepath.Join(dir, "keymap.json")
	return os.WriteFile(path, []byte(keymapJSON), 0644)
}

// LoadKeymap reads the user's keymap overrides from disk.
// Returns an empty string if no keymap file exists.
func (a *App) LoadKeymap() (string, error) {
	path := filepath.Join(a.homeDir, ".config", "filepilot", "keymap.json")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// BatchRename performs a batch rename operation on multiple files.
// Each op contains a source path and new name. All ops are validated before
// any renames are executed to ensure atomicity.
func (a *App) BatchRename(ops []RenameOp) error {
	if len(ops) == 0 {
		return nil
	}

	// Validate all operations first.
	for _, op := range ops {
		if op.Path == "" || op.NewName == "" {
			return fmt.Errorf("invalid rename: path and newName are required")
		}
		if strings.Contains(op.NewName, "/") || strings.Contains(op.NewName, string(filepath.Separator)) {
			return fmt.Errorf("invalid rename: newName cannot contain path separators")
		}
		if _, err := os.Stat(op.Path); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", op.Path)
		}
		// Check that the target doesn't already exist.
		newPath := filepath.Join(filepath.Dir(op.Path), op.NewName)
		if newPath != op.Path {
			if _, err := os.Stat(newPath); err == nil {
				return fmt.Errorf("target already exists: %s", newPath)
			}
		}
	}

	// Execute all renames.
	for i, op := range ops {
		newPath := filepath.Join(filepath.Dir(op.Path), op.NewName)
		if newPath == op.Path {
			continue
		}
		if err := os.Rename(op.Path, newPath); err != nil {
			return fmt.Errorf("rename %d failed (%s -> %s): %w", i, op.Path, op.NewName, err)
		}
	}

	return nil
}

// RenameOp represents a single file rename operation.
type RenameOp struct {
	Path    string `json:"path"`
	NewName string `json:"newName"`
}

// SaveWorkspace persists a named workspace state to disk.
func (a *App) SaveWorkspace(name string, stateJSON string) error {
	dir := filepath.Join(a.homeDir, ".config", "filepilot", "workspaces")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create workspaces dir: %w", err)
	}
	// Sanitize name for filesystem use.
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	path := filepath.Join(dir, safeName+".json")
	return os.WriteFile(path, []byte(stateJSON), 0644)
}

// LoadWorkspace reads a named workspace from disk.
func (a *App) LoadWorkspace(name string) (string, error) {
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	path := filepath.Join(a.homeDir, ".config", "filepilot", "workspaces", safeName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ListWorkspaces returns the names of all saved workspaces.
func (a *App) ListWorkspaces() ([]string, error) {
	dir := filepath.Join(a.homeDir, ".config", "filepilot", "workspaces")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}

// DeleteWorkspace removes a saved workspace.
func (a *App) DeleteWorkspace(name string) error {
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	path := filepath.Join(a.homeDir, ".config", "filepilot", "workspaces", safeName+".json")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GetGitStatus returns git file statuses for a directory, if it's inside a git repo.
// Returns nil if the directory is not a git repository.
func (a *App) GetGitStatus(dirPath string) (map[string]string, error) {
	// Check if git is available.
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil
	}

	// Check if this is a git repo.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dirPath, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // Not a git repo.
	}
	repoRoot := strings.TrimSpace(string(out))

	// Get status.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	cmd2 := exec.CommandContext(ctx2, "git", "-C", repoRoot, "status", "--porcelain", "-uall")
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return nil, nil
	}

	result := make(map[string]string)
	lines := strings.Split(string(out2), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		xy := line[0:2]
		path := strings.TrimSpace(line[3:])
		// Handle renamed files (old -> new).
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		absPath := filepath.Join(repoRoot, path)

		var status string
		switch {
		case xy == "??":
			status = "untracked"
		case xy == "!!":
			status = "ignored"
		case xy[0] == 'A':
			status = "staged"
		case xy[0] == 'D' || xy[1] == 'D':
			status = "deleted"
		case xy[0] == 'R':
			status = "renamed"
		case xy[0] == 'M' || xy[1] == 'M':
			status = "modified"
		default:
			status = "modified"
		}
		result[absPath] = status
	}

	return result, nil
}

// GitRepoRoot returns the git repo root for a directory, or empty string if not a repo.
func (a *App) GitRepoRoot(dirPath string) string {
	if _, err := exec.LookPath("git"); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dirPath, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --- Network Connection Management ---

// ConnectionInfo is the frontend-facing connection representation.
type ConnectionInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	BasePath string `json:"basePath"`
	Active   bool   `json:"active"`
}

// ListSavedConnections returns all saved connection configurations.
func (a *App) ListSavedConnections() []ConnectionInfo {
	conns := a.connMgr.ListConnections()
	result := make([]ConnectionInfo, len(conns))
	for i, c := range conns {
		result[i] = ConnectionInfo{
			ID:       c.ID,
			Name:     c.Name,
			Protocol: c.Protocol,
			Host:     c.Host,
			Port:     c.Port,
			User:     c.User,
			BasePath: c.BasePath,
			Active:   a.connMgr.IsActive(c.ID),
		}
	}
	return result
}

// SaveConnection saves a connection configuration.
func (a *App) SaveConnection(connJSON string) error {
	var cfg vfs.ConnectionConfig
	if err := json.Unmarshal([]byte(connJSON), &cfg); err != nil {
		return fmt.Errorf("invalid connection config: %w", err)
	}
	return a.connMgr.SaveConnection(cfg)
}

// DeleteConnection removes a saved connection and disconnects it.
func (a *App) DeleteConnectionByID(connID string) error {
	return a.connMgr.DeleteConnection(connID)
}

// ConnectSFTP establishes an SFTP connection using a saved connection config.
// The password is passed directly (loaded from Keychain by frontend or entered by user).
func (a *App) ConnectSFTP(connID string, password string) (string, error) {
	cfg, ok := a.connMgr.GetConnection(connID)
	if !ok {
		return "", fmt.Errorf("connection %q not found", connID)
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	sftpFS, err := vfs.NewSFTP(vfs.SFTPConfig{
		Host:           cfg.Host,
		Port:           port,
		User:           cfg.User,
		Password:       password,
		PrivateKeyPath: cfg.PrivateKeyPath,
	})
	if err != nil {
		return "", fmt.Errorf("SFTP connect failed: %w", err)
	}

	a.connMgr.SetActive(connID, sftpFS)

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = "/"
	}
	return vfs.BuildRemotePath(connID, basePath), nil
}

// ConnectFTP establishes an FTP connection using a saved connection config.
func (a *App) ConnectFTP(connID string, password string) (string, error) {
	cfg, ok := a.connMgr.GetConnection(connID)
	if !ok {
		return "", fmt.Errorf("connection %q not found", connID)
	}

	port := cfg.Port
	if port == 0 {
		port = 21
	}

	ftpFS, err := vfs.NewFTP(vfs.FTPConfig{
		Host:     cfg.Host,
		Port:     port,
		User:     cfg.User,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("FTP connect failed: %w", err)
	}

	a.connMgr.SetActive(connID, ftpFS)

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = "/"
	}
	return vfs.BuildRemotePath(connID, basePath), nil
}

// ConnectWebDAV establishes a WebDAV connection using a saved connection config.
func (a *App) ConnectWebDAV(connID string, password string) (string, error) {
	cfg, ok := a.connMgr.GetConnection(connID)
	if !ok {
		return "", fmt.Errorf("connection %q not found", connID)
	}

	scheme := "https"
	port := cfg.Port
	if port == 80 {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s", scheme, cfg.Host)
	if port > 0 && port != 80 && port != 443 {
		url = fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, port)
	}

	webdavFS, err := vfs.NewWebDAV(vfs.WebDAVConfig{
		URL:      url,
		User:     cfg.User,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("WebDAV connect failed: %w", err)
	}

	a.connMgr.SetActive(connID, webdavFS)

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = "/"
	}
	return vfs.BuildRemotePath(connID, basePath), nil
}

// ConnectS3 establishes an S3 connection using a saved connection config.
// The secretKey is passed directly (loaded from Keychain by frontend or entered by user).
func (a *App) ConnectS3(connID string, secretKey string) (string, error) {
	cfg, ok := a.connMgr.GetConnection(connID)
	if !ok {
		return "", fmt.Errorf("connection %q not found", connID)
	}

	s3FS, err := vfs.NewS3(vfs.S3Config{
		Region:         cfg.Host, // Host field stores the region for S3.
		Bucket:         cfg.BasePath,
		AccessKeyID:    cfg.User,
		SecretAccessKey: secretKey,
	})
	if err != nil {
		return "", fmt.Errorf("S3 connect failed: %w", err)
	}

	a.connMgr.SetActive(connID, s3FS)
	return vfs.BuildRemotePath(connID, "/"), nil
}

// DisconnectRemote disconnects an active remote connection.
func (a *App) DisconnectRemote(connID string) error {
	return a.connMgr.Disconnect(connID)
}

// ServeRemoteFileURL downloads a remote file to a local temp directory
// and returns a localhost URL for the asset server to serve it.
// Used for previewing remote files (images, PDFs, etc.) in the frontend.
func (a *App) ServeRemoteFileURL(remotePath string) (string, error) {
	if !vfs.IsRemotePath(remotePath) {
		return a.ServeFileURL(remotePath), nil
	}

	connID, remPath, err := vfs.ParseRemotePath(remotePath)
	if err != nil {
		return "", err
	}

	fs, ok := a.connMgr.GetActive(connID)
	if !ok {
		return "", fmt.Errorf("connection %q is not active", connID)
	}

	if a.tempFiles == nil {
		return "", fmt.Errorf("temp file manager not available")
	}

	localPath, err := a.tempFiles.Download(fs, connID, remPath)
	if err != nil {
		return "", err
	}

	if a.assetServer == nil {
		return "", fmt.Errorf("asset server not available")
	}

	return a.assetServer.FileURL(localPath), nil
}

// UploadRemoteFile pushes a locally-edited temp file back to the remote server.
func (a *App) UploadRemoteFile(remotePath string) error {
	if !vfs.IsRemotePath(remotePath) {
		return fmt.Errorf("not a remote path: %s", remotePath)
	}

	connID, remPath, err := vfs.ParseRemotePath(remotePath)
	if err != nil {
		return err
	}

	fs, ok := a.connMgr.GetActive(connID)
	if !ok {
		return fmt.Errorf("connection %q is not active", connID)
	}

	if a.tempFiles == nil {
		return fmt.Errorf("temp file manager not available")
	}

	return a.tempFiles.Upload(fs, connID, remPath)
}

// --- Folder Sync ---

// SyncDiffEntry is the frontend-facing representation of a diff entry.
type SyncDiffEntry struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	IsDir     bool   `json:"isDir"`
	LeftSize  int64  `json:"leftSize"`
	RightSize int64  `json:"rightSize"`
	LeftMod   int64  `json:"leftMod"`
	RightMod  int64  `json:"rightMod"`
}

// CompareFolders compares two directories and returns a list of differences.
// Paths can be local or remote (remote://{connID}/path).
func (a *App) CompareFolders(leftPath string, rightPath string, ignoreHidden bool) ([]SyncDiffEntry, error) {
	leftFS, leftResolvedPath, err := a.resolveFS(leftPath)
	if err != nil {
		return nil, fmt.Errorf("resolve left: %w", err)
	}

	rightFS, rightResolvedPath, err := a.resolveFS(rightPath)
	if err != nil {
		return nil, fmt.Errorf("resolve right: %w", err)
	}

	diff, err := fpsync.Compare(leftFS, leftResolvedPath, rightFS, rightResolvedPath, fpsync.DiffOptions{
		IgnoreHidden: ignoreHidden,
	})
	if err != nil {
		return nil, err
	}

	result := make([]SyncDiffEntry, len(diff))
	for i, d := range diff {
		entry := SyncDiffEntry{
			Path:   d.Path,
			Name:   d.Name,
			Status: string(d.Status),
			IsDir:  d.IsDir,
		}
		if d.LeftEntry != nil {
			entry.LeftSize = d.LeftEntry.Size
			entry.LeftMod = d.LeftEntry.ModTime.Unix()
		}
		if d.RightEntry != nil {
			entry.RightSize = d.RightEntry.Size
			entry.RightMod = d.RightEntry.ModTime.Unix()
		}
		result[i] = entry
	}
	return result, nil
}

// SyncFolders executes a folder sync operation.
// Direction must be "left_to_right", "right_to_left", or "bidirectional".
func (a *App) SyncFolders(leftPath, rightPath, direction string, ignoreHidden bool) (int, error) {
	leftFS, leftResolvedPath, err := a.resolveFS(leftPath)
	if err != nil {
		return 0, fmt.Errorf("resolve left: %w", err)
	}

	rightFS, rightResolvedPath, err := a.resolveFS(rightPath)
	if err != nil {
		return 0, fmt.Errorf("resolve right: %w", err)
	}

	diff, err := fpsync.Compare(leftFS, leftResolvedPath, rightFS, rightResolvedPath, fpsync.DiffOptions{
		IgnoreHidden: ignoreHidden,
	})
	if err != nil {
		return 0, err
	}

	syncDir := fpsync.SyncDirection(direction)
	plan := fpsync.PlanSync(diff, syncDir, leftFS, leftResolvedPath, rightFS, rightResolvedPath)

	// Execute operations.
	executed := 0
	for _, op := range plan {
		switch op.Type {
		case fpsync.OpCopy:
			reader, err := op.SourceFS.Read(op.SourcePath)
			if err != nil {
				return executed, fmt.Errorf("read %s: %w", op.SourcePath, err)
			}
			err = op.DestFS.Write(op.DestPath, reader)
			reader.Close()
			if err != nil {
				return executed, fmt.Errorf("write %s: %w", op.DestPath, err)
			}
		case fpsync.OpDelete:
			if err := op.DestFS.Remove(op.DestPath); err != nil {
				return executed, fmt.Errorf("delete %s: %w", op.DestPath, err)
			}
		}
		executed++
	}

	return executed, nil
}

// resolveFS returns the appropriate FileSystem and resolved path for a given path.
// Supports both local paths and remote:// paths.
func (a *App) resolveFS(p string) (vfs.FileSystem, string, error) {
	if vfs.IsRemotePath(p) {
		connID, remotePath, err := vfs.ParseRemotePath(p)
		if err != nil {
			return nil, "", err
		}
		fs, ok := a.connMgr.GetActive(connID)
		if !ok {
			return nil, "", fmt.Errorf("connection %q is not active", connID)
		}
		return fs, remotePath, nil
	}
	return a.localFS, p, nil
}

// --- Symlink Support ---

// CreateSymlink creates a symbolic link at linkPath pointing to target.
func (a *App) CreateSymlink(target, linkPath string) error {
	return os.Symlink(target, linkPath)
}

// --- Path Completion ---

// CompletePath returns up to 20 directory paths that match the given partial
// path. It splits the partial into parent directory and a basename prefix,
// lists the parent, and returns all directory entries whose names begin with
// the prefix — with a trailing slash appended to each result.
func (a *App) CompletePath(partial string) ([]string, error) {
	if partial == "" {
		return nil, nil
	}

	// Determine the parent directory and the basename prefix to match against.
	var parent, prefix string
	if strings.HasSuffix(partial, "/") {
		// The partial path already ends with a separator; list that directory.
		parent = partial
		prefix = ""
	} else {
		parent = filepath.Dir(partial)
		prefix = filepath.Base(partial)
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", parent, err)
	}

	const maxResults = 20
	var results []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(prefix)) {
			continue
		}
		results = append(results, filepath.Join(parent, e.Name())+"/")
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

// --- Archive Creation ---

// CompressFiles creates a ZIP archive at outputPath containing all entries
// listed in paths. Both files and directories (recursively) are supported.
// Returns an error if paths is empty or if outputPath already exists.
func (a *App) CompressFiles(paths []string, outputPath string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return fmt.Errorf("output already exists: %s", outputPath)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			return fmt.Errorf("stat %s: %w", p, err)
		}
		base := filepath.Base(p)
		if info.IsDir() {
			if err := addToZip(w, p, base); err != nil {
				return err
			}
		} else {
			if err := addFileToZip(w, p, base); err != nil {
				return err
			}
		}
	}
	return nil
}

// addToZip recursively adds the directory at dirPath into the zip writer,
// using prefix as the entry name root inside the archive.
func addToZip(w *zip.Writer, dirPath, prefix string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", dirPath, err)
	}
	for _, e := range entries {
		fullPath := filepath.Join(dirPath, e.Name())
		entryName := prefix + "/" + e.Name()
		if e.IsDir() {
			if err := addToZip(w, fullPath, entryName); err != nil {
				return err
			}
		} else {
			if err := addFileToZip(w, fullPath, entryName); err != nil {
				return err
			}
		}
	}
	return nil
}

// addFileToZip writes a single file into the zip writer under entryName.
func addFileToZip(w *zip.Writer, filePath, entryName string) error {
	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = entryName
	header.Method = zip.Deflate

	dst, err := w.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", entryName, err)
	}
	_, err = io.Copy(dst, src)
	return err
}

// --- Archive Extraction ---

// ExtractArchive extracts an archive file into destDir and returns the number
// of entries extracted. Supported formats: .zip, .tar, .tar.gz, .tgz, .tar.bz2.
// ZIP-slip protection is enforced: any entry whose resolved path falls outside
// destDir causes an immediate error.
func (a *App) ExtractArchive(archivePath, destDir string) (int, error) {
	name := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(name, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return extractTar(archivePath, destDir, "gz")
	case strings.HasSuffix(name, ".tar.bz2"):
		return extractTar(archivePath, destDir, "bz2")
	case strings.HasSuffix(name, ".tar"):
		return extractTar(archivePath, destDir, "")
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", filepath.Ext(archivePath))
	}
}

// extractZip extracts a ZIP archive into destDir and returns the entry count.
func extractZip(archivePath, destDir string) (int, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	count := 0

	for _, f := range r.File {
		target, err := safeJoin(destDir, cleanDest, f.Name)
		if err != nil {
			return count, err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return count, fmt.Errorf("mkdir %s: %w", target, err)
			}
			count++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return count, fmt.Errorf("mkdir parent: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return count, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return count, fmt.Errorf("create %s: %w", target, err)
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return count, fmt.Errorf("write %s: %w", target, copyErr)
		}
		count++
	}
	return count, nil
}

// extractTar extracts a tar archive (optionally compressed) into destDir and
// returns the entry count. compression must be "", "gz", or "bz2".
func extractTar(archivePath, destDir, compression string) (int, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return 0, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	var tr *tar.Reader
	switch compression {
	case "gz":
		gr, err := gzip.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		tr = tar.NewReader(gr)
	case "bz2":
		tr = tar.NewReader(bzip2.NewReader(f))
	default:
		tr = tar.NewReader(f)
	}

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	count := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("read tar entry: %w", err)
		}

		target, err := safeJoin(destDir, cleanDest, header.Name)
		if err != nil {
			return count, err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return count, fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return count, fmt.Errorf("mkdir parent: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return count, fmt.Errorf("create %s: %w", target, err)
			}
			_, copyErr := io.Copy(out, tr)
			out.Close()
			if copyErr != nil {
				return count, fmt.Errorf("write %s: %w", target, copyErr)
			}
		}
		count++
	}
	return count, nil
}

// safeJoin resolves name relative to destDir and verifies the result stays
// within cleanDest (ZIP-slip prevention). Returns an error for any traversal.
func safeJoin(destDir, cleanDest, name string) (string, error) {
	target := filepath.Join(destDir, name)
	// filepath.Clean resolves any .. components.
	if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
		return "", fmt.Errorf("zip-slip: illegal path %q", name)
	}
	return target, nil
}

// --- Terminal Integration ---

// OpenTerminal opens a terminal application at dirPath, trying Warp, then
// iTerm, and finally falling back to the built-in Terminal.app.
func (a *App) OpenTerminal(dirPath string) error {
	if _, err := os.Stat(dirPath); err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	for _, app := range []string{"Warp", "iTerm"} {
		if err := exec.Command("open", "-a", app, dirPath).Run(); err == nil {
			return nil
		}
	}

	if err := exec.Command("open", "-a", "Terminal", dirPath).Run(); err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	return nil
}

// --- Permissions & File Info ---

// FileInfoDetail contains extended metadata for a file or directory,
// including POSIX ownership and access/create/modify timestamps.
type FileInfoDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	IsDir         bool   `json:"isDir"`
	Permissions   int    `json:"permissions"`
	Owner         string `json:"owner"`
	Group         string `json:"group"`
	Created       int64  `json:"created"`
	Modified      int64  `json:"modified"`
	Accessed      int64  `json:"accessed"`
	IsSymlink     bool   `json:"isSymlink"`
	SymlinkTarget string `json:"symlinkTarget"`
}

// GetFileInfo returns extended metadata for the file at path.
// Uses os.Lstat so that symlinks are identified rather than silently followed.
// Owner and group UIDs are resolved via syscall.Stat_t on macOS/Linux.
func (a *App) GetFileInfo(path string) (*FileInfoDetail, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	symlinkTarget := ""
	isDir := info.IsDir()

	if isSymlink {
		target, readErr := os.Readlink(path)
		if readErr == nil {
			symlinkTarget = target
		}
		if targetInfo, statErr := os.Stat(path); statErr == nil {
			isDir = targetInfo.IsDir()
		}
	}

	detail := &FileInfoDetail{
		Path:          path,
		Name:          filepath.Base(path),
		Size:          info.Size(),
		IsDir:         isDir,
		Permissions:   int(info.Mode().Perm()),
		Modified:      info.ModTime().Unix(),
		IsSymlink:     isSymlink,
		SymlinkTarget: symlinkTarget,
	}

	// Populate UID/GID and atime/birthtime via the platform syscall layer.
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		detail.Owner = fmt.Sprintf("%d", stat.Uid)
		detail.Group = fmt.Sprintf("%d", stat.Gid)
		detail.Accessed = stat.Atimespec.Sec
		detail.Created = stat.Birthtimespec.Sec
	}

	return detail, nil
}

// SetPermissions changes the permission bits of the file at path.
// mode is the decimal representation of the POSIX mode (e.g. 0644 = 420).
func (a *App) SetPermissions(path string, mode int) error {
	return os.Chmod(path, os.FileMode(mode))
}

// --- macOS Finder Tags ---

// GetFileTags returns the Finder tags (kMDItemUserTags) attached to a file.
// It calls mdls and parses the plist-style array output.
func (a *App) GetFileTags(path string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "mdls", "-name", "kMDItemUserTags", "-raw", path).Output()
	if err != nil {
		return nil, fmt.Errorf("mdls failed: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	// mdls returns "(null)" when no tags are set.
	if raw == "(null)" || raw == "" {
		return []string{}, nil
	}

	// Output looks like: ( "Work", "Important" )
	raw = strings.TrimPrefix(raw, "(")
	raw = strings.TrimSuffix(raw, ")")
	parts := strings.Split(raw, ",")

	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags, nil
}

// SetFileTags writes Finder tags to the extended attribute
// com.apple.metadata:_kMDItemUserTags using xattr.
// Passing an empty slice clears all tags.
func (a *App) SetFileTags(path string, tags []string) error {
	// Build a minimal binary plist array.
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	sb.WriteString("<plist version=\"1.0\"><array>")
	for _, tag := range tags {
		sb.WriteString("<string>")
		sb.WriteString(tag)
		sb.WriteString("</string>")
	}
	sb.WriteString("</array></plist>")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xattr", "-w", "com.apple.metadata:_kMDItemUserTags", sb.String(), path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xattr failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- File Diff ---

// DiffLine represents a single line in a diff result.
type DiffLine struct {
	LineNum int    `json:"lineNum"`
	Type    string `json:"type"` // "equal", "add", "delete"
	Text    string `json:"text"`
}

// DiffFiles computes a line-level diff between two text files using a simple
// LCS algorithm. No external dependencies are required.
func (a *App) DiffFiles(pathA, pathB string) ([]DiffLine, error) {
	dataA, err := os.ReadFile(pathA)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pathA, err)
	}
	dataB, err := os.ReadFile(pathB)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pathB, err)
	}

	linesA := strings.Split(string(dataA), "\n")
	linesB := strings.Split(string(dataB), "\n")

	return lcsDiff(linesA, linesB), nil
}

// lcsDiff computes a diff between two line slices using a standard LCS
// dynamic-programming approach and returns the annotated result.
func lcsDiff(a, b []string) []DiffLine {
	la, lb := len(a), len(b)

	// Build the LCS length table.
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
	}
	for i := la - 1; i >= 0; i-- {
		for j := lb - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	// Trace back through the table to produce the diff.
	var result []DiffLine
	lineNum := 1
	i, j := 0, 0
	for i < la || j < lb {
		switch {
		case i < la && j < lb && a[i] == b[j]:
			result = append(result, DiffLine{LineNum: lineNum, Type: "equal", Text: a[i]})
			i++
			j++
		case j < lb && (i >= la || dp[i][j+1] >= dp[i+1][j]):
			result = append(result, DiffLine{LineNum: lineNum, Type: "add", Text: b[j]})
			j++
		default:
			result = append(result, DiffLine{LineNum: lineNum, Type: "delete", Text: a[i]})
			i++
		}
		lineNum++
	}
	return result
}

// --- Open With / Context Menu Helpers ---

// ListOpenWithApps returns the names of all .app bundles found in /Applications.
// This gives the frontend a list of applications to show in an "Open With" menu.
func (a *App) ListOpenWithApps(path string) ([]string, error) {
	entries, err := os.ReadDir("/Applications")
	if err != nil {
		return nil, fmt.Errorf("read /Applications: %w", err)
	}

	var apps []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") {
			apps = append(apps, strings.TrimSuffix(e.Name(), ".app"))
		}
	}
	return apps, nil
}

// OpenWithApp opens path using the named application via the macOS open command.
func (a *App) OpenWithApp(path, appName string) error {
	return exec.Command("open", "-a", appName, path).Run()
}

// DuplicateFile copies a file or directory alongside the original with a
// " copy" suffix inserted before the file extension.
// Examples: "file.txt" -> "file copy.txt", "archive.tar.gz" -> "archive copy.tar.gz",
// "folder" -> "folder copy".
// Returns the path of the newly created duplicate.
func (a *App) DuplicateFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	name := info.Name()

	// Split the name into base and extension(s).
	var base, ext string
	if info.IsDir() {
		base = name
		ext = ""
	} else {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}

	// Find a destination name that does not yet exist.
	copyName := base + " copy" + ext
	dest := filepath.Join(dir, copyName)
	counter := 2
	for {
		if _, statErr := os.Lstat(dest); os.IsNotExist(statErr) {
			break
		}
		copyName = fmt.Sprintf("%s copy %d%s", base, counter, ext)
		dest = filepath.Join(dir, copyName)
		counter++
	}

	if info.IsDir() {
		if err := a.ops.Copy(path, dest); err != nil {
			return "", fmt.Errorf("duplicate directory: %w", err)
		}
	} else {
		if err := a.ops.Copy(path, dest); err != nil {
			return "", fmt.Errorf("duplicate file: %w", err)
		}
	}

	return dest, nil
}

// PickDirectory opens a native directory chooser dialog and returns the
// selected path. Returns an empty string if the user cancels.
func (a *App) PickDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose Destination",
	})
}

// Ensure json import is used.
var _ = json.Marshal

// FormatTime returns a human-readable relative time string.
func (a *App) FormatTime(unixTime int64) string {
	t := time.Unix(unixTime, 0)
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 7*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", d)
	default:
		return t.Format("2 Jan 2006")
	}
}

// --- Feature 6.5: Image Thumbnails ---

// GetThumbnail returns a base64-encoded data URI for the image at path.
// Thumbnails are cached on disk to avoid repeated qlmanage invocations.
// size controls the pixel dimension passed to qlmanage; values <= 0 default to 256.
func (a *App) GetThumbnail(path string, size int) (string, error) {
	if size <= 0 {
		size = 256
	}

	cacheDir := filepath.Join(a.homeDir, ".cache", "filepilot", "thumbs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create thumbnail cache dir: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	// Build a cache key from the file path and its modification time so stale
	// entries are naturally invalidated when the source file changes.
	raw := path + fmt.Sprintf("%d", info.ModTime().Unix())
	sum := sha256.Sum256([]byte(raw))
	cacheKey := hex.EncodeToString(sum[:])[:16] + ".jpg"
	cachePath := filepath.Join(cacheDir, cacheKey)

	// Cache hit: serve the stored JPEG thumbnail.
	if cached, readErr := os.ReadFile(cachePath); readErr == nil {
		encoded := base64.StdEncoding.EncodeToString(cached)
		return "data:image/jpeg;base64," + encoded, nil
	}

	// Cache miss: generate the thumbnail via qlmanage (macOS Quick Look).
	tmpDir, err := os.MkdirTemp("", "filepilot-thumb-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("qlmanage", "-t", "-s", fmt.Sprintf("%d", size), "-o", tmpDir, path)
	if runErr := cmd.Run(); runErr != nil {
		return "", fmt.Errorf("qlmanage: %w", runErr)
	}

	// qlmanage writes a file named "<original>.png" into the output dir.
	var thumbPath string
	walkErr := filepath.WalkDir(tmpDir, func(p string, d fs.DirEntry, walkE error) error {
		if walkE != nil {
			return walkE
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".png") {
			thumbPath = p
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("walk temp dir: %w", walkErr)
	}
	if thumbPath == "" {
		return "", fmt.Errorf("qlmanage produced no PNG output for %s", path)
	}

	data, err := os.ReadFile(thumbPath)
	if err != nil {
		return "", fmt.Errorf("read thumbnail: %w", err)
	}

	// Persist to cache so subsequent calls skip qlmanage entirely.
	_ = os.WriteFile(cachePath, data, 0644)

	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:image/png;base64," + encoded, nil
}

// --- Feature 6.8: Content Search (grep-in-files) ---

// GrepResult holds a single line match returned by GrepInDir.
type GrepResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

// GrepInDir performs a case-insensitive full-text search for query across all
// non-binary files under dirPath. It returns up to limit matches (default 100).
// The search respects a 30-second context deadline and skips hidden directories
// (those whose names start with ".") as well as node_modules.
func (a *App) GrepInDir(query, dirPath string, limit int) ([]GrepResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if dirPath == "" {
		return nil, fmt.Errorf("dirPath must not be empty")
	}
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lowerQuery := strings.ToLower(query)
	var results []GrepResult

	walkErr := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we cannot access rather than aborting the whole walk.
			return nil
		}

		// Respect cancellation.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		name := d.Name()

		// Skip hidden directories, node_modules, and .DS_Store entries.
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}

		// Only inspect regular files.
		if !d.Type().IsRegular() {
			return nil
		}

		// Binary detection: read the first 8 KB and check for null bytes.
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer f.Close()

		header := make([]byte, 8192)
		n, _ := f.Read(header)
		for i := 0; i < n; i++ {
			if header[i] == 0 {
				// Binary file — skip without error.
				return nil
			}
		}

		// Seek back to the beginning so the scanner reads the whole file.
		if _, seekErr := f.Seek(0, 0); seekErr != nil {
			return nil
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				snippet := line
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				results = append(results, GrepResult{
					Path:    path,
					Name:    filepath.Base(path),
					Line:    lineNum,
					Snippet: strings.TrimSpace(snippet),
				})
				if len(results) >= limit {
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	if walkErr != nil && walkErr != fs.SkipAll && ctx.Err() == nil {
		return results, fmt.Errorf("walk %s: %w", dirPath, walkErr)
	}

	return results, nil
}
