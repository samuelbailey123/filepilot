package preview

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxArchiveEntries is the maximum number of file entries to list.
const maxArchiveEntries = 20

// previewZip lists entries in a zip archive, returning up to
// maxArchiveEntries names and the total count.
func previewZip(path string) (entries []string, total int, err error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	total = len(r.File)
	for i, f := range r.File {
		if i >= maxArchiveEntries {
			break
		}
		size := f.UncompressedSize64
		entries = append(entries, fmt.Sprintf("%s  (%s)", f.Name, formatArchiveSize(size)))
	}
	return entries, total, nil
}

// previewTar lists entries in a tar or tar.gz archive, returning up to
// maxArchiveEntries names and the total count.
func previewTar(path string) (entries []string, total int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open tar: %w", err)
	}
	defer f.Close()

	var reader io.Reader = f

	// Detect gzip by extension.
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, 0, fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Return what we have so far on partial read errors.
			if total > 0 {
				break
			}
			return nil, 0, fmt.Errorf("read tar: %w", err)
		}
		total++
		if len(entries) < maxArchiveEntries {
			entries = append(entries, fmt.Sprintf("%s  (%s)", hdr.Name, formatArchiveSize(uint64(hdr.Size))))
		}
	}
	return entries, total, nil
}

// formatArchiveSize formats a byte count for display in archive listings.
func formatArchiveSize(bytes uint64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
