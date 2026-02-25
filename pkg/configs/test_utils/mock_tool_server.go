package test_utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// MockToolServer is a test HTTP server that simulates tool endpoints
type MockToolServer struct {
	server      *http.Server
	addr        string
	wg          sync.WaitGroup
	mu          sync.RWMutex
	callCount   int
	lastRequest map[string]interface{}
}

// MockLintRequest represents the expected request format
type MockLintRequest struct {
	Path   string `json:"path"`
	Config string `json:"config,omitempty"`
}

// MockLintResponse represents the mock response
type MockLintResponse struct {
	Success    bool     `json:"success"`
	ToolName   string   `json:"tool_name"`
	Path       string   `json:"path"`
	Violations []string `json:"violations,omitempty"`
	Message    string   `json:"message"`
	Timestamp  int64    `json:"timestamp"`
}

// NewMockToolServer creates a new mock server on a random available port
func NewMockToolServer() *MockToolServer {
	mts := &MockToolServer{
		addr:        "localhost:30000", // Let OS assign available port
		lastRequest: make(map[string]interface{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/lint", mts.handleLint)
	mux.HandleFunc("/health", mts.handleHealth)

	mts.server = &http.Server{
		Addr:    mts.addr,
		Handler: mux,
	}

	return mts
}

// Start begins listening on the assigned port
func (mts *MockToolServer) Start() error {
	// Create listener to get actual port
	listener, err := net.Listen("tcp", mts.addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	mts.addr = listener.Addr().String()

	mts.wg.Add(1)
	go func() {
		defer mts.wg.Done()
		if err := mts.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Mock server error: %v\n", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop gracefully shuts down the server
func (mts *MockToolServer) Stop(ctx context.Context) error {
	return mts.server.Shutdown(ctx)
}

// Addr returns the server address (e.g., "localhost:18080")
func (mts *MockToolServer) Addr() string {
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	return mts.addr
}

// BaseURL returns the full base URL (e.g., "http://localhost:18080")
func (mts *MockToolServer) BaseURL() string {
	return "http://" + mts.Addr()
}

// GetCallCount returns the number of tool calls received
func (mts *MockToolServer) GetCallCount() int {
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	return mts.callCount
}

// GetLastRequest returns the last request parameters (for verification)
func (mts *MockToolServer) GetLastRequest() map[string]interface{} {
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	return mts.lastRequest
}

// Reset resets call count and last request (for multiple test cases)
func (mts *MockToolServer) Reset() {
	mts.mu.Lock()
	defer mts.mu.Unlock()
	mts.callCount = 0
	mts.lastRequest = make(map[string]interface{})
}

// ============================================================
// HTTP Handlers
// ============================================================

func (mts *MockToolServer) handleLint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MockLintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Record call
	mts.mu.Lock()
	mts.callCount++
	mts.lastRequest = map[string]interface{}{
		"path":   req.Path,
		"config": req.Config,
	}
	mts.mu.Unlock()

	// Simulate lint check
	violations := []string{}
	if req.Path != "" {
		violations = append(violations,
			fmt.Sprintf("Unused variable in %s: line 42", req.Path),
			fmt.Sprintf("Missing semicolon in %s: line 108", req.Path),
		)
	}

	resp := MockLintResponse{
		Success:    true,
		ToolName:   "mock_lint",
		Path:       req.Path,
		Violations: violations,
		Message:    fmt.Sprintf("Found %d violations in %s", len(violations), req.Path),
		Timestamp:  time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (mts *MockToolServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":    "healthy",
		"service":   "mock-tool-server",
		"timestamp": time.Now().Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
