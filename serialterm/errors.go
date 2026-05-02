package serialterm

import "strings"

// IsPermissionDenied reports whether an error message string from a failed
// serial.Open resembles a Unix permission-denied error. Device views use
// this to surface a platform-specific "add your user to the dialout group"
// hint on Linux.
func IsPermissionDenied(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "permission denied") ||
		strings.Contains(m, "access denied") ||
		strings.Contains(m, "eacces")
}
