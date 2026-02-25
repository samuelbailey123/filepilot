package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// ScanProgress reports the current state of a filesystem scan.
type ScanProgress struct {
	Scanned   int64  `json:"scanned"`
	Indexed   int64  `json:"indexed"`
	Current   string `json:"current"`
	Running   bool   `json:"running"`
	StartedAt int64  `json:"startedAt"`
	Error     string `json:"error,omitempty"`
}

// Scanner performs breadth-first filesystem indexing.
type Scanner struct {
	idx       *Index
	exclude   map[string]bool
	progress  atomic.Value
	batchSize int
	stopCh    chan struct{}
}

// defaultExclusions returns paths and directory names that should be skipped.
func defaultExclusions() map[string]bool {
	return map[string]bool{
		// System directories
		"/System":                 true,
		"/Library":                true,
		"/private":                true,
		"/usr":                    true,
		"/bin":                    true,
		"/sbin":                   true,
		"/var":                    true,
		"/cores":                  true,
		"/opt":                    true,
		"/.Spotlight-V100":        true,
		"/.fseventsd":             true,
		"/.Trashes":               true,
		"/.vol":                   true,
		"/dev":                    true,
		"/tmp":                    true,
		"/etc":                    true,
		// Common large/uninteresting directories (by name)
		"node_modules":            true,
		".git/objects":            true,
		".Spotlight-V100":         true,
		".fseventsd":              true,
		".Trashes":                true,
		"__pycache__":             true,
		".cache":                  true,
		"DerivedData":             true,
		"Library/Caches":          true,
		"Library/Application Support/Google/Chrome": true,
	}
}

// NewScanner creates a scanner that indexes files into the given Index.
func NewScanner(idx *Index) *Scanner {
	s := &Scanner{
		idx:       idx,
		exclude:   defaultExclusions(),
		batchSize: 1000,
		stopCh:    make(chan struct{}),
	}
	s.progress.Store(ScanProgress{})
	return s
}

// Progress returns the current scan progress.
func (s *Scanner) Progress() ScanProgress {
	return s.progress.Load().(ScanProgress)
}

// Stop signals the scanner to stop.
func (s *Scanner) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// Scan performs a breadth-first scan of the given root directories.
func (s *Scanner) Scan(roots ...string) error {
	s.stopCh = make(chan struct{})

	startTime := time.Now().Unix()
	var scanned, indexed int64

	s.progress.Store(ScanProgress{Running: true, StartedAt: startTime})

	// BFS queue
	queue := make([]string, 0, len(roots))
	queue = append(queue, roots...)

	batch := make([]FileEntry, 0, s.batchSize)

	for len(queue) > 0 {
		select {
		case <-s.stopCh:
			// Flush remaining batch before stopping.
			if len(batch) > 0 {
				s.idx.UpsertBatch(batch)
				indexed += int64(len(batch))
			}
			s.progress.Store(ScanProgress{
				Scanned: scanned, Indexed: indexed,
				Running: false, StartedAt: startTime,
			})
			return nil
		default:
		}

		dir := queue[0]
		queue = queue[1:]

		if s.shouldSkip(dir) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		s.progress.Store(ScanProgress{
			Scanned: scanned, Indexed: indexed,
			Current: dir, Running: true, StartedAt: startTime,
		})

		for _, entry := range entries {
			name := entry.Name()
			fullPath := filepath.Join(dir, name)

			if s.shouldSkipName(name) || s.shouldSkip(fullPath) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			scanned++

			fe := FileEntry{
				Path:       fullPath,
				Name:       name,
				ParentPath: dir,
				Extension:  strings.TrimPrefix(filepath.Ext(name), "."),
				Size:       info.Size(),
				IsDir:      info.IsDir(),
				ModTime:    info.ModTime().Unix(),
				Permissions: int(info.Mode().Perm()),
				Hidden:     strings.HasPrefix(name, "."),
			}

			batch = append(batch, fe)

			if len(batch) >= s.batchSize {
				if err := s.idx.UpsertBatch(batch); err != nil {
					s.progress.Store(ScanProgress{
						Scanned: scanned, Indexed: indexed,
						Running: false, StartedAt: startTime,
						Error: fmt.Sprintf("batch upsert: %v", err),
					})
					return fmt.Errorf("batch upsert: %w", err)
				}
				indexed += int64(len(batch))
				batch = batch[:0]
			}

			if info.IsDir() {
				queue = append(queue, fullPath)
			}
		}
	}

	// Flush remaining batch.
	if len(batch) > 0 {
		if err := s.idx.UpsertBatch(batch); err != nil {
			return fmt.Errorf("final batch upsert: %w", err)
		}
		indexed += int64(len(batch))
	}

	s.progress.Store(ScanProgress{
		Scanned: scanned, Indexed: indexed,
		Running: false, StartedAt: startTime,
	})

	return nil
}

// shouldSkip checks if a full path should be excluded.
func (s *Scanner) shouldSkip(path string) bool {
	return s.exclude[path]
}

// shouldSkipName checks if a directory/file name should be excluded.
func (s *Scanner) shouldSkipName(name string) bool {
	return s.exclude[name]
}
