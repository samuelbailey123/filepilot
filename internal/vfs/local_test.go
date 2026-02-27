package vfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Compile-time check that Local implements FileSystem.
var _ FileSystem = (*Local)(nil)

func TestLocalList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	fs := NewLocal()
	entries, err := fs.List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify one dir and two files.
	dirs := 0
	files := 0
	for _, e := range entries {
		if e.IsDir {
			dirs++
		} else {
			files++
		}
	}
	if dirs != 1 || files != 2 {
		t.Errorf("expected 1 dir + 2 files, got %d dirs + %d files", dirs, files)
	}
}

func TestLocalStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("content"), 0644)

	fs := NewLocal()
	entry, err := fs.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if entry.Name != "test.txt" {
		t.Errorf("expected name 'test.txt', got %q", entry.Name)
	}
	if entry.Size != 7 {
		t.Errorf("expected size 7, got %d", entry.Size)
	}
	if entry.IsDir {
		t.Error("expected file, got directory")
	}
}

func TestLocalReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rw.txt")

	fs := NewLocal()

	// Write.
	err := fs.Write(path, strings.NewReader("test content"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read.
	rc, err := fs.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "test content" {
		t.Errorf("expected 'test content', got %q", string(data))
	}
}

func TestLocalMkdir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c")

	fs := NewLocal()
	if err := fs.Mkdir(path); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestLocalRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rm.txt")
	os.WriteFile(path, []byte("delete me"), 0644)

	fs := NewLocal()
	if err := fs.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestLocalRename(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	dst := filepath.Join(dir, "new.txt")
	os.WriteFile(src, []byte("rename me"), 0644)

	fs := NewLocal()
	if err := fs.Rename(src, dst); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("expected old path to not exist")
	}
	if _, err := os.Stat(dst); err != nil {
		t.Error("expected new path to exist")
	}
}

func TestLocalCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	os.WriteFile(src, []byte("copy me"), 0644)

	fs := NewLocal()
	if err := fs.Copy(src, dst); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	// Both should exist.
	if _, err := os.Stat(src); err != nil {
		t.Error("expected source to still exist")
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "copy me" {
		t.Errorf("expected 'copy me', got %q", string(data))
	}
}

func TestLocalCopyDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")

	os.Mkdir(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("nested"), 0644)
	os.Mkdir(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "sub", "deep.txt"), []byte("deep"), 0644)

	fs := NewLocal()
	if err := fs.Copy(srcDir, dstDir); err != nil {
		t.Fatalf("Copy dir: %v", err)
	}

	// Verify the deep file was copied.
	data, err := os.ReadFile(filepath.Join(dstDir, "sub", "deep.txt"))
	if err != nil {
		t.Fatalf("read deep file: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("expected 'deep', got %q", string(data))
	}
}

func TestLocalMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "move.txt")
	dst := filepath.Join(dir, "moved.txt")
	os.WriteFile(src, []byte("move me"), 0644)

	fs := NewLocal()
	if err := fs.Move(src, dst); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("expected source to not exist")
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "move me" {
		t.Errorf("expected 'move me', got %q", string(data))
	}
}
