package agent_manager

import "strings"

// EnsureHTTPPrefix ensures the address has an HTTP protocol prefix
// Converts "localhost:8081" → "http://localhost:8081"
// Leaves "http://..." or "https://..." unchanged
func EnsureHTTPPrefix(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}
