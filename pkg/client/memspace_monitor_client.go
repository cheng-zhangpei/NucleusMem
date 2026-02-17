// Package client provides HTTP clients for NucleusMem services
package client

import (
	"net/http"
	"time"
)

// MemSpaceMonitorClient is an HTTP client for interacting with MemSpaceMonitor service
type MemSpaceMonitorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMemSpaceMonitorClient creates a new client for MemSpaceMonitor
func NewMemSpaceMonitorClient(baseURL string) *MemSpaceMonitorClient {
	// Ensure URL has protocol prefix
	if !isHTTPURL(baseURL) {
		baseURL = "http://" + baseURL
	}
	return &MemSpaceMonitorClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}
