package preview

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// hasBinaryContent reads up to the first 8KB of a file and returns true
// if any null bytes are found, indicating binary content.
func hasBinaryContent(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil
		}
	}
	return false, nil
}

// hexDump reads up to the first 256 bytes of a file and returns a
// formatted hex dump string with an ASCII column on the right side.
// Each line shows 16 bytes: offset, hex values, and printable ASCII.
func hexDump(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 256)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	buf = buf[:n]

	var sb strings.Builder
	for offset := 0; offset < n; offset += 16 {
		end := offset + 16
		if end > n {
			end = n
		}
		line := buf[offset:end]

		// Offset column.
		fmt.Fprintf(&sb, "%08x  ", offset)

		// Hex bytes.
		for i := 0; i < 16; i++ {
			if i < len(line) {
				fmt.Fprintf(&sb, "%02x ", line[i])
			} else {
				sb.WriteString("   ")
			}
			if i == 7 {
				sb.WriteByte(' ')
			}
		}

		// ASCII column.
		sb.WriteString(" |")
		for _, b := range line {
			if b >= 0x20 && b <= 0x7e {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}

	return sb.String(), nil
}
