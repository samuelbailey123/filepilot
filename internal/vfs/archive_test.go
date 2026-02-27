package vfs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Compile-time check that Archive implements FileSystem.
var _ FileSystem = (*Archive)(nil)

// ---------------------------------------------------------------------------
// Archive creation helpers used only in tests.
// ---------------------------------------------------------------------------

// newTestZip writes a zip archive containing entries to a temp file and
// returns its path.
func newTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "*.zip")
	if err != nil {
		t.Fatalf("create temp zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %q: %v", name, err)
		}
		if _, err := io.WriteString(fw, content); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return f.Name()
}

// newTestTarGz writes a .tar.gz archive and returns its path.
func newTestTarGz(t *testing.T, entries map[string]string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "*.tar.gz")
	if err != nil {
		t.Fatalf("create temp tar.gz: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	now := time.Now()
	for name, content := range entries {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(data)),
			Mode:     0o644,
			ModTime:  now,
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %q: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("tar data %q: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return f.Name()
}

// newTestTar writes a plain .tar archive and returns its path.
func newTestTar(t *testing.T, entries map[string]string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "*.tar")
	if err != nil {
		t.Fatalf("create temp tar: %v", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	now := time.Now()
	for name, content := range entries {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(data)),
			Mode:     0o644,
			ModTime:  now,
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %q: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("tar data %q: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return f.Name()
}

// newTestTarBz2 writes a .tar.bz2 archive using the system bzip2 binary.
// It skips the calling test if bzip2 is not found on PATH.
func newTestTarBz2(t *testing.T, entries map[string]string) string {
	t.Helper()

	bzip2Bin, err := exec.LookPath("bzip2")
	if err != nil {
		t.Skip("bzip2 binary not found on PATH; skipping tar.bz2 test")
	}

	// Build the inner tar in memory.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	now := time.Now()
	for name, content := range entries {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(data)),
			Mode:     0o644,
			ModTime:  now,
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %q: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("tar data %q: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}

	// Compress with bzip2: echo tarBuf | bzip2 -c > outfile.
	outFile := filepath.Join(t.TempDir(), "test.tar.bz2")
	cmd := exec.Command(bzip2Bin, "-c")
	cmd.Stdin = &tarBuf
	compressed, err := cmd.Output()
	if err != nil {
		t.Fatalf("bzip2 compress: %v", err)
	}
	if err := os.WriteFile(outFile, compressed, 0o600); err != nil {
		t.Fatalf("write tar.bz2: %v", err)
	}
	return outFile
}

// entryNames extracts sorted entry names from a []FileEntry slice.
func entryNames(entries []FileEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Tests for NewArchive / format detection.
// ---------------------------------------------------------------------------

func TestNewArchive_UnsupportedExtension(t *testing.T) {
	_, err := NewArchive("/tmp/nonexistent.rar")
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
}

func TestNewArchive_MissingFile(t *testing.T) {
	_, err := NewArchive(filepath.Join(t.TempDir(), "missing.zip"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ---------------------------------------------------------------------------
// ZIP tests.
// ---------------------------------------------------------------------------

func TestArchiveZip_List_Root(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{
		"readme.txt":        "top-level file",
		"subdir/nested.txt": "nested file",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entries, err := a.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names := entryNames(entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 root entries, got %d: %v", len(names), names)
	}
	if names[0] != "readme.txt" || names[1] != "subdir" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestArchiveZip_List_Subdir(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{
		"readme.txt":              "root",
		"subdir/a.txt":            "a",
		"subdir/b.txt":            "b",
		"subdir/deeper/c.txt":     "c",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entries, err := a.List("subdir")
	if err != nil {
		t.Fatalf("List subdir: %v", err)
	}

	names := entryNames(entries)
	if len(names) != 3 {
		t.Fatalf("expected 3 entries in subdir, got %d: %v", len(names), names)
	}
	// Expect a.txt, b.txt, deeper
	expected := []string{"a.txt", "b.txt", "deeper"}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("entry %d: expected %q, got %q", i, n, names[i])
		}
	}
}

func TestArchiveZip_List_NotFound(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"a.txt": "hello"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	_, err = a.List("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestArchiveZip_List_FileNotDir(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"a.txt": "hello"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	_, err = a.List("a.txt")
	if err == nil {
		t.Fatal("expected error when listing a file, got nil")
	}
}

func TestArchiveZip_Stat_File(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{
		"hello.txt": "hello world",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entry, err := a.Stat("hello.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if entry.Name != "hello.txt" {
		t.Errorf("expected name 'hello.txt', got %q", entry.Name)
	}
	if entry.IsDir {
		t.Error("expected file, got directory")
	}
	if entry.Size != int64(len("hello world")) {
		t.Errorf("expected size %d, got %d", len("hello world"), entry.Size)
	}
}

func TestArchiveZip_Stat_Root(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"a.txt": "x"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entry, err := a.Stat("")
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if !entry.IsDir {
		t.Error("expected root to be a directory")
	}
}

func TestArchiveZip_Stat_NotFound(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"a.txt": "x"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	_, err = a.Stat("does_not_exist.txt")
	if err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

func TestArchiveZip_Stat_SynthesisedDir(t *testing.T) {
	// Archive contains no explicit directory entry; the dir must be synthesised.
	archivePath := newTestZip(t, map[string]string{
		"dir/file.txt": "content",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entry, err := a.Stat("dir")
	if err != nil {
		t.Fatalf("Stat synthesised dir: %v", err)
	}
	if !entry.IsDir {
		t.Error("expected synthesised directory entry to be a dir")
	}
}

func TestArchiveZip_Read(t *testing.T) {
	const want = "file content here"
	archivePath := newTestZip(t, map[string]string{
		"greet.txt": want,
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	rc, err := a.Read("greet.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestArchiveZip_Read_NotFound(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"a.txt": "x"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	_, err = a.Read("missing.txt")
	if err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

func TestArchiveZip_Read_Directory(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"dir/f.txt": "x"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	_, err = a.Read("dir")
	if err == nil {
		t.Fatal("expected error when reading a directory, got nil")
	}
}

// TestArchiveZip_Read_Multiple verifies that Read can be called multiple
// times concurrently without data races (each call reopens the archive file).
func TestArchiveZip_Read_Multiple(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{
		"one.txt": "one",
		"two.txt": "two",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	type result struct {
		content string
		err     error
	}
	ch := make(chan result, 2)

	for _, name := range []string{"one.txt", "two.txt"} {
		name := name
		go func() {
			rc, err := a.Read(name)
			if err != nil {
				ch <- result{err: err}
				return
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			ch <- result{content: string(data), err: err}
		}()
	}

	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err != nil {
			t.Errorf("concurrent Read error: %v", r.err)
		}
	}
}

// ---------------------------------------------------------------------------
// TAR tests.
// ---------------------------------------------------------------------------

func TestArchiveTar_List_Root(t *testing.T) {
	archivePath := newTestTar(t, map[string]string{
		"readme.txt":        "top-level",
		"subdir/nested.txt": "nested",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entries, err := a.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names := entryNames(entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 root entries, got %d: %v", len(names), names)
	}
}

func TestArchiveTar_Read(t *testing.T) {
	const want = "tar file content"
	archivePath := newTestTar(t, map[string]string{
		"data.txt": want,
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	rc, err := a.Read("data.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

// ---------------------------------------------------------------------------
// TAR.GZ tests.
// ---------------------------------------------------------------------------

func TestArchiveTarGz_List(t *testing.T) {
	archivePath := newTestTarGz(t, map[string]string{
		"file1.txt":      "content1",
		"dir/file2.txt":  "content2",
		"dir/file3.txt":  "content3",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	// Root should have "file1.txt" and "dir".
	rootEntries, err := a.List("")
	if err != nil {
		t.Fatalf("List root: %v", err)
	}
	rootNames := entryNames(rootEntries)
	if len(rootNames) != 2 {
		t.Fatalf("expected 2 root entries, got %d: %v", len(rootNames), rootNames)
	}

	// "dir" should have file2.txt and file3.txt.
	dirEntries, err := a.List("dir")
	if err != nil {
		t.Fatalf("List dir: %v", err)
	}
	dirNames := entryNames(dirEntries)
	if len(dirNames) != 2 {
		t.Fatalf("expected 2 entries in dir, got %d: %v", len(dirNames), dirNames)
	}
}

func TestArchiveTarGz_Stat(t *testing.T) {
	archivePath := newTestTarGz(t, map[string]string{
		"hello.txt": "hello gz",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entry, err := a.Stat("hello.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if entry.Name != "hello.txt" {
		t.Errorf("expected 'hello.txt', got %q", entry.Name)
	}
	if entry.IsDir {
		t.Error("expected file, not dir")
	}
}

func TestArchiveTarGz_Read(t *testing.T) {
	const want = "gzipped tar content"
	archivePath := newTestTarGz(t, map[string]string{
		"out.txt": want,
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	rc, err := a.Read("out.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

// ---------------------------------------------------------------------------
// TAR.BZ2 tests.
// ---------------------------------------------------------------------------

func TestArchiveTarBz2_List(t *testing.T) {
	archivePath := newTestTarBz2(t, map[string]string{
		"bz2file.txt":    "bzip2 content",
		"dir/nested.txt": "nested bzip2",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	entries, err := a.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := entryNames(entries)
	if len(names) != 2 {
		t.Fatalf("expected 2 root entries, got %d: %v", len(names), names)
	}
}

func TestArchiveTarBz2_Read(t *testing.T) {
	const want = "bzip2 data"
	archivePath := newTestTarBz2(t, map[string]string{
		"bz2.txt": want,
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	rc, err := a.Read("bz2.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

// ---------------------------------------------------------------------------
// Read-only enforcement tests.
// ---------------------------------------------------------------------------

func TestArchive_WriteOperationsReturnReadOnly(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{"x.txt": "x"})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	t.Run("Write", func(t *testing.T) {
		err := a.Write("new.txt", bytes.NewReader([]byte("data")))
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Mkdir", func(t *testing.T) {
		err := a.Mkdir("newdir")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Remove", func(t *testing.T) {
		err := a.Remove("x.txt")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Rename", func(t *testing.T) {
		err := a.Rename("x.txt", "y.txt")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		err := a.Copy("x.txt", "z.txt")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("Move", func(t *testing.T) {
		err := a.Move("x.txt", "z.txt")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("expected ErrReadOnly, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Path normalisation tests.
// ---------------------------------------------------------------------------

func TestArchive_PathNormalisation(t *testing.T) {
	archivePath := newTestZip(t, map[string]string{
		"a/b/c.txt": "deep",
	})

	a, err := NewArchive(archivePath)
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	cases := []struct {
		input string
		want  string
	}{
		{"a/b/c.txt", "c.txt"},
		{"/a/b/c.txt", "c.txt"},
		{"a/b/../b/c.txt", "c.txt"},
	}

	for _, tc := range cases {
		entry, err := a.Stat(tc.input)
		if err != nil {
			t.Errorf("Stat(%q): %v", tc.input, err)
			continue
		}
		if entry.Name != tc.want {
			t.Errorf("Stat(%q).Name = %q, want %q", tc.input, entry.Name, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TGZ extension alias test.
// ---------------------------------------------------------------------------

func TestArchiveTgz_List(t *testing.T) {
	// Build a tar.gz in memory and write it with a .tgz extension.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte("tgz content")
	_ = tw.WriteHeader(&tar.Header{
		Name:     "tgz.txt",
		Size:     int64(len(data)),
		Mode:     0o644,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(data)
	_ = tw.Close()
	_ = gw.Close()

	f, err := os.CreateTemp(t.TempDir(), "*.tgz")
	if err != nil {
		t.Fatalf("create tgz: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(buf.Bytes()); err != nil {
		t.Fatalf("write tgz: %v", err)
	}
	f.Close()

	a, err := NewArchive(f.Name())
	if err != nil {
		t.Fatalf("NewArchive tgz: %v", err)
	}

	entries, err := a.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "tgz.txt" {
		t.Errorf("unexpected entries: %v", entries)
	}
}
