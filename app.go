package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"decima-explorer/internal/fileops"
	"decima-explorer/internal/index"
	"decima-explorer/internal/preview"
	"decima-explorer/internal/search"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the frontend.
type App struct {
	ctx     context.Context
	idx     *index.Index
	scanner *index.Scanner
	watcher *index.Watcher
	search  *search.Engine
	ops     *fileops.Ops
	dbPath  string
}

// NewApp creates a new App instance.
func NewApp() *App {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "decima-explorer", "index.db")

	return &App{
		ops:    fileops.NewOps(),
		dbPath: dbPath,
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

	// Get the underlying *sql.DB for search engine via a direct open.
	db, err := sql.Open("sqlite", a.dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&mode=ro")
	if err != nil {
		log.Printf("failed to open search db: %v", err)
	} else {
		a.search = search.NewEngine(db)
	}

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

	homeDir, _ := os.UserHomeDir()
	a.watcher.Watch(homeDir, "/Volumes")
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.scanner != nil {
		a.scanner.Stop()
	}
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.idx != nil {
		a.idx.Close()
	}
}

// backgroundScan runs the initial filesystem scan in the background.
func (a *App) backgroundScan() {
	home, _ := os.UserHomeDir()

	// Check if we have any indexed files; skip full scan if already populated.
	files, dirs, _, _ := a.idx.Stats()
	if files+dirs > 1000 {
		log.Printf("index already has %d files and %d dirs, skipping full scan", files, dirs)
		runtime.EventsEmit(a.ctx, "scan:complete", map[string]int64{"files": files, "dirs": dirs})
		return
	}

	runtime.EventsEmit(a.ctx, "scan:started", nil)

	roots := []string{home}
	// Add /Volumes if it exists.
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

// ListDir returns directory contents for the given path.
func (a *App) ListDir(dirPath string) ([]index.FileEntry, error) {
	if a.idx == nil {
		return a.listDirDirect(dirPath)
	}

	entries, err := a.idx.ListDir(dirPath)
	if err != nil || len(entries) == 0 {
		return a.listDirDirect(dirPath)
	}
	return entries, nil
}

// listDirDirect reads the filesystem directly as a fallback.
func (a *App) listDirDirect(dirPath string) ([]index.FileEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var entries []index.FileEntry
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		name := de.Name()
		entries = append(entries, index.FileEntry{
			Path:        filepath.Join(dirPath, name),
			Name:        name,
			ParentPath:  dirPath,
			Extension:   strings.TrimPrefix(filepath.Ext(name), "."),
			Size:        info.Size(),
			IsDir:       de.IsDir(),
			ModTime:     info.ModTime().Unix(),
			Permissions: int(info.Mode().Perm()),
			Hidden:      strings.HasPrefix(name, "."),
		})
	}
	return entries, nil
}

// GetEntry returns a single file entry.
func (a *App) GetEntry(path string) (*index.FileEntry, error) {
	if a.idx != nil {
		entry, err := a.idx.GetEntry(path)
		if err == nil {
			return entry, nil
		}
	}

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

// GetPreview generates a file preview.
func (a *App) GetPreview(path string) (*preview.FilePreview, error) {
	return preview.Generate(path)
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
