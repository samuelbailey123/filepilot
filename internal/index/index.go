// Package index maintains a SQLite-backed file index with FTS5 full-text search and background scanning.
package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// FileEntry represents a single file or directory in the index.
type FileEntry struct {
	ID            int64  `json:"id"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	ParentPath    string `json:"parentPath"`
	Extension     string `json:"extension"`
	Size          int64  `json:"size"`
	IsDir         bool   `json:"isDir"`
	ModTime       int64  `json:"modTime"`
	CreateTime    int64  `json:"createTime"`
	Permissions   int    `json:"permissions"`
	Hidden        bool   `json:"hidden"`
	IndexedAt     int64  `json:"indexedAt"`
	IsSymlink     bool   `json:"isSymlink"`
	SymlinkTarget string `json:"symlinkTarget"`
}

// Favorite represents a user-bookmarked location.
type Favorite struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Label     string `json:"label"`
	SortOrder int    `json:"sortOrder"`
}

// Index manages the SQLite-backed file index.
type Index struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewIndex creates or opens the SQLite index at the given path.
func NewIndex(dbPath string) (*Index, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Connection pool settings for performance.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	idx := &Index{db: db, path: dbPath}
	if err := idx.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return idx, nil
}

// migrate creates the schema if it doesn't exist.
func (idx *Index) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS files (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			path        TEXT NOT NULL UNIQUE,
			name        TEXT NOT NULL,
			parent_path TEXT NOT NULL,
			extension   TEXT,
			size        INTEGER NOT NULL DEFAULT 0,
			is_dir      INTEGER NOT NULL DEFAULT 0,
			mod_time    INTEGER NOT NULL,
			create_time INTEGER,
			permissions INTEGER,
			hidden      INTEGER NOT NULL DEFAULT 0,
			indexed_at  INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_parent ON files(parent_path);
		CREATE INDEX IF NOT EXISTS idx_ext ON files(extension);
		CREATE INDEX IF NOT EXISTS idx_mod ON files(mod_time);

		CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
			name, path, content=files, content_rowid=id
		);

		CREATE TRIGGER IF NOT EXISTS files_ai AFTER INSERT ON files BEGIN
			INSERT INTO files_fts(rowid, name, path) VALUES (new.id, new.name, new.path);
		END;

		CREATE TRIGGER IF NOT EXISTS files_ad AFTER DELETE ON files BEGIN
			INSERT INTO files_fts(files_fts, rowid, name, path) VALUES('delete', old.id, old.name, old.path);
		END;

		CREATE TRIGGER IF NOT EXISTS files_au AFTER UPDATE ON files BEGIN
			INSERT INTO files_fts(files_fts, rowid, name, path) VALUES('delete', old.id, old.name, old.path);
			INSERT INTO files_fts(rowid, name, path) VALUES (new.id, new.name, new.path);
		END;

		CREATE TABLE IF NOT EXISTS favorites (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			path       TEXT NOT NULL UNIQUE,
			label      TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0
		);
	`
	_, err := idx.db.Exec(schema)
	return err
}

// DB returns the underlying database connection for shared use (e.g. search).
func (idx *Index) DB() *sql.DB {
	return idx.db
}

// Close closes the database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// ListDir returns all entries whose parent_path matches the given path.
func (idx *Index) ListDir(dirPath string) ([]FileEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query(
		`SELECT id, path, name, parent_path, extension, size, is_dir, mod_time, create_time, permissions, hidden, indexed_at
		 FROM files WHERE parent_path = ? ORDER BY is_dir DESC, lower(name) ASC`,
		dirPath,
	)
	if err != nil {
		return nil, fmt.Errorf("query dir: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// GetEntry returns a single file entry by path.
func (idx *Index) GetEntry(path string) (*FileEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	row := idx.db.QueryRow(
		`SELECT id, path, name, parent_path, extension, size, is_dir, mod_time, create_time, permissions, hidden, indexed_at
		 FROM files WHERE path = ?`,
		path,
	)

	entry := &FileEntry{}
	var isDir, hidden int
	var createTime, perms sql.NullInt64
	err := row.Scan(
		&entry.ID, &entry.Path, &entry.Name, &entry.ParentPath,
		&entry.Extension, &entry.Size, &isDir, &entry.ModTime,
		&createTime, &perms, &hidden, &entry.IndexedAt,
	)
	if err != nil {
		return nil, err
	}
	entry.IsDir = isDir == 1
	entry.Hidden = hidden == 1
	if createTime.Valid {
		entry.CreateTime = createTime.Int64
	}
	if perms.Valid {
		entry.Permissions = int(perms.Int64)
	}
	return entry, nil
}

// UpsertBatch inserts or updates a batch of file entries in a single transaction.
func (idx *Index) UpsertBatch(entries []FileEntry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO files (path, name, parent_path, extension, size, is_dir, mod_time, create_time, permissions, hidden, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			parent_path = excluded.parent_path,
			extension = excluded.extension,
			size = excluded.size,
			is_dir = excluded.is_dir,
			mod_time = excluded.mod_time,
			create_time = excluded.create_time,
			permissions = excluded.permissions,
			hidden = excluded.hidden,
			indexed_at = excluded.indexed_at
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, e := range entries {
		isDir := 0
		if e.IsDir {
			isDir = 1
		}
		hidden := 0
		if e.Hidden {
			hidden = 1
		}
		_, err := stmt.Exec(
			e.Path, e.Name, e.ParentPath, e.Extension,
			e.Size, isDir, e.ModTime, e.CreateTime,
			e.Permissions, hidden, now,
		)
		if err != nil {
			return fmt.Errorf("upsert %s: %w", e.Path, err)
		}
	}

	return tx.Commit()
}

// RemovePath deletes a single path from the index.
func (idx *Index) RemovePath(path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, err := idx.db.Exec("DELETE FROM files WHERE path = ?", path)
	return err
}

// RemoveTree deletes a directory and all its descendants from the index.
func (idx *Index) RemoveTree(dirPath string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, err := idx.db.Exec(
		"DELETE FROM files WHERE path = ? OR path LIKE ?",
		dirPath, dirPath+"/%",
	)
	return err
}

// Stats returns index statistics.
func (idx *Index) Stats() (totalFiles int64, totalDirs int64, dbSize int64, err error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	err = idx.db.QueryRow("SELECT COUNT(*) FROM files WHERE is_dir = 0").Scan(&totalFiles)
	if err != nil {
		return
	}
	err = idx.db.QueryRow("SELECT COUNT(*) FROM files WHERE is_dir = 1").Scan(&totalDirs)
	if err != nil {
		return
	}

	info, statErr := os.Stat(idx.path)
	if statErr == nil {
		dbSize = info.Size()
	}
	return
}

// ListFavorites returns all favorites ordered by sort_order.
func (idx *Index) ListFavorites() ([]Favorite, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query("SELECT id, path, label, sort_order FROM favorites ORDER BY sort_order ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favs []Favorite
	for rows.Next() {
		var f Favorite
		var label sql.NullString
		if err := rows.Scan(&f.ID, &f.Path, &label, &f.SortOrder); err != nil {
			return nil, err
		}
		if label.Valid {
			f.Label = label.String
		}
		favs = append(favs, f)
	}
	return favs, rows.Err()
}

// AddFavorite adds a path to the favorites list.
func (idx *Index) AddFavorite(path, label string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var maxOrder int
	idx.db.QueryRow("SELECT COALESCE(MAX(sort_order), 0) FROM favorites").Scan(&maxOrder)

	_, err := idx.db.Exec(
		"INSERT INTO favorites (path, label, sort_order) VALUES (?, ?, ?) ON CONFLICT(path) DO UPDATE SET label = excluded.label",
		path, label, maxOrder+1,
	)
	return err
}

// RemoveFavorite removes a path from the favorites list.
func (idx *Index) RemoveFavorite(path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, err := idx.db.Exec("DELETE FROM favorites WHERE path = ?", path)
	return err
}

// scanEntries reads rows into FileEntry slices.
func scanEntries(rows *sql.Rows) ([]FileEntry, error) {
	var entries []FileEntry
	for rows.Next() {
		var e FileEntry
		var isDir, hidden int
		var createTime, perms sql.NullInt64
		err := rows.Scan(
			&e.ID, &e.Path, &e.Name, &e.ParentPath,
			&e.Extension, &e.Size, &isDir, &e.ModTime,
			&createTime, &perms, &hidden, &e.IndexedAt,
		)
		if err != nil {
			return nil, err
		}
		e.IsDir = isDir == 1
		e.Hidden = hidden == 1
		if createTime.Valid {
			e.CreateTime = createTime.Int64
		}
		if perms.Valid {
			e.Permissions = int(perms.Int64)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
