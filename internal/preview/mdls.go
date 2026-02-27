package preview

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// mdlsTimeout is the maximum time allowed for mdls/afinfo commands.
const mdlsTimeout = 2 * time.Second

// mdlsMulti queries multiple Spotlight metadata attributes for a file
// and returns them as a map. Attributes with "(null)" values are omitted.
func mdlsMulti(path string, attrs ...string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), mdlsTimeout)
	defer cancel()

	args := make([]string, 0, len(attrs)*2+1)
	for _, a := range attrs {
		args = append(args, "-name", a)
	}
	args = append(args, path)

	out, err := exec.CommandContext(ctx, "mdls", args...).Output()
	if err != nil {
		return nil
	}

	result := make(map[string]string, len(attrs))
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip surrounding quotes from string values.
		val = strings.Trim(val, "\"")
		if val == "" || val == "(null)" {
			continue
		}
		result[key] = val
	}
	return result
}

// afinfoAttr queries a single attribute from afinfo for an audio file.
// Returns an empty string on any error.
func afinfoAttr(path, attr string) string {
	ctx, cancel := context.WithTimeout(context.Background(), mdlsTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "afinfo", path).Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, attr) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// formatMDLSDuration converts an mdls duration value (seconds as float)
// to a human-readable "Xm Ys" or "Xs" string.
func formatMDLSDuration(val string) string {
	val = strings.TrimSpace(val)
	if val == "" || val == "(null)" {
		return ""
	}

	var secs float64
	n, err := parseFloat(val)
	if err != nil {
		return val
	}
	secs = n

	if secs < 60 {
		return formatFloat(secs, 1) + "s"
	}
	mins := int(secs) / 60
	remSecs := secs - float64(mins*60)
	return formatInt(mins) + "m " + formatFloat(remSecs, 0) + "s"
}

// parseFloat parses a string as float64 without importing strconv
// to keep the helper self-contained within the package.
func parseFloat(s string) (float64, error) {
	// Use fmt.Sscanf for simplicity.
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// formatFloat formats a float with the given number of decimal places.
func formatFloat(f float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, f)
}

// formatInt formats an integer.
func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}
