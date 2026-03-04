//go:build !unix

package metrics

// countFDsFallback is a no-op on non-Unix platforms.
func countFDsFallback() (open int, max int) {
	return -1, -1
}

// maxFDsFallback is a no-op on non-Unix platforms.
func maxFDsFallback() int {
	return -1
}
