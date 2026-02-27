package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"filepilot/internal/vfs"
)

// writeFile creates a file at dst/name with the given content and modtime.
func writeFile(t *testing.T, dir, name, content string, modtime time.Time) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", p, err)
	}
	if err := os.Chtimes(p, modtime, modtime); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	return p
}

// makeDir creates a subdirectory inside dir.
func makeDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Mkdir(p, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	return p
}

// findEntry returns the DiffEntry for relPath, or fails the test if it is not found.
func findEntry(t *testing.T, diff []DiffEntry, relPath string) DiffEntry {
	t.Helper()
	for _, e := range diff {
		if e.Path == relPath {
			return e
		}
	}
	t.Fatalf("entry %q not found in diff (have %d entries)", relPath, len(diff))
	return DiffEntry{}
}

// base is a fixed reference time used across all test fixtures.
var base = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

func TestCompare_IdenticalDirectories(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, left, "a.txt", "hello", base)
	writeFile(t, right, "a.txt", "hello", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if len(diff) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(diff))
	}

	e := diff[0]
	if e.Path != "a.txt" {
		t.Errorf("path = %q, want %q", e.Path, "a.txt")
	}
	if e.Status != StatusIdentical {
		t.Errorf("status = %q, want %q", e.Status, StatusIdentical)
	}
	if e.LeftEntry == nil || e.RightEntry == nil {
		t.Error("expected both LeftEntry and RightEntry to be non-nil")
	}
}

func TestCompare_LeftOnlyAndRightOnly(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, left, "left_only.txt", "only on left", base)
	writeFile(t, right, "right_only.txt", "only on right", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if len(diff) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(diff))
	}

	lo := findEntry(t, diff, "left_only.txt")
	if lo.Status != StatusLeftOnly {
		t.Errorf("left_only.txt status = %q, want %q", lo.Status, StatusLeftOnly)
	}
	if lo.LeftEntry == nil {
		t.Error("left_only.txt: LeftEntry must not be nil")
	}
	if lo.RightEntry != nil {
		t.Error("left_only.txt: RightEntry must be nil")
	}

	ro := findEntry(t, diff, "right_only.txt")
	if ro.Status != StatusRightOnly {
		t.Errorf("right_only.txt status = %q, want %q", ro.Status, StatusRightOnly)
	}
	if ro.RightEntry == nil {
		t.Error("right_only.txt: RightEntry must not be nil")
	}
	if ro.LeftEntry != nil {
		t.Error("right_only.txt: LeftEntry must be nil")
	}
}

func TestCompare_ModifiedFiles(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	// Same name, different size — left is newer.
	newer := base.Add(2 * time.Second)
	writeFile(t, left, "changed.txt", "longer content on left", newer)
	writeFile(t, right, "changed.txt", "short", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	e := findEntry(t, diff, "changed.txt")
	if e.Status != StatusNewerLeft {
		t.Errorf("changed.txt status = %q, want %q", e.Status, StatusNewerLeft)
	}
	if e.LeftEntry == nil || e.RightEntry == nil {
		t.Error("expected both entries to be present")
	}
}

func TestCompare_RightNewer(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	newer := base.Add(5 * time.Second)
	writeFile(t, left, "file.txt", "old", base)
	writeFile(t, right, "file.txt", "new content here", newer)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	e := findEntry(t, diff, "file.txt")
	if e.Status != StatusNewerRight {
		t.Errorf("file.txt status = %q, want %q", e.Status, StatusNewerRight)
	}
}

func TestCompare_Recursive(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	makeDir(t, left, "subdir")
	makeDir(t, right, "subdir")

	writeFile(t, filepath.Join(left, "subdir"), "nested.txt", "content", base)
	writeFile(t, filepath.Join(right, "subdir"), "nested.txt", "content", base)

	// An extra file only on the left.
	writeFile(t, filepath.Join(left, "subdir"), "extra.txt", "extra", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Expect: subdir (dir, identical), subdir/nested.txt (identical), subdir/extra.txt (left_only)
	nested := findEntry(t, diff, "subdir/nested.txt")
	if nested.Status != StatusIdentical {
		t.Errorf("subdir/nested.txt status = %q, want identical", nested.Status)
	}

	extra := findEntry(t, diff, "subdir/extra.txt")
	if extra.Status != StatusLeftOnly {
		t.Errorf("subdir/extra.txt status = %q, want left_only", extra.Status)
	}
}

func TestCompare_SortedByPath(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	for _, name := range []string{"z.txt", "a.txt", "m.txt"} {
		writeFile(t, left, name, name, base)
		writeFile(t, right, name, name, base)
	}

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	for i := 1; i < len(diff); i++ {
		if diff[i].Path < diff[i-1].Path {
			t.Errorf("diff not sorted: %q before %q", diff[i-1].Path, diff[i].Path)
		}
	}
}

func TestCompare_IgnoreHidden(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, left, "visible.txt", "hello", base)
	writeFile(t, left, ".hidden", "secret", base)

	fs := vfs.NewLocal()

	t.Run("hidden included by default", func(t *testing.T) {
		diff, err := Compare(fs, left, fs, right, DiffOptions{})
		if err != nil {
			t.Fatalf("Compare: %v", err)
		}
		paths := make(map[string]bool, len(diff))
		for _, e := range diff {
			paths[e.Path] = true
		}
		if !paths[".hidden"] {
			t.Error("expected .hidden to appear in diff when IgnoreHidden is false")
		}
	})

	t.Run("hidden excluded when IgnoreHidden set", func(t *testing.T) {
		diff, err := Compare(fs, left, fs, right, DiffOptions{IgnoreHidden: true})
		if err != nil {
			t.Fatalf("Compare: %v", err)
		}
		for _, e := range diff {
			if e.Path == ".hidden" {
				t.Error("found .hidden in diff, expected it to be excluded")
			}
		}
		// visible.txt must still appear.
		found := false
		for _, e := range diff {
			if e.Path == "visible.txt" {
				found = true
			}
		}
		if !found {
			t.Error("visible.txt missing from diff after IgnoreHidden")
		}
	})
}

func TestPlanSync_LeftToRight(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, left, "new.txt", "new file", base)
	writeFile(t, left, "updated.txt", "updated", base.Add(3*time.Second))
	writeFile(t, right, "updated.txt", "old", base)
	writeFile(t, right, "stale.txt", "stale on right", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	ops := PlanSync(diff, SyncLeftToRight, fs, left, fs, right)

	type opKey struct {
		typ  OpType
		dest string
	}
	opMap := make(map[string]SyncOp, len(ops))
	for _, op := range ops {
		opMap[op.DestPath] = op
	}

	newDest := filepath.Join(right, "new.txt")
	if op, ok := opMap[newDest]; !ok || op.Type != OpCopy {
		t.Errorf("expected OpCopy for new.txt, got %+v (found=%v)", opMap[newDest], ok)
	}

	updatedDest := filepath.Join(right, "updated.txt")
	if op, ok := opMap[updatedDest]; !ok || op.Type != OpCopy {
		t.Errorf("expected OpCopy for updated.txt, got %+v (found=%v)", opMap[updatedDest], ok)
	}

	staleDest := filepath.Join(right, "stale.txt")
	if op, ok := opMap[staleDest]; !ok || op.Type != OpDelete {
		t.Errorf("expected OpDelete for stale.txt, got %+v (found=%v)", opMap[staleDest], ok)
	}
}

func TestPlanSync_RightToLeft(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, right, "incoming.txt", "from right", base)
	writeFile(t, left, "leftonly.txt", "only left", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	ops := PlanSync(diff, SyncRightToLeft, fs, left, fs, right)

	opMap := make(map[string]SyncOp, len(ops))
	for _, op := range ops {
		opMap[op.DestPath] = op
	}

	incomingDest := filepath.Join(left, "incoming.txt")
	if op, ok := opMap[incomingDest]; !ok || op.Type != OpCopy {
		t.Errorf("expected OpCopy for incoming.txt, got %+v (found=%v)", opMap[incomingDest], ok)
	}
	if op := opMap[incomingDest]; op.SourcePath != filepath.Join(right, "incoming.txt") {
		t.Errorf("SourcePath = %q, want right-side path", op.SourcePath)
	}

	leftonlyDest := filepath.Join(left, "leftonly.txt")
	if op, ok := opMap[leftonlyDest]; !ok || op.Type != OpDelete {
		t.Errorf("expected OpDelete for leftonly.txt, got %+v (found=%v)", opMap[leftonlyDest], ok)
	}
}

func TestPlanSync_IdenticalFilesProduceNoOps(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeFile(t, left, "same.txt", "same", base)
	writeFile(t, right, "same.txt", "same", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	for _, dir := range []SyncDirection{SyncLeftToRight, SyncRightToLeft, SyncBidirectional} {
		ops := PlanSync(diff, dir, fs, left, fs, right)
		if len(ops) != 0 {
			t.Errorf("direction %q: expected no ops for identical files, got %d", dir, len(ops))
		}
	}
}

func TestPlanSync_Bidirectional(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	// left has a newer version; right has a file that does not exist on left.
	writeFile(t, left, "left_newer.txt", "updated left", base.Add(5*time.Second))
	writeFile(t, right, "left_newer.txt", "old right", base)
	writeFile(t, right, "right_only.txt", "right only", base)

	fs := vfs.NewLocal()
	diff, err := Compare(fs, left, fs, right, DiffOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	ops := PlanSync(diff, SyncBidirectional, fs, left, fs, right)

	opMap := make(map[string]SyncOp, len(ops))
	for _, op := range ops {
		opMap[op.DestPath] = op
	}

	// left_newer.txt should be copied to right.
	newerDest := filepath.Join(right, "left_newer.txt")
	if op, ok := opMap[newerDest]; !ok || op.Type != OpCopy {
		t.Errorf("expected OpCopy for left_newer.txt to right, got %+v (found=%v)", opMap[newerDest], ok)
	}

	// right_only.txt should be copied to left.
	rightOnlyDest := filepath.Join(left, "right_only.txt")
	if op, ok := opMap[rightOnlyDest]; !ok || op.Type != OpCopy {
		t.Errorf("expected OpCopy for right_only.txt to left, got %+v (found=%v)", opMap[rightOnlyDest], ok)
	}
}
