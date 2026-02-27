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

// MockToolServer simulates external tool endpoints for testing
// It tracks all calls and returns deterministic mock responses
type MockToolServer struct {
	server   *http.Server
	listener net.Listener
	addr     string
	mu       sync.RWMutex
	calls    map[string][]CallRecord // toolName -> list of calls
}

// CallRecord captures a single tool invocation for verification
type CallRecord struct {
	Timestamp int64                  `json:"timestamp"`
	Params    map[string]interface{} `json:"params"`
}

// NewMockToolServer creates a mock server on a random available port
func NewMockToolServer() (*MockToolServer, error) {
	// Let OS assign an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	mts := &MockToolServer{
		listener: listener,
		addr:     listener.Addr().String(),
		calls:    make(map[string][]CallRecord),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mock_lint", mts.handleMockLint)
	mux.HandleFunc("/mock_test", mts.handleMockTest)
	mux.HandleFunc("/mock_report", mts.handleMockReport)
	mux.HandleFunc("/health", mts.handleHealth)

	mts.server = &http.Server{
		Addr:    mts.addr,
		Handler: mux,
		// Prevent test hangs
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return mts, nil
}

// Start begins serving HTTP requests
func (mts *MockToolServer) Start() error {
	go func() {
		// Ignore ErrServerClosed on graceful shutdown
		if err := mts.server.Serve(mts.listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[MockToolServer] error: %v\n", err)
		}
	}()
	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Stop gracefully shuts down the server
func (mts *MockToolServer) Stop(ctx context.Context) error {
	return mts.server.Shutdown(ctx)
}

// BaseURL returns the full base URL for HTTP clients
func (mts *MockToolServer) BaseURL() string {
	return "http://" + mts.addr
}

// ============================================================
// Call Tracking & Verification
// ============================================================

// recordCall logs a tool invocation (thread-safe)
func (mts *MockToolServer) recordCall(toolName string, params map[string]interface{}) {
	mts.mu.Lock()
	defer mts.mu.Unlock()
	mts.calls[toolName] = append(mts.calls[toolName], CallRecord{
		Timestamp: time.Now().UnixMilli(),
		Params:    params,
	})
}

// GetCalls returns all recorded calls for a tool (for test assertions)
func (mts *MockToolServer) GetCalls(toolName string) []CallRecord {
	mts.mu.RLock()
	defer mts.mu.RUnlock()
	// Return a copy to avoid race conditions in tests
	calls := mts.calls[toolName]
	result := make([]CallRecord, len(calls))
	copy(result, calls)
	return result
}

// GetCallCount returns total invocations for a tool
func (mts *MockToolServer) GetCallCount(toolName string) int {
	return len(mts.GetCalls(toolName))
}

// Reset clears all recorded calls (use between test cases)
func (mts *MockToolServer) Reset() {
	mts.mu.Lock()
	defer mts.mu.Unlock()
	mts.calls = make(map[string][]CallRecord)
}

// GetCallTimestamps returns sorted timestamps for concurrency verification
func (mts *MockToolServer) GetCallTimestamps(toolName string) []int64 {
	calls := mts.GetCalls(toolName)
	timestamps := make([]int64, len(calls))
	for i, c := range calls {
		timestamps[i] = c.Timestamp
	}
	return timestamps
}

// ============================================================
// HTTP Handlers (Mock Tool Endpoints)
// ============================================================

func (mts *MockToolServer) handleMockLint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path   string `json:"path"`
		Config string `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	mts.recordCall("mock_lint", map[string]interface{}{
		"path":   req.Path,
		"config": req.Config,
	})

	// Simulate lint analysis
	resp := map[string]interface{}{
		"tool":    "mock_lint",
		"success": true,
		"results": map[string]interface{}{
			"issues":        0,
			"files_checked": 10,
			"metrics":       map[string]float64{"complexity": 3.2},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mts *MockToolServer) handleMockTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Coverage bool `json:"coverage,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	mts.recordCall("mock_test", map[string]interface{}{
		"coverage": req.Coverage,
	})

	resp := map[string]interface{}{
		"tool":    "mock_test",
		"success": true,
		"results": map[string]interface{}{
			"passed":   42,
			"failed":   0,
			"coverage": "85%",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mts *MockToolServer) handleMockReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Format string `json:"format,omitempty"`
		// These fields would come from upstream tools in real flow
		LintMetrics map[string]interface{} `json:"lint_metrics,omitempty"`
		TestResults map[string]interface{} `json:"test_results,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	mts.recordCall("mock_report", map[string]interface{}{
		"format":   req.Format,
		"has_lint": req.LintMetrics != nil,
		"has_test": req.TestResults != nil,
	})

	resp := map[string]interface{}{
		"tool":    "mock_report",
		"success": true,
		"results": map[string]interface{}{
			"summary": "All checks passed. Code quality: A",
			"format":  req.Format,
		},
	}
	w.Header().Set("Content-Type", "application/json")
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
