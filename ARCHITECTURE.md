# FilePilot Architecture

## Overview

FilePilot is a native macOS file explorer combining a Go backend with a Vanilla JavaScript frontend. The application leverages Wails v2 as the bridge between desktop and web technologies, providing a responsive, feature-rich file management experience.

### Core Stack

- **Backend Runtime**: Go with Wails v2 framework
- **Frontend**: Vanilla JavaScript (ES6 modules)
- **UI Rendering**: WebView (WKWebView) in native macOS window
- **File Indexing**: SQLite with FTS5 (Full-Text Search 5) virtual tables
- **Interprocess Communication**: Wails auto-generated JavaScript bindings

### Architecture Pattern

Wails v2 implements a unidirectional architecture:

1. Frontend sends method calls via JavaScript bindings to the Go backend
2. Go processes requests and returns results asynchronously via JavaScript Promises
3. Go can emit events to trigger frontend updates without request initiation
4. Frontend assets (JS, CSS, HTML, images) are embedded in the binary via `go:embed`

The native window is configured with:
- Transparent titlebar for seamless macOS integration
- System menu bar integration
- Native file dialogs and context menus
- Drag-and-drop support from Finder

---

## Backend Architecture

### Package Organization

The backend is organized into 11 packages, each with a single, well-defined responsibility:

| Package | Path | Lines | Responsibility |
|---------|------|-------|-----------------|
| `main` | `main.go` | 50 | Wails app bootstrap, window configuration, macOS-specific options |
| `main` (App) | `app.go` | 700+ | Central binding struct, 60+ exported methods, startup/shutdown lifecycle, state orchestration |
| `fileops` | `internal/fileops/` | 400+ | File operations: Move, Copy, Rename, CreateDir, CreateFile, trash (macOS `osascript`), disk usage calculation |
| `index` | `internal/index/` | 600+ | SQLite FTS5 indexing, background filesystem scanner, watcher, favorites persistence |
| `search` | `internal/search/` | 300+ | FTS5 prefix queries, substring search, regex filters, scope-aware variants |
| `preview` | `internal/preview/` | 500+ | Multi-format preview dispatcher, image base64 encoding, PDF/audio/video metadata, binary hex view |
| `vfs` | `internal/vfs/` | 800+ | FileSystem interface and implementations (local, SFTP, FTP, S3, WebDAV), connection management |
| `credentials` | `internal/credentials/` | 150+ | macOS Keychain integration via `security` CLI, service-namespaced storage |
| `git` | `internal/git/` | 250+ | Git repository detection, status parsing (porcelain format), file-level status tracking |
| `opqueue` | `internal/opqueue/` | 200+ | Channel-based worker queue, pluggable operation handlers, pause/resume/cancel semantics |
| `server` | `internal/server/` | 200+ | HTTP server for preview content delivery, MIME detection, path traversal guards |
| `sync` | `internal/sync/` | 350+ | Bidirectional folder comparison, conflict detection, sync planning |

---

## Backend: Main App Struct

**File**: `app.go`
**Lines**: 700+
**Exported Methods**: 60+

The `App` struct is the central binding point between the frontend and all backend services. It holds references to all subsystems and orchestrates their lifecycle.

### Lifecycle Methods

- `Startup(ctx context.Context)`: Called when Wails initializes; starts services, loads index, begins watcher
- `BeforeClose(ctx context.Context) bool`: Called before window close; persists state, flushes database
- `Shutdown(ctx context.Context)`: Final cleanup; stops watcher, closes connections

### File Operations

- `List(path string, opts ListOptions) ([]FileEntry, error)`: Directory listing with sorting, filtering, git status
- `Stat(path string) (*FileEntry, error)`: Single-file metadata
- `Read(path string, offset int64, limit int) ([]byte, error)`: Byte-range file reads
- `Write(path string, data []byte) error`: File write with conflict detection
- `CreateFile(name string, parentPath string) error`: Atomic file creation
- `CreateDir(name string, parentPath string) error`: Directory creation with error handling
- `Move(srcPath string, destPath string, undo bool) error`: Atomic move with undo stack
- `Copy(srcPath string, destPath string, recursive bool) error`: Recursive copy with conflict handling
- `Rename(path string, newName string) error`: Atomic rename with undo support
- `Delete(paths []string, permanent bool) error`: Batch delete (trash or permanent)
- `Duplicate(path string, newName string) error`: File duplication with name conflict handling

### Index & Search

- `SearchFiles(query string, scope string, limit int) ([]SearchResult, error)`: FTS5 full-text search
- `GetSearchSuggestions(prefix string, limit int) ([]string, error)`: Prefix-based autocomplete
- `IndexPath(path string) error`: On-demand index update for single path
- `RebuildIndex() error`: Full index rebuild
- `GetIndexStatus() IndexStatus`: Current index progress and stats
- `AddFavorite(path string) error`: Persistent favorites store
- `RemoveFavorite(path string) error`: Remove from favorites
- `ListFavorites() ([]string, error)`: Retrieve all favorites

### Preview & Preview Service

- `GetPreview(path string, maxWidth int, maxHeight int) (PreviewData, error)`: Unified preview dispatcher
- `GetFileMetadata(path string) (FileMetadata, error)`: Format-specific metadata extraction
- `GetDiskUsage(path string, recursive bool) (int64, error)`: Disk space calculation

### Remote Storage (VFS)

- `GetConnections() ([]Connection, error)`: List active remote connections
- `CreateConnection(type string, config map[string]string) (string, error)`: Establish SFTP/FTP/S3/WebDAV connection
- `TestConnection(connID string) error`: Verify connection health
- `CloseConnection(connID string) error`: Terminate connection
- `ListRemote(connID string, path string) ([]FileEntry, error)`: List files on remote
- `DownloadFromRemote(connID string, remotePath string, localPath string, onProgress func) error`: Download with progress
- `UploadToRemote(connID string, localPath string, remotePath string, onProgress func) error`: Upload with progress

### Git Integration

- `GetGitStatus(path string) (GitStatus, error)`: Get git status for single file
- `GetGitStatusBulk(paths []string) (map[string]GitStatus, error)`: Batch status lookup
- `IsInGitRepo(path string) bool`: Check if path is in git repository
- `OpenInGit(repoPath string) error`: Trigger git CLI

### Operation Queue & Batch Operations

- `QueueOperation(opType string, srcPaths []string, destPath string) (string, error)`: Queue file operation
- `PauseOperation(opID string) error`: Pause in-progress operation
- `ResumeOperation(opID string) error`: Resume paused operation
- `CancelOperation(opID string) error`: Cancel and rollback
- `GetOperationStatus(opID string) (OperationStatus, error)`: Real-time progress

### Clipboard & Drag-Drop

- `CopyToClipboard(paths []string) error`: Native clipboard copy
- `PasteFromClipboard(destPath string) ([]string, error)`: Paste with conflict handling
- `SetClipboardMode(mode string) error`: Set "copy" or "move" mode

### Sync

- `CompareFolders(leftPath string, rightPath string) ([]DiffEntry, error)`: Bidirectional comparison
- `PlanSync(leftPath string, rightPath string, strategy string) ([]SyncStep, error)`: Generate sync plan
- `ExecuteSync(plan []SyncStep) error`: Execute synchronized update

### System Integration

- `GetMountedVolumes() ([]Volume, error)`: List mounted disks with free space
- `OpenWith(path string, bundleID string) error`: Open file with specific app
- `RevealInFinder(path string) error`: Highlight file in Finder
- `ShowQuickLook(path string) error`: Open system QuickLook
- `GetSystemInfo() (SystemInfo, error)`: macOS version, available RAM, CPU

---

## Backend: Key Packages

### `fileops` — File Operations

**Responsibility**: Safe, atomic file operations with undo/redo support.

#### Core Operations

**Move/Copy/Rename**:
- Atomic: uses temporary files + rename pattern
- Conflict handling: auto-rename on collision (e.g., "file.txt" → "file 2.txt")
- Undo stack: stores operation metadata (max 100 entries, auto-pruned)
- Cross-volume moves: delegates to Copy + Delete on different filesystems

**Trash & Permanent Delete**:
- macOS: uses `osascript` to invoke Finder's trash functionality
- Permanent delete: `os.RemoveAll()` with confirmation prompt
- Recovery: undo restores from trash until emptied

**Directory & File Creation**:
- Atomicity: creates with restricted permissions (0644), then updates
- Conflict handling: same auto-rename logic as Move/Copy
- Parent creation: optional recursive parent directory creation

**Disk Usage**:
- Recursive traversal with `filepath.Walk()`
- Caching: optional memoization of results
- Hard links: counted once to avoid duplication

#### Undo Stack

- **Max size**: 100 entries
- **Entry format**: `{opType, srcPath, destPath, timestamp, metadata}`
- **Expiration**: oldest entries removed when limit exceeded
- **Persistence**: optional SQLite storage for cross-session undo

---

### `index` — File Indexing & Watching

**Responsibility**: Fast file search via SQLite FTS5, background filesystem watching.

#### SQLite Setup

```sql
CREATE VIRTUAL TABLE files USING fts5(
  path,
  name UNINDEXED,
  parent UNINDEXED,
  size UNINDEXED,
  mtime UNINDEXED,
  mime_type UNINDEXED,
  git_status UNINDEXED
);
```

- **WAL mode**: enables concurrent readers during writes
- **Pragma settings**: optimized for SSD performance, compression enabled
- **Index updates**: incremental via watcher, bulk rebuild on startup

#### Background Scanner

- **Trigger**: app startup
- **Concurrency**: single goroutine with channel-based work queue
- **Progress reporting**: emits event every N files indexed
- **Rate limiting**: configurable delay between file reads
- **Cancellation**: context-aware, stops gracefully

#### Filesystem Watcher

- **Tool**: `github.com/fsnotify/fsnotify`
- **Events**: watch for Create, Write, Remove, Rename
- **Debouncing**: 100ms batching of rapid events
- **Index updates**: async updates via channel to index goroutine
- **Recursion**: configurable max depth to prevent system overload

#### Favorites Store

- **Storage**: SQLite table `favorites(id, path, added_at)`
- **API**: `AddFavorite(path)`, `RemoveFavorite(path)`, `ListFavorites()`
- **Sync**: persisted immediately on change

---

### `search` — FTS5 Search Engine

**Responsibility**: Multi-mode file search with scope filtering.

#### Search Modes

**Prefix Search** (fastest):
```sql
SELECT * FROM files WHERE name MATCH 'prefix*' LIMIT 50;
```
- Used for autocomplete and quick filters
- Results ranked by name length (shorter first)

**Substring Search**:
```sql
SELECT * FROM files WHERE files MATCH 'sub*' ORDER BY rank;
```
- Case-insensitive matching
- Matches mid-word occurrences

**Regex Search** (post-filter in Go):
- Pattern compiled once and reused
- Applied to result set from FTS5
- Prevents network overhead for complex patterns

**Scope-Aware Search**:
```sql
SELECT * FROM files WHERE files MATCH '...' AND parent LIKE '...%' LIMIT limit;
```
- Restricts results to specified directory tree
- Single query with parent path filter

#### Result Ranking

1. FTS5 built-in rank (proximity + term frequency)
2. Custom scoring: exact name matches boosted 2x
3. Recency bonus: files modified recently ranked higher
4. Type penalties: `.tmp` and system files deprioritized

---

### `preview` — Multi-Format Preview Dispatcher

**Responsibility**: Detect file type and generate appropriate preview representation.

#### Preview Outputs

| Format | Method | Output | Constraints |
|--------|--------|--------|-------------|
| Directory | Stat+Count | Path + item count | N/A |
| Image (JPEG/PNG/WebP) | Base64 | Data URI | Max 1MB, resize to fit |
| iCloud | Stat + flag | Metadata only | No content preview |
| PDF | PDF.js | Rendered first page + metadata | Max 200 lines, 1MB cap |
| Audio (MP3/M4A) | ID3 tags | Title, artist, duration | ID3 parsing via Go lib |
| Video (MP4/MOV) | ffprobe metadata | Resolution, duration, codec | External tool required |
| Archive (ZIP/TAR/GZ) | Content listing | File tree (first 50 entries) | No decompression |
| Font (TTF/OTF/WOFF) | Font metadata | Family, style, char count | Rasterized preview |
| Plist (XML/Binary) | Parsing | Pretty-printed property list | Max 500 keys |
| Code (JS/Go/Python/etc) | Syntax highlight | First 200 lines with colors | Language detection + highlight.js |
| Plaintext | Direct read | First 200 lines | UTF-8 decoding |
| Binary | Hex dump | First 1MB as hexadecimal | 16 bytes per line |

#### Performance Optimizations

- **Lazy library loading**: PDF.js and highlight.js loaded on first use
- **Size gating**: skips expensive operations on very large files
- **Timeout**: 2-second timeout per preview operation
- **Caching**: results memoized in memory (LRU, 100-item limit)

---

### `vfs` — Virtual File System Abstraction

**Responsibility**: Unified interface for local and remote file storage.

#### FileSystem Interface

```go
type FileSystem interface {
  List(ctx context.Context, path string) ([]FileEntry, error)
  Stat(ctx context.Context, path string) (*FileEntry, error)
  Read(ctx context.Context, path string, offset int64, limit int) ([]byte, error)
  Write(ctx context.Context, path string, data []byte) error
  Mkdir(ctx context.Context, path string) error
  Remove(ctx context.Context, path string) error
  Rename(ctx context.Context, oldPath string, newPath string) error
  Copy(ctx context.Context, src string, dest string) error
  Move(ctx context.Context, src string, dest string) error
}
```

#### Implementations

**Local** (`vfs/local/`):
- Wraps `os` package methods
- Direct filesystem access
- No connection setup required

**SFTP** (`vfs/sftp/`):
- Uses `github.com/pkg/sftp`
- SSH key-based authentication
- Connection pooling for multiple operations
- Bandwidth throttling support

**FTP** (`vfs/ftp/`):
- Uses `github.com/jlaffaye/ftp`
- Active/passive mode negotiation
- TLS support (FTPS)
- Resume capability for transfers

**S3** (`vfs/s3/`):
- Uses `github.com/aws/aws-sdk-go-v2`
- IAM role or key-based authentication
- Streaming uploads for large files
- Multipart upload for >100MB

**WebDAV** (`vfs/webdav/`):
- Uses `github.com/studio-b12/gowebdav`
- HTTP/HTTPS support
- LOCK/UNLOCK for file operations
- Custom headers and proxies

#### Connection Manager

**Role**: Lifecycle management for remote connections.

```go
type ConnMgr struct {
  conns map[string]FileSystem // connID → FileSystem instance
  mu    sync.RWMutex
  // ...
}
```

- **Connection ID**: UUID or hash of credentials
- **Pooling**: reuses active connections for same credentials
- **Timeout**: closes idle connections after 30 minutes
- **Status monitoring**: periodic health checks via `Stat("/")`

#### Path Format

Remote paths use a prefix scheme:

```
remote://{connID}/{path}
```

Example:
```
remote://sftp-1e8c92f3/home/user/documents/file.txt
```

---

### `credentials` — Secure Storage

**Responsibility**: Encrypt and store connection credentials securely.

#### macOS Keychain Integration

Uses `security` CLI for access:

```bash
security add-generic-password \
  -a "filepilot-sftp-1e8c92f3" \
  -s "filepilot-sftp-1e8c92f3" \
  -w "password" \
  -U
```

**Service prefix**: All credentials stored under `"filepilot-"` namespace to prevent collisions.

#### Credential Types

| Type | Fields | Storage |
|------|--------|---------|
| SFTP | host, port, username, password/key, key passphrase | Keychain |
| FTP | host, port, username, password | Keychain |
| S3 | access_key_id, secret_access_key, region, bucket | Keychain |
| WebDAV | url, username, password, proxy_url | Keychain |

#### Operations

- **Store**: `StoreCredentials(connID string, creds map[string]string) error`
- **Retrieve**: `GetCredentials(connID string) (map[string]string, error)` — 5s timeout
- **Delete**: `DeleteCredentials(connID string) error` — called on connection close
- **List**: `ListCredentials() ([]string, error)` — returns stored connection IDs

#### Timeout Handling

- Command execution timeout: 5 seconds
- Graceful degradation: returns error if Keychain unavailable
- Manual fallback: optional in-memory storage for testing/development

---

### `git` — Git Repository Integration

**Responsibility**: Detect git repos and provide file-level status.

#### Core Methods

- **IsGitRepo(path string) bool**: Check if path is inside a git repository
- **FindRepoRoot(path string) (string, error)**: Locate `.git` directory
- **GetStatus(path string) (map[string]GitStatus, error)**: File statuses for directory

#### Status Format

Git porcelain format (v1):

```
M  file.txt          # modified
A  newfile.js        # added
D  deleted.go        # deleted
R  old.txt -> new.txt # renamed
?? untracked.py      # untracked
!! ignored.log       # ignored
```

#### Status Enum

```go
const (
  GitStatusModified   = "modified"
  GitStatusAdded      = "added"
  GitStatusDeleted    = "deleted"
  GitStatusRenamed    = "renamed"
  GitStatusUntracked  = "untracked"
  GitStatusIgnored    = "ignored"
  GitStatusStaged     = "staged"
  GitStatusConflict   = "conflict"
)
```

#### Performance

- **Caching**: results cached for 5 seconds to avoid repeated `git status` calls
- **Scope**: only queries the directory containing the requested path
- **Fallback**: graceful degradation if git not installed or not a repo

---

### `opqueue` — Operation Queue

**Responsibility**: Queue and execute long-running file operations with progress tracking.

#### Design

Channel-based worker pool:

```go
type OpQueue struct {
  workers   int
  queue     chan *Operation
  ctx       context.Context
  handlers  map[OpType]ExecuteFunc
  // ...
}
```

#### Operation Types

| Type | Handler | Parallel | Undo |
|------|---------|----------|------|
| Copy | `fileops.Copy()` | Yes (up to 4) | Yes |
| Move | `fileops.Move()` | No (serial) | Yes |
| Delete | `fileops.Delete()` | No (serial) | Yes |
| Download | `vfs.Read()` + Write | Yes (up to 2) | Yes |
| Upload | Read + `vfs.Write()` | Yes (up to 2) | Yes |

#### Lifecycle

1. **Queue**: `QueueOperation(opType, srcPaths, destPath)` → returns opID
2. **Track**: `GetOperationStatus(opID)` returns progress struct
3. **Control**: `PauseOperation(opID)`, `ResumeOperation(opID)`, `CancelOperation(opID)`
4. **Complete**: operation removed from queue, results available via binding event

#### Status Struct

```go
type OperationStatus struct {
  ID          string    // UUID
  Type        string    // Copy, Move, Delete, etc.
  State       string    // queued, in_progress, paused, completed, failed
  Progress    int       // 0-100
  ItemsCurrent int      // 3 of 50
  ItemsTotal  int
  BytesCurrent int64    // for Copy/Download
  BytesTotal  int64
  StartTime   time.Time
  EstimatedEnd time.Time
  Error       string    // if failed
}
```

---

### `server` — Preview Content Server

**Responsibility**: Serve file content via HTTP for preview rendering.

#### Startup

- Binds to `127.0.0.1:0` (random port)
- Port is passed to frontend on startup
- Only accessible from localhost

#### Routes

**GET /serve**

```
/serve?path=/Users/user/file.txt&offset=0&limit=1024
```

- `path`: absolute filesystem path
- `offset`: byte offset (for large files)
- `limit`: bytes to return

**Response headers**:
- `Content-Type`: detected via MIME type lookup
- `Content-Length`: actual byte count
- `Cache-Control`: `max-age=3600` (1 hour)
- `X-Content-Length-Display`: human-readable size

**GET /healthz**

- Returns `{"status": "ok"}` with 200 status
- Used by frontend to verify server availability

#### Security

- **Path traversal guard**: resolves all paths to absolute, rejects attempts to escape root
- **Whitelist**: only serves paths under configured include list (usually home directory)
- **CORS**: restricted to localhost only

#### MIME Detection

Hybrid approach:
1. File extension lookup (instant)
2. Magic bytes inspection (for ambiguous types)
3. Fallback: `application/octet-stream`

---

### `sync` — Bidirectional Folder Comparison

**Responsibility**: Compare two folder trees and generate sync plans.

#### Compare() Method

Walks both directory trees and produces:

```go
type DiffEntry struct {
  Path        string // relative path
  LeftFile    *FileEntry
  RightFile   *FileEntry
  Status      string // left_only, right_only, newer_left, newer_right, identical, conflict
}
```

**Algorithm**:
1. Build file tree for left path
2. Build file tree for right path
3. Merge trees by relative path
4. Compare: mtime, size, hash (if enabled)
5. Classify each entry by status

**Conflict Detection**:
- Both sides modified → `conflict`
- Same modification time but different size → `conflict`
- Different hashes (optional) → `conflict`

#### PlanSync() Method

Generates a sequence of operations to synchronize directories:

```go
type SyncStep struct {
  Action    string // copy_left_to_right, copy_right_to_left, delete_left, delete_right, skip
  SourcePath string
  DestPath   string
  Reason    string // "newer_left", "left_only", etc.
}
```

**Strategies**:
- `sync_left_to_right`: copies left → right, deletes right-only
- `sync_right_to_left`: copies right → left, deletes left-only
- `merge`: two-way sync, skips conflicts
- `mirror_left`: right becomes exact copy of left
- `mirror_right`: left becomes exact copy of right

#### ExecuteSync() Method

Runs plan atomically:
- Creates transaction if both paths are local
- Rolls back on any error
- Updates index after completion

---

## Frontend Architecture

### Module Organization

The frontend is organized into 37+ ES6 modules grouped by function:

#### Core Modules (5)

| Module | Responsibility |
|--------|-----------------|
| `main.js` | App bootstrap, DOM setup, event delegation |
| `app.js` | Global state, event emitter, router |
| `pane.js` | Dual-pane model, tab management |
| `virtual-scroller.js` | Viewport-based rendering, infinite scroll |
| `lazy-libs.js` | On-demand loading of D3, PDF.js, highlight.js |

#### View Modules (4)

| Module | Type | Features |
|--------|------|----------|
| `column.js` | Finder-style columns | Hierarchical breadcrumb, auto-expand |
| `list.js` | Table view | Sortable columns, inline rename, git status icon |
| `grid.js` | Icon grid | Thumbnail generation, label overlay |
| `graph.js` | D3 force graph | Node interaction, drag physics simulation |

#### Component Modules (20+)

| Module | Purpose |
|--------|---------|
| `sidebar.js` | Favorites, recents, mounted volumes, bookmarks |
| `breadcrumb.js` | Path navigation, clickable segments |
| `preview.js` | Content preview pane, format dispatch |
| `search.js` | Search bar, suggestions dropdown, filters |
| `context-menu.js` | Right-click menu, custom actions |
| `command-palette.js` | Cmd+K command launcher |
| `quicklook.js` | System QuickLook integration |
| `info-panel.js` | File metadata display, bulk properties |
| `tag-editor.js` | File tag management |
| `rename-dialog.js` | Inline file rename modal |
| `connection-dialog.js` | Remote storage connection setup |
| `diff-view.js` | Side-by-side folder comparison |
| `pane-container.js` | Splitview layout manager |
| `modal.js` | Generic modal wrapper |
| `toolbar.js` | Action buttons, view mode toggle |
| `status-bar.js` | Selection info, operation progress |
| `progress-indicator.js` | Download/upload progress display |
| `icon-generator.js` | System icon caching, emoji fallback |
| `notification.js` | Toast notifications, alerts |
| `settings-panel.js` | Preferences UI |

#### Service Modules (8)

| Module | Responsibility |
|--------|-----------------|
| `api.js` | Wails binding wrapper, Promise-based calls |
| `clipboard.js` | Copy/paste with conflict handling |
| `dragdrop.js` | Drag-and-drop event handling |
| `shortcuts.js` | Keyboard shortcuts, custom bindings |
| `gitstatus.js` | Git status caching and display |
| `opqueue.js` | Operation queue UI binding |
| `settings.js` | Persistent user preferences |
| `workspaces.js` | Multi-window state, session persistence |

---

### State Management

Global state is maintained in `app.js` as a reactive state object:

```javascript
const state = {
  // Navigation
  currentPath: "/Users/user",
  history: { back: [...], forward: [...] },

  // View
  viewMode: "list", // list | grid | column
  sortBy: "name",
  sortOrder: "asc",
  showHidden: false,

  // Entries & Selection
  entries: [...], // Current directory listing
  selectedFiles: new Set(),
  activePath: null,

  // Clipboard
  clipboard: { mode: "copy", paths: [...] },

  // Search
  searchQuery: "",
  searchResults: [...],

  // Git
  gitStatuses: new Map(),

  // UI
  previewOpen: true,
  sidebarCollapsed: false,
};
```

#### State Updates

- Changes trigger `app.emit("state-changed", { key, oldValue, newValue })`
- Listeners update DOM via query selectors
- No virtual DOM — direct manipulation for performance

#### Pane Model

Dual-pane architecture via `pane.js`:

```javascript
const panes = {
  left: {
    path: "/Users/user",
    entries: [...],
    selectedFiles: new Set(),
    viewMode: "list",
    tabs: [{path, viewMode}, ...],
  },
  right: { /* same structure */ },
};
```

- Each pane has independent path, view mode, selection
- Tabs per pane allow quick switching between directories
- Active pane highlighted via `[data-active-pane]` attribute

---

### Data Flow

Request lifecycle:

1. **User Action**: Click, keyboard, drag-drop
2. **Event Handler**: Captured by event delegation listener
3. **API Call**: `await api.call("App", "List", [path, opts])`
4. **Binding Resolution**: Waits for `window.go.main.App` (Wails bootstrap)
5. **Go Execution**: Backend processes request
6. **Promise Resolution**: Frontend receives response
7. **State Update**: `app.setState({...})`
8. **DOM Update**: Listen for `state-changed` event, update affected elements

#### Example: File List

```javascript
// User navigates to folder
async function navigate(path) {
  // Wait for Wails bindings
  await api.waitForBindings();

  // Call Go method
  const entries = await api.call("App", "List", [path, {
    sortBy: state.sortBy,
    sortOrder: state.sortOrder,
    showHidden: state.showHidden,
  }]);

  // Update state
  app.setState({ currentPath: path, entries });

  // Get git statuses (async)
  const statuses = await api.call("App", "GetGitStatusBulk",
    [entries.map(e => e.path)]);
  app.setState({ gitStatuses: new Map(Object.entries(statuses)) });
}
```

---

### Performance Optimizations

#### Virtual Scrolling

Triggered when item count exceeds threshold:
- **List view**: 300+ items
- **Grid view**: 300+ items

**Implementation**:
- Render visible items + 50-item buffer
- Update range on scroll via `scroll` event listener
- DOM nodes reused via template cloning
- Reduces DOM size from 10,000 to ~100 nodes

#### Thumbnail Caching

**Concurrent loading**:
- 6-worker thread pool
- Each worker: fetch image → base64 encode → cache
- Replace emoji icon with `<img src="data:...">` on completion

**Cache strategy**:
- In-memory LRU (500-item limit)
- Expires after 1 hour
- Fallback to emoji if encoding fails

#### Lazy Library Loading

**Large dependencies**:
- D3: ~280KB, loaded on first graph view
- PDF.js: ~200KB, loaded on first PDF preview
- highlight.js: ~60KB, loaded on first code file

**Implementation**:
```javascript
async function ensureD3Loaded() {
  if (window.d3) return;
  const script = document.createElement("script");
  script.src = "https://cdn.jsdelivr.net/npm/d3@7";
  await new Promise(resolve => script.onload = resolve);
  document.head.appendChild(script);
}
```

#### Event Delegation

**Container-level listeners**:
- One listener per event type per container (instead of per item)
- Uses `event.target` matching to identify item
- Reduces event listener count from 3 per file to 3 per container

**Typical setup**:
```javascript
container.addEventListener("click", (e) => {
  const item = e.target.closest("[data-file-path]");
  if (!item) return;

  const path = item.dataset.filePath;
  // Handle interaction
});
```

#### Background Index Scanning

- Runs in separate goroutine on startup
- Does not block UI
- Progress emitted via event binding every 1000 files
- Can be paused/resumed via `PauseIndexing()` / `ResumeIndexing()`

#### Binding Detection

**Wails bootstrap check**:
```javascript
async function waitForBindings(timeout = 5000) {
  const start = Date.now();
  while (!window.go?.main?.App) {
    if (Date.now() - start > timeout) {
      throw new Error("Wails bindings not available");
    }
    await new Promise(r => requestAnimationFrame(r));
  }
}
```

- Uses `requestAnimationFrame` instead of `setInterval`
- More efficient, aligns with browser repaint cycle
- Timeout prevents infinite hangs

---

## CSS Architecture

Three-layer system for maintainability and theming:

### Layer 1: Design Tokens (`theme.css`)

Defines design system values:

```css
:root {
  /* Colors */
  --bg-primary: #ffffff;
  --bg-secondary: #f5f5f5;
  --text-primary: #1a1a1a;
  --text-secondary: #666666;
  --accent: #ff9500;
  --accent-light: #ffb84d;
  --error: #ff3b30;
  --success: #34c759;

  /* Spacing */
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --spacing-lg: 24px;
  --spacing-xl: 32px;

  /* Typography */
  --font-family-base: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --font-family-mono: "Menlo", "Monaco", monospace;
  --font-size-sm: 12px;
  --font-size-base: 14px;
  --font-size-lg: 16px;
  --font-size-xl: 20px;
  --font-weight-normal: 400;
  --font-weight-bold: 600;

  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.1);
  --shadow-md: 0 4px 8px rgba(0, 0, 0, 0.15);
  --shadow-lg: 0 12px 24px rgba(0, 0, 0, 0.2);

  /* Border radius */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
}

/* Dark theme */
[data-theme="dark"] {
  --bg-primary: #1a1a1a;
  --bg-secondary: #2a2a2a;
  --text-primary: #ffffff;
  --text-secondary: #cccccc;
}
```

**Characteristics**:
- Single source of truth for all design values
- Easy theme switching via `[data-theme]` attribute
- Warm amber accent palette (--accent: #ff9500)

### Layer 2: Layout (`layout.css`)

Structural and regional layout:

```css
body {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.titlebar {
  height: 44px;
  background: var(--bg-secondary);
  -webkit-app-region: drag; /* macOS titlebar drag */
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.content {
  display: flex;
  flex: 1;
  min-height: 0;
}

.sidebar {
  width: 200px;
  border-right: 1px solid var(--border-color);
  overflow-y: auto;
  flex-shrink: 0;
}

.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.pane-container {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: var(--spacing-md);
}

.pane {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.preview {
  width: 300px;
  border-left: 1px solid var(--border-color);
  overflow-y: auto;
  flex-shrink: 0;
}

.status-bar {
  height: 32px;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--spacing-md);
  font-size: var(--font-size-sm);
}
```

**Flexbox-driven layout**:
- Flexible regions (sidebar, main, preview, status bar)
- Sticky headers during scroll
- Proper `min-width: 0` and `min-height: 0` for flex children

### Layer 3: Components (`components.css`)

Individual component styles:

```css
/* Buttons */
.btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  background: var(--accent);
  color: white;
  cursor: pointer;
  font-size: var(--font-size-base);
  transition: background 0.2s ease;
}

.btn:hover {
  background: var(--accent-light);
}

.btn.secondary {
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

/* Pills (tags, badges) */
.pill {
  display: inline-block;
  padding: 2px var(--spacing-sm);
  border-radius: 12px;
  background: var(--accent);
  color: white;
  font-size: var(--font-size-sm);
  white-space: nowrap;
}

/* Cards */
.card {
  padding: var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-secondary);
}

/* Overlays & Dialogs */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--spacing-lg);
  max-width: 600px;
  width: 90%;
}

/* Animations */
@keyframes slideIn {
  from {
    transform: translateX(-100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.sidebar {
  animation: slideIn 0.3s ease-out;
}
```

**Component-scoped styles**:
- Minimal cascading
- BEM-like class naming
- Smooth transitions and animations
- Theme-aware via CSS variables

---

## Data Models

### FileEntry

```go
type FileEntry struct {
  Path         string    // Absolute path
  Name         string    // Basename
  Size         int64     // File size in bytes
  ModTime      time.Time // Last modified
  IsDir        bool      // Directory flag
  IsSymlink    bool      // Symlink flag
  Permissions string    // e.g., "-rw-r--r--"
  MimeType     string    // e.g., "text/plain"
  GitStatus    string    // Git status enum
  IconBase64   string    // System icon as data URI
  IsHidden     bool      // Hidden file (.* prefix)
  IsFavorite   bool      // In favorites list
}
```

### SearchResult

```go
type SearchResult struct {
  Path        string    // Full path
  Name        string    // Filename
  ParentPath  string    // Parent directory
  Relevance   float64   // 0-100 relevance score
  MatchType   string    // prefix, substring, regex
  Context     string    // Snippet around match (if applicable)
}
```

### PreviewData

```go
type PreviewData struct {
  Type         string    // directory, image, pdf, audio, video, etc.
  Content      string    // Base64, HTML, or plaintext
  MimeType     string    // MIME type
  Size         int64     // File size
  Modified     time.Time // Last modified
  // Format-specific metadata
  ImageWidth   int
  ImageHeight  int
  AudioDuration float64
  PDFPages     int
  // ...
}
```

### OperationStatus

```go
type OperationStatus struct {
  ID            string
  Type          string     // Copy, Move, Delete, Download, Upload
  State         string     // queued, in_progress, paused, completed, failed
  Progress      int        // 0-100
  ItemsCurrent  int        // Current item number
  ItemsTotal    int        // Total items
  BytesCurrent  int64      // Bytes processed (for Copy/Download)
  BytesTotal    int64      // Total bytes
  StartTime     time.Time
  EstimatedEnd  time.Time
  Error         string
  Results       []string   // Paths of completed items (on completion)
}
```

### DiffEntry

```go
type DiffEntry struct {
  Path       string     // Relative path
  LeftFile   *FileEntry // File in left tree (nil if not present)
  RightFile  *FileEntry // File in right tree (nil if not present)
  Status     string     // left_only, right_only, newer_left, newer_right, identical, conflict
  Size       int64      // Size difference
  ModTime    time.Duration // Modification time difference
}
```

---

## Event System

### Binding Events (Go → Frontend)

Backend emits events that frontend can listen to:

```javascript
window.runtime.EventsOn("event:name", (data) => {
  // Handle event
});
```

#### Event Types

| Event | Payload | Purpose |
|-------|---------|---------|
| `file:changed` | `{path: string, action: "modified"\|"created"\|"deleted"}` | File watcher notification |
| `index:progress` | `{filesScanned: int, totalFiles: int, lastPath: string}` | Background index progress |
| `index:complete` | `{}` | Index rebuild finished |
| `git:status-update` | `{path: string, status: string}` | Git status changed |
| `operation:started` | `{opID: string, type: string, itemsTotal: int}` | Long-running operation started |
| `operation:progress` | `{opID: string, progress: int, itemsCurrent: int}` | Real-time operation progress |
| `operation:completed` | `{opID: string, error?: string, results: string[]}` | Operation finished |
| `server:ready` | `{port: int}` | Preview server started |
| `connection:established` | `{connID: string, type: string}` | Remote connection opened |
| `connection:closed` | `{connID: string}` | Remote connection closed |

### Method Binding (Frontend → Backend)

Frontend calls Go methods via Wails binding:

```javascript
const result = await window.go.main.App.MethodName(arg1, arg2, ...);
```

Each exported method on `App` becomes callable from frontend.

---

## Build & Distribution

### Frontend Build

```bash
npm run build
```

Outputs to `frontend/dist/` with:
- Bundled JS (tree-shaken, minified)
- Minified CSS
- Optimized assets

### Go Embedding

Frontend assets embedded via `go:embed`:

```go
//go:embed all:frontend/dist
var assets embed.FS
```

Binary includes complete UI; no external assets required.

### Wails Build

```bash
wails build -platform darwin/arm64
```

Produces:
- `.app` bundle with native macOS icon
- Code-signed with developer certificate
- Can be notarized for distribution

---

## Error Handling

### Backend Error Patterns

Go methods return `error` as second return value:

```go
func (a *App) SomeOperation() (result string, err error) {
  // ...
}
```

Frontend receives as `{code: string, message: string}` JSON:

```javascript
try {
  const result = await api.call("App", "SomeOperation", []);
} catch (err) {
  // err.code: e.g., "PERMISSION_DENIED", "NOT_FOUND", "CONFLICT"
  // err.message: human-readable message
  console.error(err);
}
```

### Frontend Error Boundaries

Components wrap async operations:

```javascript
async function loadFiles(path) {
  try {
    const entries = await api.call("App", "List", [path, {}]);
    return entries;
  } catch (err) {
    if (err.code === "PERMISSION_DENIED") {
      showNotification("Access denied to this folder", "error");
    } else if (err.code === "NOT_FOUND") {
      showNotification("Folder not found", "error");
    } else {
      showNotification(`Error: ${err.message}`, "error");
    }
    return [];
  }
}
```

---

## Security Considerations

### Path Traversal Guards

All path operations validated:
- Resolve to absolute path with `filepath.Clean()`
- Check that result doesn't escape allowed root
- Example: reject `../../../etc/passwd`

### Credential Storage

- Never hardcode credentials
- Use macOS Keychain for SFTP/FTP/S3/WebDAV
- Credentials fetched on-demand, never logged

### Sandbox Restrictions

- App runs in native macOS environment (not sandboxed)
- File access limited to user's home directory by default
- Remote connections explicitly opt-in

### Input Validation

- All user inputs validated before use
- File paths checked for traversal
- Search queries escaped for SQL injection prevention

---

## Performance Characteristics

### Benchmarks

| Operation | Time | Notes |
|-----------|------|-------|
| List 10K files | 200ms | Sorted, git status included |
| Search index (100K files) | 50ms | FTS5 prefix query |
| Thumbnail generation (100 images) | 5s | Parallel 6-worker pool |
| Copy 1GB file | 2s | Depends on disk speed |
| Remote SFTP list (100 files) | 500ms | Network latency dependent |

### Memory Usage

- Idle: ~150MB (Go runtime + WebView)
- 10K file list in memory: ~50MB
- Thumbnail cache (500 items): ~30MB
- Total typical: 200-250MB

### Scalability

- Index: handles 1M+ files efficiently
- Listing: virtual scroll handles 100K+ items
- Search: FTS5 handles 1M items with <100ms query time
- Remote: connection pooling, multiplexed transfers

---

## Testing Strategy

### Unit Tests

Backend packages include `_test.go` files:
- `fileops_test.go`: undo/redo, conflict handling
- `index_test.go`: FTS5 queries, watcher behavior
- `search_test.go`: ranking, scope filtering
- `vfs_test.go`: interface compliance, implementations

### Integration Tests

- Full workflow: navigate → select → copy → paste
- Remote storage: connect → list → transfer
- Sync: compare → plan → execute

### Manual QA

- Finder integration (drag-drop, reveal)
- macOS services (QuickLook, system dialogs)
- Performance with 10K+ files
- Remote protocols (SFTP, S3, WebDAV)

---

## Future Architecture Considerations

### Extensibility Points

- Custom preview formats: register handler in `preview.Handler`
- VFS implementations: satisfy `FileSystem` interface
- Search modes: add to `search.Query` dispatcher
- Operation types: register handler in `opqueue.handlers`

### Scalability Improvements

- Database partitioning for 10M+ files
- Distributed sync via conflict-free replicated types (CRDTs)
- Worker process pooling for remote operations
- Caching layer (Redis) for metadata

### Feature Opportunities

- Plugin system: expose Go APIs to JavaScript plugins
- Collaborative sync: real-time conflict resolution
- ML-based search ranking: learn from user patterns
- Mobile client: sync with iOS/iPadOS via protocol

---

## Glossary

| Term | Definition |
|------|-----------|
| Binding | Wails method callable from JavaScript |
| ConnMgr | Connection manager, manages remote filesystem instances |
| DiffEntry | Entry in folder comparison result |
| Event emission | Go → Frontend notification |
| FTS5 | SQLite Full-Text Search 5 virtual table |
| Pane | Independent file browser view in dual-pane layout |
| Preview | Content preview panel showing file details |
| VFS | Virtual FileSystem abstraction layer |
| Wails | Go framework for building desktop apps with web UIs |
| WebView | macOS WKWebView rendering HTML/CSS/JS |

---

## References

- [Wails v2 Documentation](https://wails.io/)
- [SQLite FTS5](https://www.sqlite.org/fts5.html)
- [macOS Keychain CLI](https://ss64.com/osx/security.html)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Virtual Scrolling Best Practices](https://web.dev/virtual-lists-react/)
