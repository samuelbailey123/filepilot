package search

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}

	schema := `
		CREATE TABLE files (
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

		CREATE VIRTUAL TABLE files_fts USING fts5(
			name, path, content=files, content_rowid=id
		);

		CREATE TRIGGER files_ai AFTER INSERT ON files BEGIN
			INSERT INTO files_fts(rowid, name, path) VALUES (new.id, new.name, new.path);
		END;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return db
}

// insertTestFile inserts a file record into the test database.
func insertTestFile(t *testing.T, db *sql.DB, path, name, ext string, isDir bool) {
	t.Helper()
	dir := 0
	if isDir {
		dir = 1
	}
	_, err := db.Exec(`
		INSERT INTO files (path, name, parent_path, extension, size, is_dir, mod_time, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, path, name, "/test", ext, 1024, dir, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		t.Fatalf("insert test file: %v", err)
	}
}

// TestSearchBasic verifies that searching returns matching results.
func TestSearchBasic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestFile(t, db, "/test/hello.go", "hello.go", "go", false)
	insertTestFile(t, db, "/test/world.txt", "world.txt", "txt", false)
	insertTestFile(t, db, "/test/docs", "docs", "", true)

	engine := NewEngine(db)

	results, err := engine.Search("hello", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "hello.go" {
		t.Errorf("expected name 'hello.go', got %q", results[0].Name)
	}
}

// TestSearchEmpty verifies that an empty query returns no results.
func TestSearchEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	engine := NewEngine(db)

	results, err := engine.Search("", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

// TestSearchInDir verifies that directory-scoped search works.
func TestSearchInDir(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestFile(t, db, "/test/src/main.go", "main.go", "go", false)
	insertTestFile(t, db, "/other/main.go", "main.go", "go", false)

	engine := NewEngine(db)

	results, err := engine.SearchInDir("main", "/test", 10)
	if err != nil {
		t.Fatalf("SearchInDir: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result scoped to /test, got %d", len(results))
	}
	if results[0].Path != "/test/src/main.go" {
		t.Errorf("expected path '/test/src/main.go', got %q", results[0].Path)
	}
}

// TestSubstringSearch verifies that LIKE-based substring matching works.
func TestSubstringSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestFile(t, db, "/test/hello_world.go", "hello_world.go", "go", false)
	insertTestFile(t, db, "/test/readme.md", "readme.md", "md", false)
	insertTestFile(t, db, "/test/helpers.go", "helpers.go", "go", false)

	engine := NewEngine(db)

	// Substring "llo" should match "hello_world.go" (middle of word).
	results, err := engine.SubstringSearch("llo", 10)
	if err != nil {
		t.Fatalf("SubstringSearch: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'llo', got %d", len(results))
	}
	if results[0].Name != "hello_world.go" {
		t.Errorf("expected name 'hello_world.go', got %q", results[0].Name)
	}
}

// TestSubstringSearchInDir verifies scoped substring search.
func TestSubstringSearchInDir(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestFile(t, db, "/test/src/helper.go", "helper.go", "go", false)
	insertTestFile(t, db, "/other/helper.go", "helper.go", "go", false)

	engine := NewEngine(db)

	results, err := engine.SubstringSearchInDir("elp", "/test", 10)
	if err != nil {
		t.Fatalf("SubstringSearchInDir: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result scoped to /test, got %d", len(results))
	}
}

// TestRegexSearch verifies regex-based search.
func TestRegexSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestFile(t, db, "/test/app_v1.go", "app_v1.go", "go", false)
	insertTestFile(t, db, "/test/app_v2.go", "app_v2.go", "go", false)
	insertTestFile(t, db, "/test/readme.md", "readme.md", "md", false)

	engine := NewEngine(db)

	// Regex for files matching "app_v\d"
	results, err := engine.RegexSearch(`app_v\d`, 10)
	if err != nil {
		t.Fatalf("RegexSearch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'app_v\\d', got %d", len(results))
	}
}

// TestRegexSearchInvalid verifies that an invalid regex returns an error.
func TestRegexSearchInvalid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	engine := NewEngine(db)

	_, err := engine.RegexSearch("[invalid", 10)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

// TestSubstringSearchEmpty verifies that an empty substring returns no results.
func TestSubstringSearchEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	engine := NewEngine(db)

	results, err := engine.SubstringSearch("", 10)
	if err != nil {
		t.Fatalf("SubstringSearch: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}
