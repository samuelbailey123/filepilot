# Changelog

All notable changes to Decima Explorer are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] — 2026-02-27

### Added
- Lazy-load of D3.js, PDF.js, and highlight.js on first use (540KB saved at startup)
- Go package-level documentation comments for all 11 packages
- Comprehensive README rewrite with complete feature documentation
- Architecture documentation (ARCHITECTURE.md)
- Custom FilePilot app icon with folder and compass needle motif

### Changed
- Replaced binding polling with requestAnimationFrame for faster startup detection
- Implemented event delegation on list, grid, and column views to reduce listener count
- Version alignment to 0.7.0 across all configuration files

### Removed
- Personal email address from build configuration

## [0.6.0] — 2026-02-27

### Added
- Archive extraction support (ZIP, tar.gz, tar.bz2) to chosen directory
- Open Terminal at current directory feature
- Image thumbnail generation with 6-worker concurrent loading
- Content grep (full-text search within file contents)
- Operation queue with pause, resume, and cancel controls
- Folder size calculation displayed in list and column views
- Finder tag display with colored dots per file

### Improved
- Performance optimizations for large directory handling

## [0.5.0] — 2026-02-26

### Added
- Command palette with Cmd+K and fuzzy path completion
- Quick Look overlay (Space key) for rapid file preview
- Symlink creation and display with target indicators
- File compression to ZIP archives
- File permissions viewer in info panel
- Batch tag editing across multiple selected files
- Diff view for comparing two files side-by-side
- Open With menu showing registered applications
- Duplicate file operation (Cmd+D)
- Directory picker dialog for choosing operation targets

## [0.4.0] — 2026-02-26

### Added
- SFTP filesystem support with password and key authentication
- S3 filesystem support with bucket browsing
- FTP and WebDAV filesystem support
- Connection manager dialog with saved connections
- macOS Keychain integration for secure credential storage
- Remote file preview via temporary file download
- Folder comparison across local and remote filesystems
- Bidirectional folder synchronization planner

## [0.3.0] — 2026-02-26

### Added
- Dual-pane mode with independent navigation (Cmd+`)
- Tab support within each pane
- Clipboard cut/copy/paste operations (Cmd+C/X/V) with visual indicators
- Batch rename with find/replace, prefix/suffix, and numbering options
- Git status indicators per file (modified, added, deleted, untracked)
- Git repository root detection
- Workspace save and restore for layout persistence
- Custom keymap configuration
- Settings overlay with user preferences

## [0.2.0] — 2026-02-26

### Added
- Virtual filesystem abstraction layer (FileSystem interface)
- Local filesystem implementation wrapping os package
- Connection dialog UI for adding remote connections
- Remote path format support (remote://{connID}/{path})
- Sidebar sections for favorites, locations, and network connections
- Volume detection and eject support
- Background file index watcher for real-time updates

## [0.1.0] — 2026-02-26

### Added
- Column view with Finder-style cascading navigation
- List view with sortable columns (name, size, date, kind)
- Grid view with icon layout
- Graph view with D3.js force-directed visualization
- File preview for code with syntax highlighting
- File preview for images, PDF, audio, video, archives, fonts, plists, and hex dump
- SQLite FTS5 full-text search with folder scoping
- iCloud placeholder file detection and download triggering
- Disk usage display with per-volume bars
- Drag-and-drop file move and copy operations
- Keyboard shortcuts with help overlay
- Dark and light themes with warm amber palette
- Full ARIA accessibility (roles, live regions, skip links, focus management)
