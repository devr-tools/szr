//go:build windows

package engine

// Windows has no ETXTBSY; exec start failures are never busy-retryable.
func isExecTextFileBusy(error) bool {
	return false
}
