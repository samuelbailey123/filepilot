// Package search provides full-text and substring search over the file index.
package search

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Result represents a single search match.
type Result struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Size      int64  `json:"size"`
	IsDir     bool   `json:"isDir"`
	ModTime   int64  `json:"modTime"`
	Rank      float64 `json:"rank"`
	Snippet   string `json:"snippet"`
}

// Engine provides full-text search over the file index.
type Engine struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewEngine creates a search engine using the given database connection.
func NewEngine(db *sql.DB) *Engine {
	return &Engine{db: db}
}

// Search performs an FTS5 query and returns ranked results.
func (e *Engine) Search(query string, limit int) ([]Result, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Build FTS5 query: support both exact phrases and prefix matching.
	ftsQuery := buildFTSQuery(query)

	rows, err := e.db.Query(`
		SELECT f.id, f.path, f.name, f.extension, f.size, f.is_dir, f.mod_time,
		       rank
		FROM files_fts
		JOIN files f ON f.id = files_fts.rowid
		WHERE files_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime, &r.Rank); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.IsDir = isDir == 1
		r.Snippet = buildSnippet(r.Path, query)
		results = append(results, r)
	}

	return results, rows.Err()
}

// SearchInDir performs an FTS5 query scoped to a directory subtree.
func (e *Engine) SearchInDir(query string, dirPath string, limit int) ([]Result, error) {
	if query == "" || dirPath == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	ftsQuery := buildFTSQuery(query)
	prefix := dirPath + "/%"

	rows, err := e.db.Query(`
		SELECT f.id, f.path, f.name, f.extension, f.size, f.is_dir, f.mod_time,
		       rank
		FROM files_fts
		JOIN files f ON f.id = files_fts.rowid
		WHERE files_fts MATCH ? AND f.path LIKE ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("search in dir query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime, &r.Rank); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.IsDir = isDir == 1
		r.Snippet = buildSnippet(r.Path, query)
		results = append(results, r)
	}

	return results, rows.Err()
}

// SearchByExtension finds files with the given extension.
func (e *Engine) SearchByExtension(ext string, parentPath string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 100
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if parentPath != "" {
		rows, err = e.db.Query(`
			SELECT id, path, name, extension, size, is_dir, mod_time
			FROM files
			WHERE extension = ? AND path LIKE ?
			ORDER BY mod_time DESC
			LIMIT ?
		`, ext, parentPath+"/%", limit)
	} else {
		rows, err = e.db.Query(`
			SELECT id, path, name, extension, size, is_dir, mod_time
			FROM files
			WHERE extension = ?
			ORDER BY mod_time DESC
			LIMIT ?
		`, ext, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime); err != nil {
			return nil, err
		}
		r.IsDir = isDir == 1
		results = append(results, r)
	}
	return results, rows.Err()
}

// SubstringSearch performs a LIKE-based substring search on file names.
func (e *Engine) SubstringSearch(query string, limit int) ([]Result, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	pattern := "%" + query + "%"
	rows, err := e.db.Query(`
		SELECT id, path, name, extension, size, is_dir, mod_time
		FROM files
		WHERE name LIKE ? COLLATE NOCASE
		ORDER BY mod_time DESC
		LIMIT ?
	`, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("substring search: %w", err)
	}
	defer rows.Close()

	return scanResults(rows, query)
}

// SubstringSearchInDir performs a LIKE-based substring search scoped to a directory.
func (e *Engine) SubstringSearchInDir(query string, dirPath string, limit int) ([]Result, error) {
	if query == "" || dirPath == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	pattern := "%" + query + "%"
	prefix := dirPath + "/%"
	rows, err := e.db.Query(`
		SELECT id, path, name, extension, size, is_dir, mod_time
		FROM files
		WHERE name LIKE ? COLLATE NOCASE AND path LIKE ?
		ORDER BY mod_time DESC
		LIMIT ?
	`, pattern, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("substring search in dir: %w", err)
	}
	defer rows.Close()

	return scanResults(rows, query)
}

// RegexSearch performs a regex-based search using Go's regexp package.
// Results are fetched from the database and post-filtered by the regex pattern.
func (e *Engine) RegexSearch(pattern string, limit int) ([]Result, error) {
	if pattern == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Fetch a larger batch for post-filtering.
	rows, err := e.db.Query(`
		SELECT id, path, name, extension, size, is_dir, mod_time
		FROM files
		ORDER BY mod_time DESC
		LIMIT ?
	`, limit*10)
	if err != nil {
		return nil, fmt.Errorf("regex search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.IsDir = isDir == 1
		if re.MatchString(r.Name) {
			r.Snippet = buildSnippet(r.Path, pattern)
			results = append(results, r)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, rows.Err()
}

// RegexSearchInDir performs a regex-based search scoped to a directory.
func (e *Engine) RegexSearchInDir(pattern string, dirPath string, limit int) ([]Result, error) {
	if pattern == "" || dirPath == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	prefix := dirPath + "/%"
	rows, err := e.db.Query(`
		SELECT id, path, name, extension, size, is_dir, mod_time
		FROM files
		WHERE path LIKE ?
		ORDER BY mod_time DESC
		LIMIT ?
	`, prefix, limit*10)
	if err != nil {
		return nil, fmt.Errorf("regex search in dir query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.IsDir = isDir == 1
		if re.MatchString(r.Name) {
			r.Snippet = buildSnippet(r.Path, pattern)
			results = append(results, r)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, rows.Err()
}

// scanResults reads result rows from a non-FTS query.
func scanResults(rows *sql.Rows, query string) ([]Result, error) {
	var results []Result
	for rows.Next() {
		var r Result
		var isDir int
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Extension, &r.Size, &isDir, &r.ModTime); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.IsDir = isDir == 1
		r.Snippet = buildSnippet(r.Path, query)
		results = append(results, r)
	}
	return results, rows.Err()
}

// buildFTSQuery converts a user query into an FTS5 match expression.
// Adds prefix matching for partial words.
func buildFTSQuery(query string) string {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return query
	}

	// Each term becomes a prefix search.
	var parts []string
	for _, t := range terms {
		// Escape FTS5 special characters.
		t = strings.ReplaceAll(t, "\"", "")
		t = strings.ReplaceAll(t, "*", "")
		if t != "" {
			parts = append(parts, t+"*")
		}
	}

	return strings.Join(parts, " ")
}

// buildSnippet creates a path-based snippet highlighting the query match.
func buildSnippet(path, query string) string {
	lower := strings.ToLower(path)
	q := strings.ToLower(query)
	idx := strings.LastIndex(lower, q)
	if idx < 0 {
		// Show just the last 2 path components.
		parts := strings.Split(path, "/")
		if len(parts) > 2 {
			return ".../" + strings.Join(parts[len(parts)-2:], "/")
		}
		return path
	}

	// Show context around the match.
	start := idx - 20
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 20
	if end > len(path) {
		end = len(path)
	}

	snippet := path[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(path) {
		snippet = snippet + "..."
	}
	return snippet
}
