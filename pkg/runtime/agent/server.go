// Package agent provides the HTTP server for managing AI agents
package agent

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"os"
	"time"
)

// AgentHTTPServer handles HTTP requests for agent operations
type AgentHTTPServer struct {
	agent *Agent // Holds the core agent service
}

// NewAgentHTTPServer creates a new HTTP server for the given agent
func NewAgentHTTPServer(agent *Agent) *AgentHTTPServer {
	return &AgentHTTPServer{agent: agent}
}

// POST /api/v1/chat/temp
// Handles temporary chat requests that use in-memory context only
func (s *AgentHTTPServer) handleTempChat(w http.ResponseWriter, r *http.Request) {
	var req api.TempChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response, err := s.agent.TempChat(req.Message)
	resp := api.TempChatResponse{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		resp.Response = response
	}

	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/chat
// Handles persistent chat requests that interact with MemSpace (future implementation)
func (s *AgentHTTPServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var req api.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// TODO(cheng): Implement persistent chat with MemSpace integration
	response, err := s.agent.Chat(req.Message)
	resp := api.ChatResponse{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		resp.Response = response
	}
	json.NewEncoder(w).Encode(resp)
}

// GET /api/v1/health
// Returns health status of the agent
// POST /api/v1/health (changed from GET to POST to carry payload)
func (s *AgentHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.AgentHealthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Store monitor binding
	s.agent.SetBoundMonitor(req.MonitorID)

	// Perform health check on LLM client
	_, err := s.agent.chatClient.HealthCheck()
	status := "healthy"
	if err != nil {
		status = "unhealthy"
	}

	resp := api.AgentHealthResponse{
		Status:    status,
		IsJob:     s.agent.isJob,
		MonitorID: req.MonitorID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func (s *AgentHTTPServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Infof("[agent]the agent:%d get shutdown signal start shutdown process!", s.agent.AgentId)
	// 触发 Agent 自我销毁（优雅关闭）
	go func() {
		time.Sleep(100 * time.Millisecond) // 给响应时间
		os.Exit(0)                         // 或者更优雅的方式
	}()
	resp := api.ShutdownResponse{Success: true}
	json.NewEncoder(w).Encode(resp)
}

// Start initializes and starts the HTTP server
// addr should be in the format "host:port" (e.g., ":8080")
func (s *AgentHTTPServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/chat/temp", s.handleTempChat)
	mux.HandleFunc("/api/v1/agent/chat", s.handleChat)
	mux.HandleFunc("/api/v1/agent/health", s.handleHealth)
	mux.HandleFunc("/api/v1/agent/shutdown", s.handleShutdown)

	log.Infof("Agent HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
