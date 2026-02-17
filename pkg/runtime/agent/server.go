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

func (s *AgentHTTPServer) handleTempChat(w http.ResponseWriter, r *http.Request) {
	var req api.TempChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Submit as TempChat task
	task := AgentTask{
		Type:    TaskTypeTempChat,
		Content: req.Message,
	}
	if err := s.agent.SubmitTask(task); err != nil {
		http.Error(w, "Failed to submit task", http.StatusInternalServerError)
		return
	}

	// Immediate ack (async processing)
	resp := api.TempChatResponse{Success: true}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/agent/chat
func (s *AgentHTTPServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var req api.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Submit as Chat task
	task := AgentTask{
		Type:    TaskTypeChat,
		Content: req.Message,
	}
	if err := s.agent.SubmitTask(task); err != nil {
		http.Error(w, "Failed to submit task", http.StatusInternalServerError)
		return
	}

	resp := api.ChatResponse{Success: true}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/agent/notify
func (s *AgentHTTPServer) handleNotify(w http.ResponseWriter, r *http.Request) {
	var req api.NotifyRequest // ← 你需要定义这个结构
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Submit as Comm task
	task := AgentTask{
		Type:    TaskTypeComm,
		Key:     req.Key,
		Content: req.Content,
	}
	if err := s.agent.SubmitTask(task); err != nil {
		http.Error(w, "Failed to submit notify task", http.StatusInternalServerError)
		return
	}
	resp := api.NotifyResponse{Success: true}
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
	mux.HandleFunc("/api/v1/agent/notify", s.handleNotify)

	log.Infof("Agent HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
