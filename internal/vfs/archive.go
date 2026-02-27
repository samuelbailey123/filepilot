package vfs

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

// ErrReadOnly is returned by all mutating operations on an Archive because
// archives opened through this VFS are read-only.
var ErrReadOnly = errors.New("archive vfs is read-only")

// archiveFormat enumerates the supported archive types.
type archiveFormat int

const (
	formatZip archiveFormat = iota
	formatTar
	formatTarGz
	formatTarBz2
)

// archiveEntry is the internal representation of a single indexed entry.
type archiveEntry struct {
	path    string // cleaned, relative path (no leading slash)
	name    string // base name
	size    int64
	isDir   bool
	modTime time.Time
	mode    int // unix permission bits
}

// Archive implements FileSystem for zip and tar archives.
// All mutating methods (Write, Mkdir, Remove, Rename, Copy, Move) return
// ErrReadOnly because archives are opened in read-only mode.
//
// Index design: entries stores the full ordered list used for List traversal.
// index maps a cleaned path to the position in entries, avoiding pointer
// aliasing problems that arise when the slice is grown by append.
type Archive struct {
	archivePath string
	format      archiveFormat
	entries     []archiveEntry // ordered list; position is the canonical value
	index       map[string]int // path → index into entries
}

// NewArchive opens and indexes the archive at archivePath. The format is
// detected from the file extension (.zip, .tar, .tar.gz, .tgz, .tar.bz2).
// NewArchive reads the full table of contents into memory so subsequent List
// and Stat calls do not touch the filesystem.
func NewArchive(archivePath string) (*Archive, error) {
	format, err := detectFormat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("detect archive format: %w", err)
	}

	a := &Archive{
		archivePath: archivePath,
		format:      format,
		index:       make(map[string]int),
	}

	switch format {
	case formatZip:
		err = a.indexZip()
	case formatTar, formatTarGz, formatTarBz2:
		err = a.indexTar()
	}
	if err != nil {
		return nil, fmt.Errorf("index archive %s: %w", archivePath, err)
	}

	return a, nil
}

// detectFormat maps a file path to its archiveFormat by inspecting the
// lowercased extension(s). Returns an error for unrecognised extensions.
func detectFormat(archivePath string) (archiveFormat, error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return formatZip, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return formatTarGz, nil
	case strings.HasSuffix(lower, ".tar.bz2"):
		return formatTarBz2, nil
	case strings.HasSuffix(lower, ".tar"):
		return formatTar, nil
	default:
		return 0, fmt.Errorf("unsupported archive extension: %q", path.Ext(archivePath))
	}
}

// normPath cleans a virtual path to a consistent form: no leading slash, no
// trailing slash, and no ".." components. The root is represented as "".
func normPath(p string) string {
	p = path.Clean("/" + p)
	p = strings.TrimPrefix(p, "/")
	return p
}

// addEntry appends an entry to the ordered list and records its index in the
// lookup map. It then synthesises parent directory entries for every ancestor
// that has not already been recorded.
func (a *Archive) addEntry(e archiveEntry) {
	e.path = normPath(e.path)
	if _, exists := a.index[e.path]; !exists {
		a.index[e.path] = len(a.entries)
		a.entries = append(a.entries, e)
	}
	a.ensureParents(e.path, e.modTime)
}

// ensureParents walks up the path hierarchy and synthesises directory entries
// for any ancestor that is not yet present in the index.
func (a *Archive) ensureParents(p string, modTime time.Time) {
	for {
		parent := path.Dir(p)
		if parent == "." || parent == p {
			break
		}
		if _, exists := a.index[parent]; !exists {
			dir := archiveEntry{
				path:    parent,
				name:    path.Base(parent),
				isDir:   true,
				modTime: modTime,
				mode:    0o755,
			}
			a.index[parent] = len(a.entries)
			a.entries = append(a.entries, dir)
		}
		p = parent
	}
}

// indexZip reads the central directory of the zip archive and populates the
// in-memory index.
func (a *Archive) indexZip() error {
	info, err := os.Stat(a.archivePath)
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}

	f, err := os.Open(a.archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fmt.Errorf("parse zip: %w", err)
	}

	for _, zf := range zr.File {
		fi := zf.FileInfo()
		cleanName := normPath(zf.Name)
		baseName := path.Base(cleanName)
		// zip entries for explicit directories end in "/" which normPath strips,
		// leaving an empty base when the path itself also normalises to "".
		// Guard against that edge case.
		if baseName == "" || baseName == "." {
			baseName = cleanName
		}
		e := archiveEntry{
			path:    cleanName,
			name:    baseName,
			size:    fi.Size(),
			isDir:   fi.IsDir(),
			modTime: zf.Modified,
			mode:    int(fi.Mode().Perm()),
		}
		a.addEntry(e)
	}

	return nil
}

// indexTar reads all tar headers and populates the in-memory index.
func (a *Archive) indexTar() error {
	tr, closer, err := a.openTar()
	if err != nil {
		return err
	}
	defer closer.Close()

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		cleanName := normPath(hdr.Name)
		e := archiveEntry{
			path:    cleanName,
			name:    path.Base(cleanName),
			size:    hdr.Size,
			isDir:   hdr.Typeflag == tar.TypeDir,
			modTime: hdr.ModTime,
			mode:    int(hdr.Mode & 0o777),
		}
		a.addEntry(e)
	}

	return nil
}

// openTar opens the archive file, wraps it in the appropriate decompressor,
// and returns a *tar.Reader along with a Closer that must be called when done.
func (a *Archive) openTar() (*tar.Reader, io.Closer, error) {
	f, err := os.Open(a.archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open archive: %w", err)
	}

	switch a.format {
	case formatTarGz:
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("create gzip reader: %w", err)
		}
		// multiCloser closes gr before f so that gzip can flush any state.
		mc := &multiCloser{closers: []io.Closer{gr, f}}
		return tar.NewReader(gr), mc, nil

	case formatTarBz2:
		// compress/bzip2 provides a decode-only reader with no Close method.
		br := bzip2.NewReader(f)
		return tar.NewReader(br), f, nil

	default: // formatTar
		return tar.NewReader(f), f, nil
	}
}

// multiCloser closes a sequence of io.Closers in order.
type multiCloser struct {
	closers []io.Closer
}

func (m *multiCloser) Close() error {
	var first error
	for _, c := range m.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// List returns the direct children at the virtual path inside the archive.
// Pass "" or "." for the root. Returns an error if the path does not exist or
// is not a directory.
func (a *Archive) List(p string) ([]FileEntry, error) {
	p = normPath(p)

	// The root "" is always valid. Any other path must be an indexed directory.
	if p != "" {
		idx, ok := a.index[p]
		if !ok {
			return nil, fmt.Errorf("path not found in archive: %q", p)
		}
		if !a.entries[idx].isDir {
			return nil, fmt.Errorf("not a directory: %q", p)
		}
	}

	var results []FileEntry
	for i := range a.entries {
		e := &a.entries[i]
		parent := path.Dir(e.path)
		if parent == "." {
			parent = ""
		}
		// Only emit direct children of p; skip p itself if it appears as an
		// explicit entry (e.g. an explicit directory record in a zip file).
		if parent != p || e.path == p {
			continue
		}
		results = append(results, toFileEntry(e))
	}
	return results, nil
}

// Stat returns metadata for a single entry within the archive. Pass "" or "."
// for the root directory, which always succeeds regardless of archive content.
func (a *Archive) Stat(p string) (*FileEntry, error) {
	p = normPath(p)

	// The root is always present as a virtual directory.
	if p == "" {
		fe := FileEntry{
			Path:        "",
			Name:        ".",
			IsDir:       true,
			ModTime:     time.Time{},
			Permissions: 0o755,
		}
		return &fe, nil
	}

	idx, ok := a.index[p]
	if !ok {
		return nil, fmt.Errorf("path not found in archive: %q", p)
	}
	fe := toFileEntry(&a.entries[idx])
	return &fe, nil
}

// Read returns an io.ReadCloser for the named entry. The caller is responsible
// for closing the returned reader, which also closes underlying file handles.
func (a *Archive) Read(p string) (io.ReadCloser, error) {
	p = normPath(p)

	idx, ok := a.index[p]
	if !ok {
		return nil, fmt.Errorf("path not found in archive: %q", p)
	}
	if a.entries[idx].isDir {
		return nil, fmt.Errorf("cannot read a directory: %q", p)
	}

	switch a.format {
	case formatZip:
		return a.readZip(p)
	default:
		return a.readTar(p)
	}
}

// readZip opens the zip file, locates the entry by path, and returns a reader
// that closes both the zip entry and the underlying file when done.
func (a *Archive) readZip(p string) (io.ReadCloser, error) {
	info, err := os.Stat(a.archivePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive: %w", err)
	}

	f, err := os.Open(a.archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("parse zip: %w", err)
	}

	for _, zf := range zr.File {
		if normPath(zf.Name) != p {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("open zip entry %q: %w", p, err)
		}
		// Wrap so that closing the returned reader also closes f.
		return &zipEntryReader{ReadCloser: rc, file: f}, nil
	}

	f.Close()
	return nil, fmt.Errorf("entry not found in zip: %q", p)
}

// zipEntryReader wraps a zip entry ReadCloser and closes the underlying
// archive file when Close is called.
type zipEntryReader struct {
	io.ReadCloser
	file *os.File
}

func (z *zipEntryReader) Close() error {
	err1 := z.ReadCloser.Close()
	err2 := z.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// readTar opens the tar archive, seeks to the named entry, and returns a
// reader that streams its content. A goroutine copies the tar data through an
// io.Pipe so all underlying handles are closed after the data is consumed.
func (a *Archive) readTar(p string) (io.ReadCloser, error) {
	tr, closer, err := a.openTar()
	if err != nil {
		return nil, err
	}

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			closer.Close()
			return nil, fmt.Errorf("entry not found in tar: %q", p)
		}
		if err != nil {
			closer.Close()
			return nil, fmt.Errorf("read tar header: %w", err)
		}
		if normPath(hdr.Name) != p {
			continue
		}
		// Pipe the entry data out so the caller receives a clean ReadCloser.
		// The goroutine closes all underlying handles once copying is complete
		// or the pipe reader is closed (triggering a pipe error on the write
		// side, which the goroutine handles safely).
		pr, pw := io.Pipe()
		go func() {
			_, copyErr := io.Copy(pw, tr)
			pw.CloseWithError(copyErr)
			closer.Close()
		}()
		return pr, nil
	}
}

// Write always returns ErrReadOnly.
func (a *Archive) Write(_ string, _ io.Reader) error { return ErrReadOnly }

// Mkdir always returns ErrReadOnly.
func (a *Archive) Mkdir(_ string) error { return ErrReadOnly }

// Remove always returns ErrReadOnly.
func (a *Archive) Remove(_ string) error { return ErrReadOnly }

// Rename always returns ErrReadOnly.
func (a *Archive) Rename(_, _ string) error { return ErrReadOnly }

// Copy always returns ErrReadOnly.
func (a *Archive) Copy(_, _ string) error { return ErrReadOnly }

// Move always returns ErrReadOnly.
func (a *Archive) Move(_, _ string) error { return ErrReadOnly }

// toFileEntry converts an internal archiveEntry to the public FileEntry type.
func toFileEntry(e *archiveEntry) FileEntry {
	return FileEntry{
		Path:        e.path,
		Name:        e.name,
		Size:        e.size,
		IsDir:       e.isDir,
		ModTime:     e.modTime,
		Permissions: e.mode,
	}
}
