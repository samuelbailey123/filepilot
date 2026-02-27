package preview

import (
	"os"
	"path/filepath"
	"strings"
)

// IsICloudPlaceholder returns true if the file at path is an iCloud
// placeholder (not yet downloaded). On macOS, iCloud Drive stores
// placeholders as hidden files named ".FILENAME.icloud" with zero or
// minimal size while the real content is in the cloud.
func IsICloudPlaceholder(path string) bool {
	name := filepath.Base(path)
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".icloud")
}

// ICloudRealName extracts the actual filename from an iCloud placeholder
// name. ".Document.pdf.icloud" → "Document.pdf".
func ICloudRealName(path string) string {
	name := filepath.Base(path)
	// Strip leading "." and trailing ".icloud".
	name = strings.TrimPrefix(name, ".")
	name = strings.TrimSuffix(name, ".icloud")
	return name
}

// CheckICloudStatus checks whether the given file path is an iCloud
// placeholder or if the real file exists but hasn't finished downloading.
// Returns true if the file is pending download.
func CheckICloudStatus(path string) bool {
	// Direct placeholder check.
	if IsICloudPlaceholder(path) {
		return true
	}

	// Check if a placeholder exists for a normal-looking path.
	// When the user clicks "Document.pdf", the actual file on disk might
	// be ".Document.pdf.icloud" if it hasn't been downloaded yet.
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	placeholderPath := filepath.Join(dir, "."+name+".icloud")

	if _, err := os.Stat(placeholderPath); err == nil {
		// The placeholder exists — file is pending iCloud download.
		// Also check if the real file is missing or zero-size.
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			return true
		}
	}

	return false
}
