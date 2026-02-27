// Package preview generates file previews for code, images, PDFs, archives, and other formats.
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
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	MimeType  string            `json:"mimeType"`
	Content   string            `json:"content"`
	Size      int64             `json:"size"`
	Lines     int               `json:"lines"`
	Truncated bool              `json:"truncated"`
	Language  string            `json:"language"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Entries   []string          `json:"entries,omitempty"`
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

// pdfExtensions identifies PDF files.
var pdfExtensions = map[string]bool{
	"pdf": true,
}

// audioExtensions identifies audio files.
var audioExtensions = map[string]bool{
	"mp3": true, "wav": true, "flac": true,
	"m4a": true, "aac": true, "ogg": true,
}

// videoExtensions identifies video files.
var videoExtensions = map[string]bool{
	"mp4": true, "mov": true, "mkv": true,
	"avi": true, "webm": true,
}

// archiveExtensions identifies archive files.
var archiveExtensions = map[string]bool{
	"zip": true, "tar": true, "gz": true, "tgz": true,
}

// fontExtensions identifies font files.
var fontExtensions = map[string]bool{
	"ttf": true, "otf": true, "woff": true,
}

// plistExtensions identifies property list files.
var plistExtensions = map[string]bool{
	"plist": true,
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
	// Check for iCloud placeholder files before stat.
	if CheckICloudStatus(path) {
		realName := filepath.Base(path)
		if IsICloudPlaceholder(path) {
			realName = ICloudRealName(path)
		}
		return &FilePreview{
			Path: path,
			Name: realName,
			Type: "icloud",
			Metadata: map[string]string{
				"Status": "Downloading from iCloud...",
			},
		}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// 1. Directory — unchanged.
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

	// 2. Image preview — return base64 data URI (unchanged).
	if mime, ok := imageExtensions[ext]; ok {
		return generateImage(path, name, mime, info.Size())
	}

	// 3. PDF — mdls metadata.
	if pdfExtensions[ext] {
		return generatePDF(path, name, info.Size())
	}

	// 4. Audio — mdls + afinfo.
	if audioExtensions[ext] {
		return generateAudio(path, name, info.Size())
	}

	// 5. Video — mdls.
	if videoExtensions[ext] {
		return generateVideo(path, name, info.Size())
	}

	// 6. Archive — stdlib listing.
	if archiveExtensions[ext] {
		return generateArchive(path, name, ext, info.Size())
	}

	// 7. Font — binary parse.
	if fontExtensions[ext] {
		return generateFont(path, name, info.Size())
	}

	// 8. Plist — plutil convert.
	if plistExtensions[ext] {
		return generatePlist(path, name, info.Size())
	}

	// 9. Known code extension — unchanged.
	lang := languageMap[ext]
	if lang != "" {
		return generateCode(path, name, lang, info.Size())
	}

	// 10. Known text extension — plaintext.
	if isLikelyText(ext) {
		return generateCode(path, name, "plaintext", info.Size())
	}

	// 11. Unknown ext + size < 1MB → check for binary content.
	if ext == "" || info.Size() < maxPreviewBytes {
		binary, err := hasBinaryContent(path)
		if err == nil && !binary {
			return generateCode(path, name, "plaintext", info.Size())
		}
	}

	// 12. Truly binary — hex dump.
	return generateHex(path, name, info.Size())
}

// generateImage creates a base64-encoded image preview.
func generateImage(path, name, mime string, size int64) (*FilePreview, error) {
	if size > 10<<20 {
		return &FilePreview{
			Path: path, Name: name, Type: "image",
			MimeType: mime, Size: size,
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
		Size:     size,
		Content:  fmt.Sprintf("data:%s;base64,%s", mime, b64),
	}, nil
}

// generatePDF creates a PDF preview with metadata from mdls.
func generatePDF(path, name string, size int64) (*FilePreview, error) {
	meta := mdlsMulti(path,
		"kMDItemTitle",
		"kMDItemAuthors",
		"kMDItemNumberOfPages",
	)

	md := make(map[string]string)
	if v := meta["kMDItemTitle"]; v != "" {
		md["Title"] = v
	}
	if v := meta["kMDItemAuthors"]; v != "" {
		// mdls returns arrays as "(\n    \"Name\"\n)" — extract the value.
		v = cleanMDLSArray(v)
		if v != "" {
			md["Author"] = v
		}
	}
	if v := meta["kMDItemNumberOfPages"]; v != "" {
		md["Pages"] = v
	}

	return &FilePreview{
		Path:     path,
		Name:     name,
		Type:     "pdf",
		Size:     size,
		Metadata: md,
	}, nil
}

// generateAudio creates an audio preview with metadata from mdls and afinfo.
func generateAudio(path, name string, size int64) (*FilePreview, error) {
	meta := mdlsMulti(path,
		"kMDItemDurationSeconds",
		"kMDItemAudioBitRate",
		"kMDItemAuthors",
		"kMDItemAlbum",
		"kMDItemTitle",
	)

	md := make(map[string]string)
	if v := meta["kMDItemTitle"]; v != "" {
		md["Title"] = v
	}
	if v := meta["kMDItemAuthors"]; v != "" {
		v = cleanMDLSArray(v)
		if v != "" {
			md["Artist"] = v
		}
	}
	if v := meta["kMDItemAlbum"]; v != "" {
		md["Album"] = v
	}

	// Duration: prefer mdls, fall back to afinfo.
	if v := meta["kMDItemDurationSeconds"]; v != "" {
		md["Duration"] = formatMDLSDuration(v)
	} else {
		if v := afinfoAttr(path, "estimated duration"); v != "" {
			md["Duration"] = formatMDLSDuration(v)
		}
	}

	if v := meta["kMDItemAudioBitRate"]; v != "" {
		// Convert from bits/s to kbps.
		if f, err := parseFloat(v); err == nil {
			md["Bitrate"] = formatFloat(f/1000, 0) + " kbps"
		}
	} else {
		if v := afinfoAttr(path, "bit rate"); v != "" {
			md["Bitrate"] = v
		}
	}

	return &FilePreview{
		Path:     path,
		Name:     name,
		Type:     "audio",
		Size:     size,
		Metadata: md,
	}, nil
}

// generateVideo creates a video preview with metadata from mdls.
func generateVideo(path, name string, size int64) (*FilePreview, error) {
	meta := mdlsMulti(path,
		"kMDItemDurationSeconds",
		"kMDItemPixelWidth",
		"kMDItemPixelHeight",
		"kMDItemCodecs",
		"kMDItemTitle",
	)

	md := make(map[string]string)
	if v := meta["kMDItemTitle"]; v != "" {
		md["Title"] = v
	}
	if v := meta["kMDItemDurationSeconds"]; v != "" {
		md["Duration"] = formatMDLSDuration(v)
	}
	w := meta["kMDItemPixelWidth"]
	h := meta["kMDItemPixelHeight"]
	if w != "" && h != "" {
		md["Resolution"] = w + " x " + h
	}
	if v := meta["kMDItemCodecs"]; v != "" {
		v = cleanMDLSArray(v)
		if v != "" {
			md["Codec"] = v
		}
	}

	return &FilePreview{
		Path:     path,
		Name:     name,
		Type:     "video",
		Size:     size,
		Metadata: md,
	}, nil
}

// generateArchive creates an archive preview with a file listing.
func generateArchive(path, name, ext string, size int64) (*FilePreview, error) {
	var entries []string
	var total int
	var err error

	switch ext {
	case "zip":
		entries, total, err = previewZip(path)
	case "tar", "gz", "tgz":
		entries, total, err = previewTar(path)
	}

	md := make(map[string]string)
	if err == nil {
		md["Files"] = fmt.Sprintf("%d", total)
	} else {
		md["Error"] = err.Error()
	}

	return &FilePreview{
		Path:     path,
		Name:     name,
		Type:     "archive",
		Size:     size,
		Metadata: md,
		Entries:  entries,
	}, nil
}

// generateFont creates a font preview with the family name.
func generateFont(path, name string, size int64) (*FilePreview, error) {
	md := make(map[string]string)
	family := fontFamilyName(path)
	if family != "" {
		md["Family"] = family
	}

	return &FilePreview{
		Path:     path,
		Name:     name,
		Type:     "font",
		Size:     size,
		Metadata: md,
	}, nil
}

// generatePlist creates a plist preview with syntax-highlighted XML.
func generatePlist(path, name string, size int64) (*FilePreview, error) {
	content, err := previewPlist(path)
	if err != nil {
		return &FilePreview{
			Path: path,
			Name: name,
			Type: "binary",
			Size: size,
		}, nil
	}

	// Truncate to maxPreviewLines.
	lines := strings.Count(content, "\n") + 1
	truncated := false
	if lines > maxPreviewLines {
		parts := strings.SplitN(content, "\n", maxPreviewLines+1)
		content = strings.Join(parts[:maxPreviewLines], "\n")
		truncated = true
	}

	return &FilePreview{
		Path:      path,
		Name:      name,
		Type:      "code",
		Size:      size,
		Content:   content,
		Lines:     lines,
		Truncated: truncated,
		Language:  "xml",
	}, nil
}

// generateCode creates a text/code preview.
func generateCode(path, name, lang string, size int64) (*FilePreview, error) {
	content, lines, truncated, err := readTextPreview(path)
	if err != nil {
		return nil, err
	}

	return &FilePreview{
		Path:      path,
		Name:      name,
		Type:      "code",
		Size:      size,
		Content:   content,
		Lines:     lines,
		Truncated: truncated,
		Language:  lang,
	}, nil
}

// generateHex creates a hex dump preview of the first 256 bytes.
func generateHex(path, name string, size int64) (*FilePreview, error) {
	content, err := hexDump(path)
	if err != nil {
		return &FilePreview{
			Path: path,
			Name: name,
			Type: "binary",
			Size: size,
		}, nil
	}

	return &FilePreview{
		Path:    path,
		Name:    name,
		Type:    "hex",
		Size:    size,
		Content: content,
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
		"rtf": true, "srt": true, "sub": true, "vtt": true,
		"ics": true, "vcf": true, "pgp": true, "asc": true,
		"pem": true, "crt": true, "key": true, "pub": true,
		"service": true, "timer": true, "socket": true,
		"desktop": true, "rules": true, "list": true,
		"babelrc": true, "browserslistrc": true, "stylelintrc": true,
		"huskyrc": true, "lintstagedrc": true,
	}
	return textExts[ext]
}

// cleanMDLSArray extracts values from an mdls array output like:
//
//	(\n    "Value1",\n    "Value2"\n)
//
// Returns a comma-separated string of values.
func cleanMDLSArray(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return s
	}
	s = strings.Trim(s, "()")
	var parts []string
	for _, line := range strings.Split(s, ",") {
		v := strings.TrimSpace(line)
		v = strings.Trim(v, "\"")
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}
