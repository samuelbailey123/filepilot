package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// checkGitAvailable skips the test if the git binary is not on PATH.
func checkGitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH: skipping test")
	}
}

// initRepo creates a temporary directory, runs `git init` inside it, and
// configures a local user identity so commits and staging work without a
// global git config being required.
func initRepo(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-status-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")
	run("git", "config", "commit.gpgsign", "false")

	return dir
}

// writeFile creates a file with the given content inside dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
	return path
}

// runGit runs a git command inside dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestIsGitRepo_InsideRepo verifies that IsGitRepo returns true for a directory
// that has been initialised with `git init`.
func TestIsGitRepo_InsideRepo(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)

	if !IsGitRepo(dir) {
		t.Errorf("IsGitRepo(%q) = false, want true", dir)
	}
}

// TestIsGitRepo_Subdirectory verifies that IsGitRepo returns true for a
// subdirectory inside a git repository, not just the root.
func TestIsGitRepo_Subdirectory(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	subdir := filepath.Join(dir, "subdir", "nested")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	if !IsGitRepo(subdir) {
		t.Errorf("IsGitRepo(%q) = false, want true for nested subdir", subdir)
	}
}

// TestIsGitRepo_NonGitDirectory verifies that IsGitRepo returns false for a
// plain directory that has not been initialised as a git repository.
func TestIsGitRepo_NonGitDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "not-a-git-repo-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if IsGitRepo(dir) {
		t.Errorf("IsGitRepo(%q) = true, want false for non-git directory", dir)
	}
}

// TestFindRepoRoot verifies that FindRepoRoot returns the correct repository
// root path as an absolute path.
func TestFindRepoRoot(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)

	root, err := FindRepoRoot(dir)
	if err != nil {
		t.Fatalf("FindRepoRoot(%q): %v", dir, err)
	}

	// Resolve symlinks for a stable comparison on macOS where /tmp -> /private/tmp.
	wantEval, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("eval symlinks for %q: %v", dir, err)
	}
	gotEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks for %q: %v", root, err)
	}

	if gotEval != wantEval {
		t.Errorf("FindRepoRoot = %q, want %q", gotEval, wantEval)
	}
}

// TestFindRepoRoot_NonGitDirectory verifies that FindRepoRoot returns an error
// when called from outside a git repository.
func TestFindRepoRoot_NonGitDirectory(t *testing.T) {
	checkGitAvailable(t)

	dir, err := os.MkdirTemp("", "not-a-git-repo-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	_, err = FindRepoRoot(dir)
	if err == nil {
		t.Error("FindRepoRoot in non-git directory: expected error, got nil")
	}
}

// TestGetStatus_UntrackedFile verifies that a newly created file that has not
// been staged appears as "untracked" in the status map.
func TestGetStatus_UntrackedFile(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello world\n")

	status, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	got, ok := status["hello.txt"]
	if !ok {
		t.Fatalf("expected hello.txt in status map, got keys: %v", mapKeys(status))
	}
	if got != FileStatusUntracked {
		t.Errorf("hello.txt status = %q, want %q", got, FileStatusUntracked)
	}
}

// TestGetStatus_StagedFile verifies that after `git add`, the file transitions
// from untracked to staged.
func TestGetStatus_StagedFile(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello world\n")

	// Before staging the file should be untracked.
	beforeStatus, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus before add: %v", err)
	}
	if s := beforeStatus["hello.txt"]; s != FileStatusUntracked {
		t.Errorf("before add: hello.txt = %q, want %q", s, FileStatusUntracked)
	}

	// Stage the file.
	runGit(t, dir, "add", "hello.txt")

	afterStatus, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus after add: %v", err)
	}

	got, ok := afterStatus["hello.txt"]
	if !ok {
		t.Fatalf("expected hello.txt in status map after add, got keys: %v", mapKeys(afterStatus))
	}
	if got != FileStatusStaged {
		t.Errorf("after add: hello.txt status = %q, want %q", got, FileStatusStaged)
	}
}

// TestGetStatus_ModifiedFile verifies that a file that was committed and then
// modified appears as "modified".
func TestGetStatus_ModifiedFile(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	writeFile(t, dir, "hello.txt", "hello world\n")
	runGit(t, dir, "add", "hello.txt")
	runGit(t, dir, "commit", "-m", "initial commit")

	// Modify the committed file.
	writeFile(t, dir, "hello.txt", "changed content\n")

	status, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	got, ok := status["hello.txt"]
	if !ok {
		t.Fatalf("expected hello.txt in status map, got keys: %v", mapKeys(status))
	}
	if got != FileStatusModified {
		t.Errorf("hello.txt status = %q, want %q", got, FileStatusModified)
	}
}

// TestGetStatus_DeletedFile verifies that a file deleted from the working tree
// after being committed appears as "deleted".
func TestGetStatus_DeletedFile(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	path := writeFile(t, dir, "goodbye.txt", "bye\n")
	runGit(t, dir, "add", "goodbye.txt")
	runGit(t, dir, "commit", "-m", "add goodbye")

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	status, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	got, ok := status["goodbye.txt"]
	if !ok {
		t.Fatalf("expected goodbye.txt in status map, got keys: %v", mapKeys(status))
	}
	if got != FileStatusDeleted {
		t.Errorf("goodbye.txt status = %q, want %q", got, FileStatusDeleted)
	}
}

// TestGetStatus_EmptyRepo verifies that GetStatus returns an empty map when
// the repository is clean (no modified, added, or untracked files).
func TestGetStatus_EmptyRepo(t *testing.T) {
	checkGitAvailable(t)

	dir := initRepo(t)
	writeFile(t, dir, "committed.txt", "content\n")
	runGit(t, dir, "add", "committed.txt")
	runGit(t, dir, "commit", "-m", "initial")

	status, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus on clean repo: %v", err)
	}
	if len(status) != 0 {
		t.Errorf("expected empty status map for clean repo, got %v", status)
	}
}

// TestParsePorcelain exercises parsePorcelain directly with a variety of XY
// codes to ensure the mapping is correct.
func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPath string
		wantStat FileStatus
	}{
		{
			name:     "untracked",
			input:    "?? new-file.go\n",
			wantPath: "new-file.go",
			wantStat: FileStatusUntracked,
		},
		{
			name:     "staged new file (A space)",
			input:    "A  staged.go\n",
			wantPath: "staged.go",
			wantStat: FileStatusStaged,
		},
		{
			name:     "staged then modified (AM)",
			input:    "AM modified-staged.go\n",
			wantPath: "modified-staged.go",
			wantStat: FileStatusStaged,
		},
		{
			name:     "modified in worktree (space M)",
			input:    " M worktree-mod.go\n",
			wantPath: "worktree-mod.go",
			wantStat: FileStatusModified,
		},
		{
			name:     "modified in index (M space)",
			input:    "M  index-mod.go\n",
			wantPath: "index-mod.go",
			wantStat: FileStatusModified,
		},
		{
			name:     "modified both (MM)",
			input:    "MM both-mod.go\n",
			wantPath: "both-mod.go",
			wantStat: FileStatusModified,
		},
		{
			name:     "deleted from worktree (space D)",
			input:    " D deleted.go\n",
			wantPath: "deleted.go",
			wantStat: FileStatusDeleted,
		},
		{
			name:     "deleted from index (D space)",
			input:    "D  index-deleted.go\n",
			wantPath: "index-deleted.go",
			wantStat: FileStatusDeleted,
		},
		{
			name:     "renamed",
			input:    "R  new-name.go -> old-name.go\n",
			wantPath: "new-name.go",
			wantStat: FileStatusRenamed,
		},
		{
			name:     "ignored",
			input:    "!! build/output\n",
			wantPath: "build/output",
			wantStat: FileStatusIgnored,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parsePorcelain([]byte(tc.input))
			got, ok := result[tc.wantPath]
			if !ok {
				t.Fatalf("path %q not in result map; keys: %v", tc.wantPath, mapKeys(result))
			}
			if got != tc.wantStat {
				t.Errorf("status for %q = %q, want %q", tc.wantPath, got, tc.wantStat)
			}
		})
	}
}

// mapKeys returns the keys of a FileStatus map as a slice for error messages.
func mapKeys(m map[string]FileStatus) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
