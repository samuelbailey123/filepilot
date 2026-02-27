package index

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// testIndex creates a temporary Index backed by an in-memory-like temp file.
// The caller should defer idx.Close() and os.RemoveAll(dir).
func testIndex(t *testing.T) (*Index, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	idx, err := NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	return idx, dir
}

func TestNewIndex_CreatesDB(t *testing.T) {
	idx, dir := testIndex(t)
	defer idx.Close()

	dbPath := filepath.Join(dir, "test.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

func TestNewIndex_InvalidPath(t *testing.T) {
	// A path under /dev/null should fail on directory creation.
	_, err := NewIndex("/dev/null/impossible/path/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestNewIndex_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create twice — second open should succeed (migrate is idempotent).
	idx1, err := NewIndex(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	idx1.Close()

	idx2, err := NewIndex(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	idx2.Close()
}

func TestUpsertBatch_And_ListDir(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/home/user/docs", Name: "docs", ParentPath: "/home/user", IsDir: true, ModTime: 1000},
		{Path: "/home/user/readme.md", Name: "readme.md", ParentPath: "/home/user", Extension: "md", Size: 256, ModTime: 1001},
		{Path: "/home/user/main.go", Name: "main.go", ParentPath: "/home/user", Extension: "go", Size: 512, ModTime: 1002},
	}

	if err := idx.UpsertBatch(entries); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	result, err := idx.ListDir("/home/user")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// Directories should sort first.
	if !result[0].IsDir {
		t.Error("expected first entry to be a directory")
	}
	if result[0].Name != "docs" {
		t.Errorf("expected first entry name 'docs', got %q", result[0].Name)
	}
}

func TestUpsertBatch_Update(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/test/file.txt", Name: "file.txt", ParentPath: "/test", Extension: "txt", Size: 100, ModTime: 1000},
	}
	if err := idx.UpsertBatch(entries); err != nil {
		t.Fatalf("initial UpsertBatch: %v", err)
	}

	// Update the same path with different size.
	entries[0].Size = 200
	if err := idx.UpsertBatch(entries); err != nil {
		t.Fatalf("update UpsertBatch: %v", err)
	}

	entry, err := idx.GetEntry("/test/file.txt")
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if entry.Size != 200 {
		t.Errorf("expected size 200 after update, got %d", entry.Size)
	}
}

func TestUpsertBatch_Empty(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	// Empty batch should succeed without error.
	if err := idx.UpsertBatch(nil); err != nil {
		t.Fatalf("UpsertBatch(nil): %v", err)
	}
	if err := idx.UpsertBatch([]FileEntry{}); err != nil {
		t.Fatalf("UpsertBatch(empty): %v", err)
	}
}

func TestGetEntry_Found(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/a/b.go", Name: "b.go", ParentPath: "/a", Extension: "go", Size: 42, ModTime: 9999, Permissions: 0644, Hidden: false},
	}
	idx.UpsertBatch(entries)

	entry, err := idx.GetEntry("/a/b.go")
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if entry.Name != "b.go" {
		t.Errorf("expected name 'b.go', got %q", entry.Name)
	}
	if entry.Extension != "go" {
		t.Errorf("expected extension 'go', got %q", entry.Extension)
	}
	if entry.Size != 42 {
		t.Errorf("expected size 42, got %d", entry.Size)
	}
}

func TestGetEntry_NotFound(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	_, err := idx.GetEntry("/nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestRemovePath(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/x/y.txt", Name: "y.txt", ParentPath: "/x", Extension: "txt", Size: 10, ModTime: 1},
		{Path: "/x/z.txt", Name: "z.txt", ParentPath: "/x", Extension: "txt", Size: 20, ModTime: 2},
	}
	idx.UpsertBatch(entries)

	if err := idx.RemovePath("/x/y.txt"); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}

	_, err := idx.GetEntry("/x/y.txt")
	if err != sql.ErrNoRows {
		t.Error("expected y.txt to be removed")
	}

	// z.txt should still exist.
	entry, err := idx.GetEntry("/x/z.txt")
	if err != nil {
		t.Fatalf("z.txt should still exist: %v", err)
	}
	if entry.Name != "z.txt" {
		t.Error("z.txt has wrong name")
	}
}

func TestRemoveTree(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/proj", Name: "proj", ParentPath: "/", IsDir: true, ModTime: 1},
		{Path: "/proj/src", Name: "src", ParentPath: "/proj", IsDir: true, ModTime: 2},
		{Path: "/proj/src/main.go", Name: "main.go", ParentPath: "/proj/src", Extension: "go", Size: 100, ModTime: 3},
		{Path: "/other/file.txt", Name: "file.txt", ParentPath: "/other", Extension: "txt", Size: 50, ModTime: 4},
	}
	idx.UpsertBatch(entries)

	if err := idx.RemoveTree("/proj"); err != nil {
		t.Fatalf("RemoveTree: %v", err)
	}

	// All /proj paths should be gone.
	for _, p := range []string{"/proj", "/proj/src", "/proj/src/main.go"} {
		_, err := idx.GetEntry(p)
		if err != sql.ErrNoRows {
			t.Errorf("%s should have been removed", p)
		}
	}

	// /other/file.txt should survive.
	entry, err := idx.GetEntry("/other/file.txt")
	if err != nil {
		t.Fatalf("/other/file.txt should survive: %v", err)
	}
	if entry.Name != "file.txt" {
		t.Error("wrong name for surviving entry")
	}
}

func TestStats(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/d", Name: "d", ParentPath: "/", IsDir: true, ModTime: 1},
		{Path: "/d/a.txt", Name: "a.txt", ParentPath: "/d", Extension: "txt", Size: 10, ModTime: 2},
		{Path: "/d/b.txt", Name: "b.txt", ParentPath: "/d", Extension: "txt", Size: 20, ModTime: 3},
	}
	idx.UpsertBatch(entries)

	files, dirs, dbSize, err := idx.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if files != 2 {
		t.Errorf("expected 2 files, got %d", files)
	}
	if dirs != 1 {
		t.Errorf("expected 1 dir, got %d", dirs)
	}
	if dbSize <= 0 {
		t.Errorf("expected positive dbSize, got %d", dbSize)
	}
}

func TestListDir_Empty(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	result, err := idx.ListDir("/nonexistent")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries for nonexistent dir, got %d", len(result))
	}
}

func TestListDir_SortOrder(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/root/zebra.txt", Name: "zebra.txt", ParentPath: "/root", Extension: "txt", Size: 1, ModTime: 1},
		{Path: "/root/alpha", Name: "alpha", ParentPath: "/root", IsDir: true, ModTime: 2},
		{Path: "/root/aardvark.go", Name: "aardvark.go", ParentPath: "/root", Extension: "go", Size: 2, ModTime: 3},
		{Path: "/root/beta", Name: "beta", ParentPath: "/root", IsDir: true, ModTime: 4},
	}
	idx.UpsertBatch(entries)

	result, err := idx.ListDir("/root")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result))
	}

	// Directories first, then files, each alphabetical.
	if result[0].Name != "alpha" {
		t.Errorf("expected alpha first, got %s", result[0].Name)
	}
	if result[1].Name != "beta" {
		t.Errorf("expected beta second, got %s", result[1].Name)
	}
	if result[2].Name != "aardvark.go" {
		t.Errorf("expected aardvark.go third, got %s", result[2].Name)
	}
	if result[3].Name != "zebra.txt" {
		t.Errorf("expected zebra.txt fourth, got %s", result[3].Name)
	}
}

func TestAddFavorite_And_ListFavorites(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	if err := idx.AddFavorite("/home/docs", "Documents"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	if err := idx.AddFavorite("/home/desktop", "Desktop"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}

	favs, err := idx.ListFavorites()
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}

	if favs[0].Path != "/home/docs" || favs[0].Label != "Documents" {
		t.Errorf("first favorite: got path=%q label=%q", favs[0].Path, favs[0].Label)
	}
	if favs[1].Path != "/home/desktop" || favs[1].Label != "Desktop" {
		t.Errorf("second favorite: got path=%q label=%q", favs[1].Path, favs[1].Label)
	}

	// Sort order should increment.
	if favs[0].SortOrder >= favs[1].SortOrder {
		t.Errorf("expected ascending sort order, got %d >= %d", favs[0].SortOrder, favs[1].SortOrder)
	}
}

func TestAddFavorite_Upsert(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	idx.AddFavorite("/path", "Original")
	idx.AddFavorite("/path", "Updated")

	favs, _ := idx.ListFavorites()
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after upsert, got %d", len(favs))
	}
	if favs[0].Label != "Updated" {
		t.Errorf("expected label 'Updated', got %q", favs[0].Label)
	}
}

func TestRemoveFavorite(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	idx.AddFavorite("/home/docs", "Documents")
	idx.AddFavorite("/home/desktop", "Desktop")

	if err := idx.RemoveFavorite("/home/docs"); err != nil {
		t.Fatalf("RemoveFavorite: %v", err)
	}

	favs, err := idx.ListFavorites()
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after removal, got %d", len(favs))
	}
	if favs[0].Path != "/home/desktop" {
		t.Errorf("wrong favorite remaining: %q", favs[0].Path)
	}
}

func TestRemoveFavorite_Nonexistent(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	// Removing a nonexistent favorite should not error.
	if err := idx.RemoveFavorite("/nonexistent"); err != nil {
		t.Fatalf("RemoveFavorite nonexistent: %v", err)
	}
}

func TestFTS5_Integration(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/proj/README.md", Name: "README.md", ParentPath: "/proj", Extension: "md", Size: 100, ModTime: 1},
		{Path: "/proj/main.go", Name: "main.go", ParentPath: "/proj", Extension: "go", Size: 200, ModTime: 2},
		{Path: "/proj/utils.go", Name: "utils.go", ParentPath: "/proj", Extension: "go", Size: 300, ModTime: 3},
	}
	idx.UpsertBatch(entries)

	// The FTS5 virtual table should be populated via triggers.
	var count int
	err := idx.db.QueryRow("SELECT COUNT(*) FROM files_fts WHERE files_fts MATCH 'main'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS5 query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS5 match for 'main', got %d", count)
	}

	// Search for 'go' in path — should match both .go files.
	err = idx.db.QueryRow("SELECT COUNT(*) FROM files_fts WHERE files_fts MATCH 'utils'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS5 query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS5 match for 'utils', got %d", count)
	}
}

func TestFTS5_UpdateTrigger(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/a/old.txt", Name: "old.txt", ParentPath: "/a", Extension: "txt", Size: 10, ModTime: 1},
	}
	idx.UpsertBatch(entries)

	// Update the name.
	entries[0].Name = "new.txt"
	idx.UpsertBatch(entries)

	// Old name should not match in the name column.
	var count int
	idx.db.QueryRow("SELECT COUNT(*) FROM files_fts WHERE name MATCH 'old'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 FTS5 name matches for 'old' after update, got %d", count)
	}

	// New name should match in the name column.
	idx.db.QueryRow("SELECT COUNT(*) FROM files_fts WHERE name MATCH 'new'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 FTS5 name match for 'new' after update, got %d", count)
	}
}

func TestFTS5_DeleteTrigger(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/x/gone.txt", Name: "gone.txt", ParentPath: "/x", Extension: "txt", Size: 10, ModTime: 1},
	}
	idx.UpsertBatch(entries)
	idx.RemovePath("/x/gone.txt")

	var count int
	idx.db.QueryRow("SELECT COUNT(*) FROM files_fts WHERE files_fts MATCH 'gone'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 FTS5 matches after deletion, got %d", count)
	}
}

func TestHiddenAndSymlink(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	entries := []FileEntry{
		{Path: "/home/.hidden", Name: ".hidden", ParentPath: "/home", Hidden: true, ModTime: 1},
		{Path: "/home/link", Name: "link", ParentPath: "/home", IsSymlink: true, SymlinkTarget: "/target", ModTime: 2},
	}
	idx.UpsertBatch(entries)

	e, err := idx.GetEntry("/home/.hidden")
	if err != nil {
		t.Fatalf("GetEntry hidden: %v", err)
	}
	if !e.Hidden {
		t.Error("expected hidden=true")
	}
}

func TestClose(t *testing.T) {
	idx, _ := testIndex(t)

	if err := idx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Operations after close should fail.
	_, err := idx.ListDir("/")
	if err == nil {
		t.Error("expected error after Close")
	}
}

func TestDB(t *testing.T) {
	idx, _ := testIndex(t)
	defer idx.Close()

	db := idx.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}

	// Should be able to run a query.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count); err != nil {
		t.Fatalf("query via DB(): %v", err)
	}
}
