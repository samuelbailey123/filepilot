package preview

import (
	"encoding/binary"
	"io"
	"os"
)

// fontFamilyName parses the OpenType/TrueType name table to extract
// the font family name (nameID 1). Returns an empty string if the
// name cannot be determined.
func fontFamilyName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Read the offset table header (12 bytes).
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return ""
	}

	numTables := binary.BigEndian.Uint16(header[4:6])

	// Scan table records for the 'name' table.
	var nameOffset, nameLength uint32
	for i := uint16(0); i < numTables; i++ {
		var rec [16]byte
		if _, err := io.ReadFull(f, rec[:]); err != nil {
			return ""
		}
		tag := string(rec[0:4])
		if tag == "name" {
			nameOffset = binary.BigEndian.Uint32(rec[8:12])
			nameLength = binary.BigEndian.Uint32(rec[12:16])
			break
		}
	}
	if nameOffset == 0 || nameLength == 0 {
		return ""
	}

	// Read the entire name table.
	if nameLength > 64*1024 {
		return "" // Sanity limit.
	}
	nameTable := make([]byte, nameLength)
	if _, err := f.ReadAt(nameTable, int64(nameOffset)); err != nil {
		return ""
	}

	if len(nameTable) < 6 {
		return ""
	}

	count := binary.BigEndian.Uint16(nameTable[2:4])
	storageOffset := binary.BigEndian.Uint16(nameTable[4:6])

	// Scan name records (12 bytes each) looking for nameID 1 (font family).
	for i := uint16(0); i < count; i++ {
		recStart := 6 + int(i)*12
		if recStart+12 > len(nameTable) {
			break
		}
		rec := nameTable[recStart : recStart+12]

		platformID := binary.BigEndian.Uint16(rec[0:2])
		nameID := binary.BigEndian.Uint16(rec[6:8])
		strLength := binary.BigEndian.Uint16(rec[8:10])
		strOffset := binary.BigEndian.Uint16(rec[10:12])

		if nameID != 1 {
			continue
		}

		start := int(storageOffset) + int(strOffset)
		end := start + int(strLength)
		if end > len(nameTable) {
			continue
		}

		raw := nameTable[start:end]

		// Platform 1 (Macintosh) uses single-byte encoding.
		if platformID == 1 {
			return string(raw)
		}

		// Platform 0 (Unicode) and 3 (Windows) use UTF-16BE.
		if platformID == 0 || platformID == 3 {
			return decodeUTF16BE(raw)
		}
	}
	return ""
}

// decodeUTF16BE decodes a UTF-16 big-endian byte slice to a Go string.
func decodeUTF16BE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		runes = append(runes, rune(binary.BigEndian.Uint16(b[i:i+2])))
	}
	return string(runes)
}
