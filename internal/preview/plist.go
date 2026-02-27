package preview

import (
	"context"
	"os"
	"os/exec"
)

// plistBinaryMagic is the magic bytes that identify a binary plist file.
var plistBinaryMagic = []byte("bplist")

// previewPlist reads a plist file and returns its XML representation.
// Binary plists are converted to XML via macOS plutil. XML plists are
// returned directly. Returns the XML string and any error.
func previewPlist(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// If it's already XML, return as-is.
	if !isBinaryPlist(data) {
		return string(data), nil
	}

	// Convert binary plist to XML using plutil.
	ctx, cancel := context.WithTimeout(context.Background(), mdlsTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "plutil", "-convert", "xml1", "-o", "-", path).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// isBinaryPlist checks if the data starts with the binary plist magic bytes.
func isBinaryPlist(data []byte) bool {
	if len(data) < len(plistBinaryMagic) {
		return false
	}
	for i, b := range plistBinaryMagic {
		if data[i] != b {
			return false
		}
	}
	return true
}
