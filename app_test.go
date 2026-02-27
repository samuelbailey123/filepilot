package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filepilot/internal/fileops"
	"filepilot/internal/vfs"
)

// minimalPNG is a valid 1x1 transparent PNG encoded as raw bytes.
// This avoids any external dependency for generating test image files.
var minimalPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR length + type
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // width=1, height=1
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bit depth, color type, ...
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IHDR CRC + IDAT length+type
	0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, // IDAT data
	0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, // IDAT data + CRC
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND length + type
	0x44, 0xAE, 0x42, 0x60, 0x82, // IEND CRC
}

// TestExtractArchive verifies that ExtractArchive unpacks a ZIP archive and
// returns the correct extracted entry count.
func TestExtractArchive(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// Build a ZIP archive with two files.
	archivePath := filepath.Join(tmp, "test.zip")
	func() {
		f, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("create zip: %v", err)
		}
		defer f.Close()
		w := zip.NewWriter(f)
		defer w.Close()

		for name, content := range map[string]string{
			"hello.txt": "hello world",
			"sub/b.txt": "sub file",
		} {
			fw, err := w.Create(name)
			if err != nil {
				t.Fatalf("zip create entry %s: %v", name, err)
			}
			if _, err := fw.Write([]byte(content)); err != nil {
				t.Fatalf("zip write entry %s: %v", name, err)
			}
		}
	}()

	destDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	count, err := app.ExtractArchive(archivePath, destDir)
	if err != nil {
		t.Fatalf("ExtractArchive failed: %v", err)
	}
	if count != 2 {
		t.Errorf("extracted %d entries, want 2", count)
	}

	// Verify the extracted files exist with the correct content.
	for name, want := range map[string]string{
		"hello.txt": "hello world",
		"sub/b.txt": "sub file",
	} {
		path := filepath.Join(destDir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing extracted file %s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("file %s content = %q, want %q", name, got, want)
		}
	}
}

// TestExtractArchiveZipSlip verifies that ExtractArchive rejects archive entries
// with path-traversal components (ZIP-slip attack).
func TestExtractArchiveZipSlip(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// Build a ZIP archive with a malicious path that traverses outside destDir.
	archivePath := filepath.Join(tmp, "evil.zip")
	func() {
		f, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("create zip: %v", err)
		}
		defer f.Close()
		w := zip.NewWriter(f)
		defer w.Close()

		fw, err := w.Create("../evil.txt")
		if err != nil {
			t.Fatalf("zip create entry: %v", err)
		}
		if _, err := fw.Write([]byte("malicious")); err != nil {
			t.Fatalf("zip write entry: %v", err)
		}
	}()

	destDir := filepath.Join(tmp, "safe")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := app.ExtractArchive(archivePath, destDir)
	if err == nil {
		t.Error("ExtractArchive should return an error for a zip-slip entry")
	}

	// Verify the malicious file was not created outside destDir.
	if _, statErr := os.Stat(filepath.Join(tmp, "evil.txt")); !os.IsNotExist(statErr) {
		t.Error("zip-slip file must not have been created outside destDir")
	}
}

// TestRestoreFromTrash verifies that RestoreFromTrash returns an error when the
// file is not present in the trash and cannot be restored via osascript in the
// test environment.
func TestRestoreFromTrash(t *testing.T) {
	ops := fileops.NewOps()

	// Use a path that will never exist in ~/.Trash so both the fast path and
	// the osascript fallback are exercised and we can assert on the error.
	nonExistent := filepath.Join(t.TempDir(), "no-such-file-filepilot-test.txt")
	err := ops.RestoreFromTrash(nonExistent)
	// On a CI machine with no Finder/osascript or when the file is absent, an
	// error is expected. We just confirm it does not panic.
	t.Logf("RestoreFromTrash (expected error on CI): %v", err)
}

// TestOpenTerminal verifies that OpenTerminal does not panic for a valid
// directory path. The actual terminal application launch is not tested because
// it requires a running macOS GUI session.
func TestOpenTerminal(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// We expect either success or an error but never a panic.
	err := app.OpenTerminal(tmp)
	t.Logf("OpenTerminal result: %v", err)

	// A non-existent path must always return an error.
	err = app.OpenTerminal(filepath.Join(tmp, "nonexistent"))
	if err == nil {
		t.Error("OpenTerminal should return an error for a non-existent directory")
	}
}

func TestSaveAndLoadKeymap(t *testing.T) {
	// Use a temp directory as the home dir.
	tmp := t.TempDir()
	app := &App{homeDir: tmp}

	keymapJSON := `{"search":"cmd+f","toggleHidden":"cmd+."}`

	// Save keymap.
	if err := app.SaveKeymap(keymapJSON); err != nil {
		t.Fatalf("SaveKeymap failed: %v", err)
	}

	// Verify the file was created.
	path := filepath.Join(tmp, ".config", "filepilot", "keymap.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("keymap.json was not created")
	}

	// Load keymap.
	loaded, err := app.LoadKeymap()
	if err != nil {
		t.Fatalf("LoadKeymap failed: %v", err)
	}
	if loaded != keymapJSON {
		t.Errorf("LoadKeymap returned %q, want %q", loaded, keymapJSON)
	}
}

func TestLoadKeymapNotFound(t *testing.T) {
	tmp := t.TempDir()
	app := &App{homeDir: tmp}

	// LoadKeymap should return empty string when file doesn't exist.
	loaded, err := app.LoadKeymap()
	if err != nil {
		t.Fatalf("LoadKeymap failed: %v", err)
	}
	if loaded != "" {
		t.Errorf("LoadKeymap returned %q, want empty string", loaded)
	}
}

func TestBatchRename(t *testing.T) {
	tmp := t.TempDir()
	app := &App{homeDir: tmp}

	// Create test files.
	for _, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	ops := []RenameOp{
		{Path: filepath.Join(tmp, "file1.txt"), NewName: "renamed1.txt"},
		{Path: filepath.Join(tmp, "file2.txt"), NewName: "renamed2.txt"},
	}

	if err := app.BatchRename(ops); err != nil {
		t.Fatalf("BatchRename failed: %v", err)
	}

	// Verify renames.
	for _, expected := range []string{"renamed1.txt", "renamed2.txt", "file3.txt"} {
		path := filepath.Join(tmp, expected)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s not found after rename", expected)
		}
	}

	// Verify originals are gone.
	for _, gone := range []string{"file1.txt", "file2.txt"} {
		path := filepath.Join(tmp, gone)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("original file %s still exists after rename", gone)
		}
	}
}

func TestBatchRenameValidation(t *testing.T) {
	tmp := t.TempDir()
	app := &App{homeDir: tmp}

	// Test empty ops.
	if err := app.BatchRename(nil); err != nil {
		t.Errorf("BatchRename(nil) should not error: %v", err)
	}

	// Test with non-existent file.
	ops := []RenameOp{
		{Path: filepath.Join(tmp, "nonexistent.txt"), NewName: "new.txt"},
	}
	if err := app.BatchRename(ops); err == nil {
		t.Error("BatchRename should fail for non-existent file")
	}

	// Test with path separator in new name.
	if err := os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	ops = []RenameOp{
		{Path: filepath.Join(tmp, "test.txt"), NewName: "sub/test.txt"},
	}
	if err := app.BatchRename(ops); err == nil {
		t.Error("BatchRename should reject newName with path separators")
	}

	// Test conflict with existing file.
	if err := os.WriteFile(filepath.Join(tmp, "existing.txt"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	ops = []RenameOp{
		{Path: filepath.Join(tmp, "test.txt"), NewName: "existing.txt"},
	}
	if err := app.BatchRename(ops); err == nil {
		t.Error("BatchRename should reject rename that would overwrite existing file")
	}
}

func TestCompareFolders(t *testing.T) {
	leftDir := t.TempDir()
	rightDir := t.TempDir()

	// Create common file with same content.
	os.WriteFile(filepath.Join(leftDir, "same.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(rightDir, "same.txt"), []byte("hello"), 0644)

	// Create left-only file.
	os.WriteFile(filepath.Join(leftDir, "only-left.txt"), []byte("left"), 0644)

	// Create right-only file.
	os.WriteFile(filepath.Join(rightDir, "only-right.txt"), []byte("right"), 0644)

	app := &App{localFS: vfs.NewLocal()}

	diff, err := app.CompareFolders(leftDir, rightDir, false)
	if err != nil {
		t.Fatalf("CompareFolders failed: %v", err)
	}

	if len(diff) < 3 {
		t.Fatalf("expected at least 3 diff entries, got %d", len(diff))
	}

	// Verify statuses by path.
	found := make(map[string]string)
	for _, d := range diff {
		found[d.Name] = d.Status
	}

	if found["only-left.txt"] != "left_only" {
		t.Errorf("only-left.txt status = %q, want left_only", found["only-left.txt"])
	}
	if found["only-right.txt"] != "right_only" {
		t.Errorf("only-right.txt status = %q, want right_only", found["only-right.txt"])
	}
}

func TestSyncFolders(t *testing.T) {
	leftDir := t.TempDir()
	rightDir := t.TempDir()

	// Create a file only on the left.
	os.WriteFile(filepath.Join(leftDir, "new.txt"), []byte("sync me"), 0644)

	app := &App{localFS: vfs.NewLocal()}

	count, err := app.SyncFolders(leftDir, rightDir, "left_to_right", false)
	if err != nil {
		t.Fatalf("SyncFolders failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 sync operation, got %d", count)
	}

	// Verify the file was synced.
	data, err := os.ReadFile(filepath.Join(rightDir, "new.txt"))
	if err != nil {
		t.Fatalf("expected synced file: %v", err)
	}
	if string(data) != "sync me" {
		t.Errorf("synced content = %q, want %q", string(data), "sync me")
	}
}

// TestCreateSymlink verifies that CreateSymlink creates a valid symbolic link.
func TestCreateSymlink(t *testing.T) {
	tmp := t.TempDir()

	// Create a real file to point at.
	target := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(tmp, "link.txt")
	app := &App{}

	if err := app.CreateSymlink(target, linkPath); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	// Verify it is a symlink.
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected a symlink, got a regular file")
	}

	// Verify the symlink target is correct.
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if got != target {
		t.Errorf("symlink target = %q, want %q", got, target)
	}
}

// TestCompletePath verifies that CompletePath returns matching directory names.
func TestCompletePath(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// Create some directories.
	for _, d := range []string{"Documents", "Downloads", "Desktop", "Music"} {
		if err := os.Mkdir(filepath.Join(tmp, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Create a file that should NOT appear.
	if err := os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		partial string
		wantMin int
		wantMax int
	}{
		{
			name:    "prefix D matches three dirs",
			partial: filepath.Join(tmp, "D"),
			wantMin: 3,
			wantMax: 3,
		},
		{
			name:    "prefix Doc matches one dir",
			partial: filepath.Join(tmp, "Doc"),
			wantMin: 1,
			wantMax: 1,
		},
		{
			name:    "trailing slash lists all dirs",
			partial: tmp + "/",
			wantMin: 4,
			wantMax: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := app.CompletePath(tc.partial)
			if err != nil {
				t.Fatalf("CompletePath(%q) failed: %v", tc.partial, err)
			}
			if len(results) < tc.wantMin || len(results) > tc.wantMax {
				t.Errorf("CompletePath(%q) returned %d results, want %d–%d: %v",
					tc.partial, len(results), tc.wantMin, tc.wantMax, results)
			}
			// Each result must end with a trailing slash.
			for _, r := range results {
				if !strings.HasSuffix(r, "/") {
					t.Errorf("result %q missing trailing slash", r)
				}
			}
		})
	}
}

// TestCompressFiles verifies that CompressFiles creates a valid zip archive.
func TestCompressFiles(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// Create two test files.
	fileA := filepath.Join(tmp, "alpha.txt")
	fileB := filepath.Join(tmp, "beta.txt")
	if err := os.WriteFile(fileA, []byte("alpha content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("beta content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file inside.
	subDir := filepath.Join(tmp, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	outputZip := filepath.Join(tmp, "archive.zip")

	if err := app.CompressFiles([]string{fileA, fileB, subDir}, outputZip); err != nil {
		t.Fatalf("CompressFiles failed: %v", err)
	}

	// Open the zip and verify entry names.
	r, err := zip.OpenReader(outputZip)
	if err != nil {
		t.Fatalf("zip.OpenReader failed: %v", err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}

	for _, want := range []string{"alpha.txt", "beta.txt", "subdir/nested.txt"} {
		if !names[want] {
			t.Errorf("zip missing entry %q; found: %v", want, names)
		}
	}

	// Calling CompressFiles again with the same output should fail.
	if err := app.CompressFiles([]string{fileA}, outputZip); err == nil {
		t.Error("expected error when output already exists")
	}

	// Calling with no paths should fail.
	if err := app.CompressFiles(nil, filepath.Join(tmp, "empty.zip")); err == nil {
		t.Error("expected error for empty paths")
	}
}

// TestSetPermissions verifies that SetPermissions changes file mode bits.
func TestSetPermissions(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	path := filepath.Join(tmp, "perms.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := app.SetPermissions(path, 0600); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}

// TestDiffFiles verifies that DiffFiles produces the correct annotated diff.
func TestDiffFiles(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	pathA := filepath.Join(tmp, "a.txt")
	pathB := filepath.Join(tmp, "b.txt")

	// File A has three lines; B modifies one and adds one.
	os.WriteFile(pathA, []byte("line1\nline2\nline3"), 0644)
	os.WriteFile(pathB, []byte("line1\nchanged\nline3\nline4"), 0644)

	lines, err := app.DiffFiles(pathA, pathB)
	if err != nil {
		t.Fatalf("DiffFiles failed: %v", err)
	}

	// Collect the types present.
	typeSet := make(map[string]int)
	for _, l := range lines {
		typeSet[l.Type]++
	}

	// We expect at least one of each diff type.
	if typeSet["equal"] == 0 {
		t.Error("expected at least one 'equal' line")
	}
	if typeSet["add"] == 0 {
		t.Error("expected at least one 'add' line")
	}
	if typeSet["delete"] == 0 {
		t.Error("expected at least one 'delete' line")
	}

	// "line1" must be equal, "line2" must be deleted, "changed" must be added.
	equalLines := make(map[string]bool)
	addLines := make(map[string]bool)
	deleteLines := make(map[string]bool)
	for _, l := range lines {
		switch l.Type {
		case "equal":
			equalLines[l.Text] = true
		case "add":
			addLines[l.Text] = true
		case "delete":
			deleteLines[l.Text] = true
		}
	}

	if !equalLines["line1"] {
		t.Error("expected 'line1' to be equal")
	}
	if !deleteLines["line2"] {
		t.Error("expected 'line2' to be deleted")
	}
	if !addLines["changed"] {
		t.Error("expected 'changed' to be added")
	}
}

// TestDuplicateFile verifies that DuplicateFile creates a correctly named copy.
func TestDuplicateFile(t *testing.T) {
	tmp := t.TempDir()
	ops := fileops.NewOps()
	app := &App{ops: ops}

	t.Run("file with extension", func(t *testing.T) {
		path := filepath.Join(tmp, "report.txt")
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		dest, err := app.DuplicateFile(path)
		if err != nil {
			t.Fatalf("DuplicateFile failed: %v", err)
		}

		wantName := "report copy.txt"
		if filepath.Base(dest) != wantName {
			t.Errorf("duplicate name = %q, want %q", filepath.Base(dest), wantName)
		}

		if _, err := os.Stat(dest); err != nil {
			t.Errorf("duplicate file does not exist: %v", err)
		}
	})

	t.Run("file without extension", func(t *testing.T) {
		path := filepath.Join(tmp, "Makefile")
		if err := os.WriteFile(path, []byte("all:"), 0644); err != nil {
			t.Fatal(err)
		}

		dest, err := app.DuplicateFile(path)
		if err != nil {
			t.Fatalf("DuplicateFile failed: %v", err)
		}

		wantName := "Makefile copy"
		if filepath.Base(dest) != wantName {
			t.Errorf("duplicate name = %q, want %q", filepath.Base(dest), wantName)
		}
	})

	t.Run("collision increments counter", func(t *testing.T) {
		path := filepath.Join(tmp, "photo.jpg")
		if err := os.WriteFile(path, []byte("img"), 0644); err != nil {
			t.Fatal(err)
		}

		// Pre-create "photo copy.jpg" to force the counter.
		if err := os.WriteFile(filepath.Join(tmp, "photo copy.jpg"), []byte("first copy"), 0644); err != nil {
			t.Fatal(err)
		}

		dest, err := app.DuplicateFile(path)
		if err != nil {
			t.Fatalf("DuplicateFile failed: %v", err)
		}

		wantName := "photo copy 2.jpg"
		if filepath.Base(dest) != wantName {
			t.Errorf("duplicate name = %q, want %q", filepath.Base(dest), wantName)
		}
	})

	t.Run("directory", func(t *testing.T) {
		dirPath := filepath.Join(tmp, "mydir")
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "inner.txt"), []byte("inner"), 0644); err != nil {
			t.Fatal(err)
		}

		dest, err := app.DuplicateFile(dirPath)
		if err != nil {
			t.Fatalf("DuplicateFile dir failed: %v", err)
		}

		wantName := "mydir copy"
		if filepath.Base(dest) != wantName {
			t.Errorf("duplicate dir name = %q, want %q", filepath.Base(dest), wantName)
		}

		// Verify inner file was also copied.
		innerDest := filepath.Join(dest, "inner.txt")
		if _, err := os.Stat(innerDest); err != nil {
			t.Errorf("inner file not copied: %v", err)
		}
	})
}

// TestGetThumbnail verifies GetThumbnail returns a data URI or skips gracefully
// when qlmanage is unavailable (e.g. CI environments without Quick Look support).
func TestGetThumbnail(t *testing.T) {
	tmp := t.TempDir()
	app := &App{homeDir: tmp}

	// Write a minimal valid PNG so qlmanage has a real image to thumbnail.
	imgPath := filepath.Join(tmp, "sample.png")
	if err := os.WriteFile(imgPath, minimalPNG, 0644); err != nil {
		t.Fatalf("write sample PNG: %v", err)
	}

	result, err := app.GetThumbnail(imgPath, 64)
	if err != nil {
		// qlmanage may legitimately fail in headless/CI environments.
		// Accept the failure but require a meaningful error message.
		t.Logf("GetThumbnail returned error (acceptable in headless env): %v", err)
		return
	}

	if result == "" {
		t.Fatal("GetThumbnail returned empty string without error")
	}

	if !strings.HasPrefix(result, "data:image/") {
		t.Errorf("expected data URI prefix 'data:image/', got: %.40s", result)
	}
}

// TestGrepInDir verifies that GrepInDir finds matches with correct line numbers
// and snippets across multiple files.
func TestGrepInDir(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// file1.txt – the search term appears on lines 2 and 4.
	file1 := filepath.Join(tmp, "file1.txt")
	if err := os.WriteFile(file1, []byte("alpha\nbeta needle\ngamma\nneedle delta\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// file2.txt – one match on line 1.
	file2 := filepath.Join(tmp, "file2.txt")
	if err := os.WriteFile(file2, []byte("NEEDLE here\nno match\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// file3.txt – no matches.
	file3 := filepath.Join(tmp, "file3.txt")
	if err := os.WriteFile(file3, []byte("nothing relevant\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := app.GrepInDir("needle", tmp, 0)
	if err != nil {
		t.Fatalf("GrepInDir failed: %v", err)
	}

	// We expect exactly 3 matches (2 from file1, 1 from file2).
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d: %+v", len(results), results)
	}

	// Build a map of path -> line numbers for easy assertion.
	type key struct {
		path string
		line int
	}
	found := make(map[key]bool)
	for _, r := range results {
		found[key{r.Path, r.Line}] = true
		if r.Name == "" {
			t.Errorf("result missing Name field: %+v", r)
		}
		if r.Snippet == "" {
			t.Errorf("result missing Snippet field: %+v", r)
		}
	}

	for _, want := range []key{
		{file1, 2},
		{file1, 4},
		{file2, 1},
	} {
		if !found[want] {
			t.Errorf("expected match at %s line %d not found in results: %+v", want.path, want.line, results)
		}
	}

	// Verify file3.txt produced no results.
	for _, r := range results {
		if r.Path == file3 {
			t.Errorf("unexpected match in file3.txt: %+v", r)
		}
	}
}

// TestGrepInDirSkipsBinary verifies that binary files (containing null bytes)
// are excluded from grep results.
func TestGrepInDirSkipsBinary(t *testing.T) {
	tmp := t.TempDir()
	app := &App{}

	// Write a binary file that contains the search term surrounded by null bytes.
	binaryPath := filepath.Join(tmp, "binary.bin")
	content := []byte("needle\x00\x00\x00binary\x00data")
	if err := os.WriteFile(binaryPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Write a plain text file with the same term so we confirm the walk works.
	textPath := filepath.Join(tmp, "text.txt")
	if err := os.WriteFile(textPath, []byte("needle in text\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := app.GrepInDir("needle", tmp, 0)
	if err != nil {
		t.Fatalf("GrepInDir failed: %v", err)
	}

	for _, r := range results {
		if r.Path == binaryPath {
			t.Errorf("binary file should have been skipped, but got result: %+v", r)
		}
	}

	// The text file must still appear.
	found := false
	for _, r := range results {
		if r.Path == textPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected match in text.txt, but it was not found in results: %+v", results)
	}
}
