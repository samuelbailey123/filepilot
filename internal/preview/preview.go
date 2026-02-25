package preview

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FilePreview contains rendered preview data for a file.
type FilePreview struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	MimeType  string `json:"mimeType"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Lines     int    `json:"lines"`
	Truncated bool   `json:"truncated"`
	Language  string `json:"language"`
}

// maxPreviewLines is the maximum number of lines to read for text previews.
const maxPreviewLines = 200

// maxPreviewBytes is the maximum bytes to read (1 MB).
const maxPreviewBytes = 1 << 20

// imageExtensions maps image file extensions to MIME types.
var imageExtensions = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"svg":  "image/svg+xml",
	"webp": "image/webp",
	"ico":  "image/x-icon",
	"bmp":  "image/bmp",
}

// languageMap maps file extensions to highlight.js language identifiers.
var languageMap = map[string]string{
	"go":         "go",
	"js":         "javascript",
	"mjs":        "javascript",
	"cjs":        "javascript",
	"ts":         "typescript",
	"tsx":        "typescript",
	"jsx":        "javascript",
	"py":         "python",
	"rb":         "ruby",
	"rs":         "rust",
	"java":       "java",
	"kt":         "kotlin",
	"swift":      "swift",
	"c":          "c",
	"h":          "c",
	"cpp":        "cpp",
	"cc":         "cpp",
	"hpp":        "cpp",
	"cs":         "csharp",
	"php":        "php",
	"sql":        "sql",
	"sh":         "bash",
	"bash":       "bash",
	"zsh":        "bash",
	"fish":       "bash",
	"html":       "html",
	"htm":        "html",
	"css":        "css",
	"scss":       "scss",
	"less":       "less",
	"json":       "json",
	"yaml":       "yaml",
	"yml":        "yaml",
	"toml":       "ini",
	"xml":        "xml",
	"md":         "markdown",
	"markdown":   "markdown",
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
	"lua":        "lua",
	"r":          "r",
	"dart":       "dart",
	"ex":         "elixir",
	"exs":        "elixir",
	"erl":        "erlang",
	"hs":         "haskell",
	"tf":         "hcl",
	"hcl":        "hcl",
	"proto":      "protobuf",
	"graphql":    "graphql",
	"gql":        "graphql",
	"vim":        "vim",
	"ini":        "ini",
	"conf":       "ini",
	"cfg":        "ini",
}

// Generate creates a preview for the given file path.
func Generate(path string) (*FilePreview, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if info.IsDir() {
		return &FilePreview{
			Path: path,
			Name: filepath.Base(path),
			Type: "directory",
			Size: info.Size(),
		}, nil
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	name := filepath.Base(path)

	// Handle special filenames without extensions.
	lowerName := strings.ToLower(name)
	if ext == "" {
		switch {
		case lowerName == "dockerfile":
			ext = "dockerfile"
		case lowerName == "makefile":
			ext = "makefile"
		case lowerName == "gemfile", lowerName == "rakefile":
			ext = "rb"
		}
	}

	// Image preview — return base64 data URI.
	if mime, ok := imageExtensions[ext]; ok {
		if info.Size() > 10<<20 {
			return &FilePreview{
				Path: path, Name: name, Type: "image",
				MimeType: mime, Size: info.Size(),
				Content: "Image too large for preview",
			}, nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return &FilePreview{
			Path:     path,
			Name:     name,
			Type:     "image",
			MimeType: mime,
			Size:     info.Size(),
			Content:  fmt.Sprintf("data:%s;base64,%s", mime, b64),
		}, nil
	}

	// Text/code preview.
	lang := languageMap[ext]
	if lang == "" && isLikelyText(ext) {
		lang = "plaintext"
	}
	if lang == "" {
		// Binary or unknown file.
		return &FilePreview{
			Path: path,
			Name: name,
			Type: "binary",
			Size: info.Size(),
		}, nil
	}

	content, lines, truncated, err := readTextPreview(path)
	if err != nil {
		return nil, err
	}

	return &FilePreview{
		Path:      path,
		Name:      name,
		Type:      "code",
		Size:      info.Size(),
		Content:   content,
		Lines:     lines,
		Truncated: truncated,
		Language:  lang,
	}, nil
}

// readTextPreview reads up to maxPreviewLines lines from a file.
func readTextPreview(path string) (content string, lines int, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, false, err
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxPreviewBytes)

	for scanner.Scan() {
		lines++
		if lines > maxPreviewLines {
			truncated = true
			break
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return sb.String(), lines, truncated, err
	}

	return sb.String(), lines, truncated, nil
}

// isLikelyText returns true for extensions commonly associated with text files.
func isLikelyText(ext string) bool {
	textExts := map[string]bool{
		"txt": true, "log": true, "csv": true, "tsv": true,
		"env": true, "gitignore": true, "gitattributes": true,
		"editorconfig": true, "prettierrc": true, "eslintrc": true,
		"npmrc": true, "nvmrc": true, "dockerignore": true,
		"properties": true, "lock": true, "sum": true, "mod": true,
	}
	return textExts[ext]
}
