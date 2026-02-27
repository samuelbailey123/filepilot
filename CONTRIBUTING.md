# Contributing to FilePilot

## Development Setup

1. Clone the repository
2. Install prerequisites: Go 1.23+, Node.js 18+, Wails CLI
3. Install frontend dependencies: `cd frontend && npm install`
4. Start the dev server: `wails dev`

## Architecture

For a detailed overview of how the backend and frontend are structured, see [ARCHITECTURE.md](ARCHITECTURE.md). Key concepts:

- **11 Go packages** under `internal/` — each with a single responsibility
- **37+ JS modules** organized into views, components, services, and core
- **VFS abstraction** for local and remote filesystem access
- **SQLite FTS5** for full-text search indexing

## Project Structure

FilePilot uses a Go backend (Wails v2) with a vanilla JavaScript frontend. There is no frontend framework — the UI is built with plain ES6 modules, DOM manipulation, and CSS custom properties.

- **Go backend** (`main.go`, `app.go`, `internal/`) — file operations, SQLite indexing, search, previews
- **JavaScript frontend** (`frontend/src/js/`) — views, components, services
- **CSS** (`frontend/src/css/`) — design tokens in `theme.css`, layout in `layout.css`, component styles in `components.css`

## Code Style

### JavaScript
- Vanilla ES6 modules, no TypeScript
- Use `var` and `function` declarations (ES5-compatible style throughout)
- No build-time transpilation beyond Vite bundling

### Go
- Standard `gofmt` formatting
- All packages must have `// Package ...` doc comments
- Run `go vet ./...` before submitting

### CSS
- CSS custom properties for all colors and spacing
- No preprocessors

## Testing

### Go Tests

```sh
# Run all tests with race detector
go test -race -count=1 ./...

# Run tests for a specific package
go test -race ./internal/fileops/

# Run with verbose output
go test -v -race ./...

# Check code correctness
go vet ./...
```

### Frontend Build Verification

```sh
# Build frontend to verify no module errors
cd frontend && npx vite build

# Start dev server for manual testing
cd frontend && npm run dev
```

### Manual Testing Checklist

- [ ] `wails dev` — app opens correctly
- [ ] Navigate to a large directory (500+ files) — virtual scrolling works
- [ ] Switch to Graph view — D3.js loads on demand
- [ ] Preview a code file — syntax highlighting works
- [ ] Preview a PDF — PDF.js loads on demand
- [ ] Test drag and drop between directories
- [ ] Test Cmd+C/X/V clipboard operations
- [ ] Test search (/, Cmd+Shift+F)
- [ ] Verify dark/light theme toggle (t)

## Pull Requests

1. Branch from `main`
2. Describe your changes clearly
3. One commit per file for clean separation
4. Run `go vet ./...` and `go test -race ./...` before submitting
5. Ensure `cd frontend && npx vite build` completes without errors

## License

By contributing, you agree that your contributions will be licensed under the [PolyForm Noncommercial 1.0.0](LICENSE) license.
