# FilePilot

A fast, native file explorer for macOS built with Go (Wails v2 backend) and vanilla JavaScript frontend.

<!-- ![FilePilot screenshot](docs/screenshot.png) -->

FilePilot reimagines the file explorer experience with native performance, multiple visualization modes, and advanced features including full-text search, remote filesystem support, collaborative file operations, and comprehensive keyboard navigation.

## Quick Start

```sh
# Install dependencies
cd frontend && npm install && cd ..

# Start development server with hot reload
make dev

# Build production binary
make build
```

The production binary is available in `build/bin/FilePilot.app`.

## Features Overview

### Views

**Column View** — Finder-style cascading columns
- Nested folder navigation with inline folder size indicators
- Click column headers to focus and explore deeper
- Smooth animations and keyboard arrow navigation

**List View** — Sortable table with metadata
- Columns: name, size, date modified, file kind
- Click headers to sort ascending/descending
- Multi-select with Shift+Click (range) or Cmd+Click (toggle)

**Grid View** — Icon grid with thumbnails
- Image and document previews as cached thumbnails
- Responsive grid layout with icon-based browsing
- Keyboard arrow navigation with visual focus ring

**Graph View** — Interactive D3.js visualization
- Force-directed graph showing directory structure
- Node clustering and physics simulation
- Zoom, pan, and click-to-navigate interactions
- Automatically laid out with collision detection

### File Operations

**Clipboard Operations**
- `Cmd+C` — Copy selected files
- `Cmd+X` — Cut selected files
- `Cmd+V` — Paste files (respects cut vs. copy semantics)
- Cross-application clipboard support (native pasteboard)

**Drag and Drop**
- Drop files to move within same volume
- Hold Option during drop to copy instead of move
- Visual feedback during drag with valid target highlighting
- Supports dragging between panes in dual-pane mode

**Batch Operations**
- Batch rename with pattern support (regex or simple token replacement)
- Compress selected files to ZIP archive
- Extract archives (ZIP, TAR, TAR.GZ, etc.)
- Duplicate files with incremental naming
- Create aliases and symlinks

**Undo Stack**
- `Cmd+Z` — Undo last operation
- Tracks clipboard operations, renames, deletions, and more
- Stack maintained per session

### Search

**Full-Text Search** — `/ ` or Cmd+Shift+F
- SQLite FTS5 index for lightning-fast queries
- Indexes file names and optionally file contents
- Folder-scoped search to limit scope
- Real-time indexing in background

**Search Methods**
- Substring matching for quick file finding
- Regex patterns for advanced filtering
- Content grep to search within file text
- Case-sensitive and case-insensitive modes

**Search Features**
- Results sorted by relevance
- Preview results before opening
- Large result set pagination
- Advanced search syntax support (AND, OR, NOT)

### Remote Filesystems

**Supported Protocols**
- SFTP — SSH File Transfer Protocol with key and password auth
- S3 — Amazon S3 and compatible services
- FTP — Classic FTP and FTPS
- WebDAV — Remote HTTP-based file access

**VFS Abstraction**
- Unified filesystem interface across local and remote
- Seamless file preview for remote files
- Remote file operations (copy, move, rename)
- Connection pooling and efficient resource usage

**Connection Manager**
- Save connection profiles with auto-fill
- macOS Keychain integration for secure credential storage
- Connection test and status indicator
- Automatic reconnection on transient failures

**Remote Preview**
- Stream preview of remote files on-demand
- Thumbnail caching for remote images
- Download on-access without full file sync

### Navigation

**Back/Forward History**
- `Cmd+[` — Navigate to previous directory
- `Cmd+]` — Navigate to next directory
- History stack per pane independent

**Breadcrumb Path Bar**
- Click any segment to jump to parent
- Autocomplete as you type path segments
- Visual indicator of current location

**Favorites Sidebar**
- Pin frequently-used folders
- Drag folders to sidebar to add
- Custom names and icons
- Reorder by dragging

**Dual-Pane and Tabs**
- `Cmd+\`` — Toggle dual-pane mode
- Independent navigation per pane
- Synchronized selection or independent operation
- Tab-based browsing within each pane

**Command Palette**
- `Cmd+K` — Open command palette
- Fuzzy search across all available commands
- Quick access to view switching, operations, and settings
- Help text for each command

### File Preview

**Code Files** — Syntax highlighting via highlight.js
- Supports 100+ languages
- Line numbering and copy selection
- Dark/light theme matching

**Images** — Inline preview and thumbnail generation
- Supports JPEG, PNG, GIF, WebP, SVG, HEIC, TIFF
- Exif metadata display (camera, settings, date)
- Animated GIF playback

**PDF** — Page-by-page rendering via PDF.js
- Navigate with arrow keys or page controls
- Zoom and fit-to-width
- Page counter and search within document

**Audio/Video** — Metadata display
- Duration, bitrate, codec information
- Album art and ID3 tags
- Playback controls for audio

**Archives** — Content listing
- ZIP, TAR, TAR.GZ, 7Z, RAR (read-only listing)
- File tree with sizes and counts
- Extract to folder or individual files

**Additional Formats**
- Hex dump for binary files
- Font preview (TTF, OTF) with sample rendering
- Property list (plist) viewer with syntax
- iCloud placeholder detection with download trigger

### Collaboration Features

**Git Status Indicators**
- Per-file git status (modified, staged, ignored, untracked)
- Color-coded status in all views
- Git-aware sorting options
- Batch operations on git status

**Finder Tags**
- Display and edit Finder tags per-file
- Batch tag application across selection
- Tag-based filtering and search
- Tag colors preserved from Finder

**File Permissions**
- View and edit POSIX permissions (755, 644, etc.)
- User, group, other permissions breakdown
- Recursive permission application
- Extended attributes display

### Performance

**Virtual Scrolling**
- Efficiently render 300+ items without lag
- Viewport-aware item rendering
- Smooth scrolling with large datasets
- Memory-efficient DOM updates

**Image Thumbnail Caching**
- Cache thumbnails on-disk for reuse
- Concurrent thumbnail generation
- Concurrency limiting to prevent resource exhaustion
- Smart cache invalidation

**Lazy Asset Loading**
- D3.js loaded only when Graph view opened
- highlight.js loaded only for code preview
- PDF.js loaded only for PDF preview
- Reduces initial bundle size

**Background Indexing**
- File index scanned in background thread
- Non-blocking main UI thread
- Incremental index updates
- Configurable index scope and frequency

## Tech Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Backend Runtime | Go | 1.23+ |
| Desktop Framework | Wails | 2.x |
| Database | SQLite (FTS5) | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) |
| Frontend | Vanilla JavaScript | ES6 modules |
| Visualization | D3.js | 7.x |
| Syntax Highlighting | highlight.js | 11.x |
| PDF Rendering | PDF.js | 4.x |
| Build Tool | Vite | 3.x |

## Prerequisites

- Go 1.23 or later
- Node.js 18 or later
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- macOS 11 or later

## Build

### Development Build (Hot Reload)

```sh
# Install frontend dependencies
cd frontend && npm install && cd ..

# Start development server
make dev
```

The development server watches both frontend and backend files, hot-reloading as you save.

### Production Build

```sh
# Install dependencies if not done
cd frontend && npm install && cd ..

# Build production binary
make build
```

The optimized production binary is output to `build/bin/FilePilot.app`.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `/` | Search files |
| `Cmd+Shift+F` | Search in current folder |
| `Cmd+K` | Command palette |
| `Escape` | Close overlay / deselect |
| `Cmd+Z` | Undo last operation |
| `Cmd+C` | Copy selected files |
| `Cmd+X` | Cut selected files |
| `Cmd+V` | Paste files |
| `Cmd+D` | Duplicate file |
| `Cmd+[` | Navigate back |
| `Cmd+]` | Navigate forward |
| `Cmd+\`` | Toggle dual pane |
| `.` | Toggle hidden files |
| `t` | Toggle dark/light theme |
| `?` | Show shortcuts help |
| `Space` | Quick Look preview |
| `Enter` | Open file / enter directory |
| `Backspace` | Go to parent directory |
| `Cmd+Shift+N` | New folder |
| `Arrow keys` | Navigate files |
| `Shift+Click` | Range select |
| `Cmd+Click` | Toggle select |
| `1` | Switch to column view |
| `2` | Switch to list view |
| `3` | Switch to grid view |
| `4` | Switch to graph view |
| `i` | Toggle info/preview panel |
| `Cmd+,` | Open settings |

## Project Structure

```
filepilot/
  main.go                  # Entry point, Wails app configuration
  app.go                   # Backend bindings (700+ lines)
  internal/
    fileops/               # File operations, disk usage, trash handling
    index/                 # SQLite FTS5 index, background scanner, watcher
    search/                # Full-text, substring, regex search
    preview/               # File preview generation (code, images, archives, etc.)
    vfs/                   # FileSystem interface: local, SFTP, FTP, S3, WebDAV
    credentials/           # macOS Keychain credential storage
    git/                   # Git status parsing
    opqueue/               # Concurrent operation queue
    server/                # HTTP asset server for thumbnails
    sync/                  # Folder comparison and sync
  frontend/
    index.html             # Main HTML shell
    src/
      css/
        theme.css          # Dark/light theme and variables
        layout.css         # Page layout and responsive design
        components.css     # Component-specific styles
      js/
        main.js            # ES module bootstrap
        app.js             # Application state and router
        views/
          column.js        # Cascading column view
          list.js          # Sortable table view
          grid.js          # Icon grid view
          graph.js         # D3.js force-directed graph
        components/
          sidebar.js       # Favorites and directory tree
          breadcrumb.js    # Path navigation bar
          preview.js       # File preview panel
          search.js        # Full-text search interface
          modal.js         # Generic modal dialogs
          context-menu.js  # Right-click context menu
          command-palette.js # Cmd+K command search
          quicklook.js     # Quick Look preview
          info-panel.js    # File info and metadata
          tag-editor.js    # Finder tag management
          rename-dialog.js # Batch rename interface
          connection-dialog.js # Remote connection setup
          diff-view.js     # Folder comparison view
          pane-container.js # Dual-pane management
        services/
          api.js           # Backend API wrapper
          clipboard.js     # Native clipboard operations
          dragdrop.js      # Drag-and-drop handlers
          shortcuts.js     # Global keyboard shortcuts
          gitstatus.js     # Git status integration
          opqueue.js       # Operation queue UI
          settings.js      # Preferences and storage
          workspaces.js    # Workspace management
        core/
          pane.js          # Pane state and lifecycle
          virtual-scroller.js # Efficient large list rendering
          lazy-libs.js     # Lazy loading for D3, highlight.js, PDF.js
```

## API Reference

### Backend Bindings (Go)

Core bindings exported to frontend via Wails:

```go
// Directory listing with metadata
ListDir(path string, options ListOptions) ([]FileInfo, error)

// File operations
Copy(src string, dst string) error
Move(src string, dst string) error
Delete(path string) error
CreateFolder(path string) error
Rename(oldPath string, newPath string) error

// Search
Search(query string, options SearchOptions) ([]FileInfo, error)
SearchFolder(folder string, query string) ([]FileInfo, error)

// Preview
GetPreview(path string) (PreviewData, error)
GetThumbnail(path string, size int) ([]byte, error)

// File info
GetFileInfo(path string) (FileInfo, error)
GetDiskUsage(path string) (DiskUsage, error)

// Undo
Undo() error

// Remote filesystems
AddRemoteConnection(config RemoteConfig) error
ListRemote(connectionID string, path string) ([]FileInfo, error)
```

### Frontend API

JavaScript services expose these core methods:

```javascript
// View management
api.listDir(path, options) // List directory contents
api.getFileInfo(path) // Get metadata
api.getThumbnail(path, size) // Get cached thumbnail

// File operations
api.copy(paths, destination) // Copy files
api.move(paths, destination) // Move files
api.delete(paths) // Delete files
api.rename(path, newName) // Rename file

// Search
api.search(query, options) // Full-text search
api.searchFolder(folder, query) // Scoped search

// Preview
api.getPreview(path) // Get preview data
api.getGitStatus(path) // Get git status

// History
history.back() // Navigate back
history.forward() // Navigate forward
```

## Development Workflow

### Making Changes

1. **Frontend changes** — Edit files in `frontend/src/`, Vite hot-reloads automatically
2. **Backend changes** — Edit Go files in `internal/`, restart `make dev`
3. **Add new view** — Create file in `frontend/src/js/views/`, register in app.js
4. **Add new component** — Create file in `frontend/src/js/components/`, import where needed

### Testing

```sh
# Run Go tests
go test ./...

# Run frontend tests
cd frontend && npm test && cd ..
```

### Building for Distribution

```sh
# Clean previous build
make clean

# Build production binary
make build

# Output is in build/bin/FilePilot.app
```

## Configuration

Settings are stored in `~/.config/filepilot/config.json`:

```json
{
  "theme": "dark",
  "defaultView": "column",
  "showHiddenFiles": false,
  "thumbnailSize": 128,
  "indexPath": "~/.cache/filepilot",
  "keyboard": {
    "searchKey": "/",
    "commandPaletteKey": "cmd+k"
  },
  "favorites": [
    "/Users/username/Documents",
    "/Users/username/Desktop"
  ]
}
```

## Performance Characteristics

- **Virtual scrolling** — Renders 300+ items smoothly
- **Full-text search** — FTS5 queries on 100k+ files return in <100ms
- **Thumbnail generation** — Concurrent pipeline with configurable concurrency (default 4)
- **Memory usage** — ~100-200MB baseline, scales with preview pane contents
- **Startup time** — <500ms to first render (after cold start)
- **Index background scanning** — Non-blocking, 0% impact on UI responsiveness

## Troubleshooting

### Search Not Finding Files

1. Check that indexing has completed — look for index progress indicator
2. Verify search scope is correct — use `Cmd+Shift+F` for folder-scoped search
3. Rebuild index — delete `~/.cache/filepilot` and restart

### Thumbnails Not Showing

1. Ensure image format is supported (JPEG, PNG, GIF, WebP, HEIC, SVG, TIFF)
2. Check that thumbnail cache directory exists: `~/.cache/filepilot/thumbnails`
3. For remote files, verify connection and bandwidth

### Remote Connection Failing

1. Test SSH/FTP credentials outside FilePilot first
2. Check that remote host is reachable: `ping hostname`
3. Verify firewall allows outbound connections on required ports
4. Review credentials in Keychain (Cmd+Space, Keychain Access)

### Performance Issues with Large Folders

1. Enable virtual scrolling (automatic in list and grid views)
2. Disable thumbnail generation for folders with 1000+ files: Settings → Preview
3. Check available disk space for index cache

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines, code style, and pull request process.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed information on:
- Backend design patterns
- VFS abstraction and remote filesystem support
- Search indexing strategy
- Performance optimization techniques
- Component architecture and lifecycle

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release notes and version history.

## License

[PolyForm Noncommercial 1.0.0](LICENSE) — FilePilot is free for personal and noncommercial use.

## Author

Samuel Bailey
