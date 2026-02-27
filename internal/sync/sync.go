// Package sync implements folder comparison and bidirectional synchronization across filesystems.
package sync

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"filepilot/internal/vfs"
)

// DiffStatus describes the relationship between a file entry on the left and right side.
type DiffStatus string

const (
	// StatusLeftOnly means the entry exists only in the left filesystem.
	StatusLeftOnly DiffStatus = "left_only"

	// StatusRightOnly means the entry exists only in the right filesystem.
	StatusRightOnly DiffStatus = "right_only"

	// StatusNewerLeft means the entry exists on both sides but the left copy is newer.
	StatusNewerLeft DiffStatus = "newer_left"

	// StatusNewerRight means the entry exists on both sides but the right copy is newer.
	StatusNewerRight DiffStatus = "newer_right"

	// StatusIdentical means the entry exists on both sides and appears unchanged.
	StatusIdentical DiffStatus = "identical"

	// StatusConflict means the entry exists on both sides but cannot be automatically resolved.
	// This is reserved for future bidirectional conflict detection.
	StatusConflict DiffStatus = "conflict"
)

// modTimeTolerance is the minimum time difference required to consider two files as having
// different modification times. Filesystem precision varies across protocols and platforms.
const modTimeTolerance = time.Second

// DiffEntry describes a single path in the comparison result.
type DiffEntry struct {
	// Path is the relative path from the respective root, using forward slashes.
	Path string

	// Name is the base name of the entry.
	Name string

	// Status is the relationship between the left and right copies of this entry.
	Status DiffStatus

	// LeftEntry is the file metadata on the left side. Nil if not present on the left.
	LeftEntry *vfs.FileEntry

	// RightEntry is the file metadata on the right side. Nil if not present on the right.
	RightEntry *vfs.FileEntry

	// IsDir indicates whether this entry is a directory.
	IsDir bool
}

// DiffOptions controls the behaviour of Compare.
type DiffOptions struct {
	// IgnoreHidden skips entries whose name begins with a dot.
	IgnoreHidden bool

	// CompareContent requests byte-level comparison of file bodies to determine identity.
	// This field is reserved for future use; the current implementation compares size and modtime.
	CompareContent bool
}

// SyncDirection specifies which side drives the sync operation.
type SyncDirection string

const (
	// SyncLeftToRight propagates changes from left to right.
	SyncLeftToRight SyncDirection = "left_to_right"

	// SyncRightToLeft propagates changes from right to left.
	SyncRightToLeft SyncDirection = "right_to_left"

	// SyncBidirectional applies the newest version to both sides.
	SyncBidirectional SyncDirection = "bidirectional"
)

// OpType classifies the operation a SyncOp represents.
type OpType string

const (
	// OpCopy transfers a file from SourcePath on SourceFS to DestPath on DestFS.
	OpCopy OpType = "copy"

	// OpDelete removes a file at DestPath on DestFS.
	OpDelete OpType = "delete"
)

// SyncOp is a single, atomic operation produced by PlanSync.
type SyncOp struct {
	// Type is the kind of operation to perform.
	Type OpType

	// SourceFS is the filesystem from which to read. Nil for delete operations.
	SourceFS vfs.FileSystem

	// SourcePath is the path on SourceFS. Empty for delete operations.
	SourcePath string

	// DestFS is the filesystem on which to write or delete.
	DestFS vfs.FileSystem

	// DestPath is the path on DestFS.
	DestPath string
}

// SyncPlan groups the filtered diff entries and the operations derived from them.
type SyncPlan struct {
	// Entries are the diff entries that require action under the chosen direction.
	Entries []DiffEntry

	// Direction is the sync direction used to build this plan.
	Direction SyncDirection

	// Ops is the ordered list of operations to execute.
	Ops []SyncOp
}

// Compare walks leftPath on leftFS and rightPath on rightFS, then returns a flat, sorted list
// of DiffEntry values describing every path seen on either side. Directories are walked
// recursively. The returned paths are relative to the respective roots.
func Compare(
	leftFS vfs.FileSystem,
	leftPath string,
	rightFS vfs.FileSystem,
	rightPath string,
	opts DiffOptions,
) ([]DiffEntry, error) {
	leftMap := make(map[string]vfs.FileEntry)
	if err := walk(leftFS, leftPath, "", opts, leftMap); err != nil {
		return nil, fmt.Errorf("walk left %s: %w", leftPath, err)
	}

	rightMap := make(map[string]vfs.FileEntry)
	if err := walk(rightFS, rightPath, "", opts, rightMap); err != nil {
		return nil, fmt.Errorf("walk right %s: %w", rightPath, err)
	}

	// Collect all relative paths from both sides.
	seen := make(map[string]struct{}, len(leftMap)+len(rightMap))
	for rel := range leftMap {
		seen[rel] = struct{}{}
	}
	for rel := range rightMap {
		seen[rel] = struct{}{}
	}

	entries := make([]DiffEntry, 0, len(seen))
	for rel := range seen {
		left, hasLeft := leftMap[rel]
		right, hasRight := rightMap[rel]

		entry := DiffEntry{
			Path: rel,
			Name: path.Base(rel),
		}

		switch {
		case hasLeft:
			entry.IsDir = left.IsDir
		case hasRight:
			entry.IsDir = right.IsDir
		}

		if hasLeft {
			cp := left
			entry.LeftEntry = &cp
		}
		if hasRight {
			cp := right
			entry.RightEntry = &cp
		}

		entry.Status = classify(hasLeft, hasRight, left, right)
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

// PlanSync converts a diff into a concrete list of SyncOp values given a direction.
// The leftFS and rightFS parameters are the same filesystems passed to Compare so that
// SyncOp can reference them directly.
func PlanSync(
	diff []DiffEntry,
	direction SyncDirection,
	leftFS vfs.FileSystem,
	leftRoot string,
	rightFS vfs.FileSystem,
	rightRoot string,
) []SyncOp {
	ops := make([]SyncOp, 0, len(diff))

	for _, entry := range diff {
		leftAbs := path.Join(leftRoot, entry.Path)
		rightAbs := path.Join(rightRoot, entry.Path)

		switch direction {
		case SyncLeftToRight:
			op, ok := planLeftToRight(entry, leftFS, leftAbs, rightFS, rightAbs)
			if ok {
				ops = append(ops, op)
			}

		case SyncRightToLeft:
			op, ok := planRightToLeft(entry, leftFS, leftAbs, rightFS, rightAbs)
			if ok {
				ops = append(ops, op)
			}

		case SyncBidirectional:
			op, ok := planBidirectional(entry, leftFS, leftAbs, rightFS, rightAbs)
			if ok {
				ops = append(ops, op)
			}
		}
	}

	return ops
}

// planLeftToRight returns the operation needed to make the right side match the left side.
func planLeftToRight(
	entry DiffEntry,
	leftFS vfs.FileSystem, leftAbs string,
	rightFS vfs.FileSystem, rightAbs string,
) (SyncOp, bool) {
	switch entry.Status {
	case StatusLeftOnly, StatusNewerLeft:
		return SyncOp{
			Type:       OpCopy,
			SourceFS:   leftFS,
			SourcePath: leftAbs,
			DestFS:     rightFS,
			DestPath:   rightAbs,
		}, true
	case StatusRightOnly:
		return SyncOp{
			Type:     OpDelete,
			DestFS:   rightFS,
			DestPath: rightAbs,
		}, true
	}
	return SyncOp{}, false
}

// planRightToLeft returns the operation needed to make the left side match the right side.
func planRightToLeft(
	entry DiffEntry,
	leftFS vfs.FileSystem, leftAbs string,
	rightFS vfs.FileSystem, rightAbs string,
) (SyncOp, bool) {
	switch entry.Status {
	case StatusRightOnly, StatusNewerRight:
		return SyncOp{
			Type:       OpCopy,
			SourceFS:   rightFS,
			SourcePath: rightAbs,
			DestFS:     leftFS,
			DestPath:   leftAbs,
		}, true
	case StatusLeftOnly:
		return SyncOp{
			Type:     OpDelete,
			DestFS:   leftFS,
			DestPath: leftAbs,
		}, true
	}
	return SyncOp{}, false
}

// planBidirectional propagates whichever version is newer to the opposite side.
// Conflicts and identical files are skipped.
func planBidirectional(
	entry DiffEntry,
	leftFS vfs.FileSystem, leftAbs string,
	rightFS vfs.FileSystem, rightAbs string,
) (SyncOp, bool) {
	switch entry.Status {
	case StatusLeftOnly, StatusNewerLeft:
		return SyncOp{
			Type:       OpCopy,
			SourceFS:   leftFS,
			SourcePath: leftAbs,
			DestFS:     rightFS,
			DestPath:   rightAbs,
		}, true
	case StatusRightOnly, StatusNewerRight:
		return SyncOp{
			Type:       OpCopy,
			SourceFS:   rightFS,
			SourcePath: rightAbs,
			DestFS:     leftFS,
			DestPath:   leftAbs,
		}, true
	}
	return SyncOp{}, false
}

// walk recursively lists all entries under root on fs, recording them into dst keyed by their
// path relative to root. The rel parameter tracks the current relative prefix.
func walk(fs vfs.FileSystem, root, rel string, opts DiffOptions, dst map[string]vfs.FileEntry) error {
	abs := root
	if rel != "" {
		abs = path.Join(root, rel)
	}

	entries, err := fs.List(abs)
	if err != nil {
		return fmt.Errorf("list %s: %w", abs, err)
	}

	for _, e := range entries {
		if opts.IgnoreHidden && strings.HasPrefix(e.Name, ".") {
			continue
		}

		relPath := e.Name
		if rel != "" {
			relPath = path.Join(rel, e.Name)
		}

		dst[relPath] = e

		if e.IsDir {
			if err := walk(fs, root, relPath, opts, dst); err != nil {
				return err
			}
		}
	}

	return nil
}

// classify determines the DiffStatus for a single relative path given its presence and metadata
// on both sides.
func classify(hasLeft, hasRight bool, left, right vfs.FileEntry) DiffStatus {
	switch {
	case hasLeft && !hasRight:
		return StatusLeftOnly

	case !hasLeft && hasRight:
		return StatusRightOnly

	default:
		// Both sides have the entry.
		if left.IsDir && right.IsDir {
			// Directories themselves are not compared by content.
			return StatusIdentical
		}

		if left.Size == right.Size {
			diff := left.ModTime.Sub(right.ModTime)
			if diff < 0 {
				diff = -diff
			}
			if diff <= modTimeTolerance {
				return StatusIdentical
			}
		}

		if left.ModTime.After(right.ModTime.Add(modTimeTolerance)) {
			return StatusNewerLeft
		}
		if right.ModTime.After(left.ModTime.Add(modTimeTolerance)) {
			return StatusNewerRight
		}

		// Same modtime but different size — treat as conflict.
		return StatusConflict
	}
}
