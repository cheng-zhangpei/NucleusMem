// Package memspace provides the HTTP server for MemSpace operations
package memspace

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

// MemSpaceHTTPServer handles HTTP requests for MemSpace
type MemSpaceHTTPServer struct {
	memSpace *MemSpace
}

// NewMemSpaceHTTPServer creates a new HTTP server for the given MemSpace
func NewMemSpaceHTTPServer(memSpace *MemSpace) *MemSpaceHTTPServer {
	return &MemSpaceHTTPServer{memSpace: memSpace}
}

// POST /api/v1/memspace/write_memory
func (s *MemSpaceHTTPServer) handleWriteMemory(w http.ResponseWriter, r *http.Request) {
	var req api.WriteMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.memSpace.WriteMemory(req.Content, agentID)
	resp := api.WriteMemoryResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/get_memory_context
func (s *MemSpaceHTTPServer) handleGetMemoryContext(w http.ResponseWriter, r *http.Request) {
	var req api.GetMemoryContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	summary, memories, err := s.memSpace.GetMemoryContext(req.SummaryBefore, req.Query, req.N)
	resp := api.GetMemoryContextResponse{
		Success:  err == nil,
		Summary:  summary,
		Memories: memories,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/register_agent
func (s *MemSpaceHTTPServer) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.memSpace.RegisterAgent(agentID, req.Addr, req.Role)
	resp := api.RegisterAgentResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/unregister_agent
func (s *MemSpaceHTTPServer) handleUnregisterAgent(w http.ResponseWriter, r *http.Request) {
	var req api.UnregisterAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.memSpace.UnRegisterAgent(agentID)
	resp := api.UnregisterAgentResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/send_message
func (s *MemSpaceHTTPServer) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var req api.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fromAgent, err := strconv.ParseUint(req.FromAgent, 10, 64)
	if err != nil {
		http.Error(w, "Invalid from_agent", http.StatusBadRequest)
		return
	}

	toAgent, err := strconv.ParseUint(req.ToAgent, 10, 64)
	if err != nil {
		http.Error(w, "Invalid to_agent", http.StatusBadRequest)
		return
	}

	err = s.memSpace.SendMessage(fromAgent, toAgent, req.Key, req.RefType)
	resp := api.SendMessageResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/list_agents
func (s *MemSpaceHTTPServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Optional: Parse empty body or ignore payload
	// We'll just ignore the body for now

	agents := s.memSpace.ListAgents()
	resp := api.ListAgentsResponse{
		Success: true,
		Agents:  make([]api.AgentRegistryEntry, len(agents)),
	}

	for i, a := range agents {
		resp.Agents[i] = api.AgentRegistryEntry{
			AgentID:   fmt.Sprintf("%d", a.AgentID),
			Addr:      a.Addr,
			Role:      a.Role,
			Timestamp: a.Timestamp,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func (s *MemSpaceHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := api.MemSpaceHealthResponse{
		Status:      "healthy",
		MemSpaceID:  s.memSpace.ID,
		Name:        s.memSpace.ID, // 或从配置读取
		Type:        string(s.memSpace.Type),
		OwnerID:     s.memSpace.OwnerID,
		Description: s.memSpace.Description,
		Timestamp:   time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func (s *MemSpaceHTTPServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Trigger graceful shutdown
	go func() {
		// Give time for response to be sent
		time.Sleep(100 * time.Millisecond)
		s.memSpace.Stop()
		os.Exit(0)
	}()
	resp := map[string]bool{"success": true}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/bind_agent
func (s *MemSpaceHTTPServer) handleBindAgent(w http.ResponseWriter, r *http.Request) {
	var req api.BindAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.memSpace.BindAgent(agentID)
	resp := api.BindAgentResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/unbind_agent
func (s *MemSpaceHTTPServer) handleUnbindAgent(w http.ResponseWriter, r *http.Request) {
	var req api.UnbindAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.memSpace.UnBindAgent(agentID)
	resp := api.UnbindAgentResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// Start initializes and starts the HTTP server
func (s *MemSpaceHTTPServer) Start() error {
	mux := http.NewServeMux()
	addr := s.memSpace.httpAddr
	// Core methods
	mux.HandleFunc("/api/v1/memspace/write_memory", s.handleWriteMemory)
	mux.HandleFunc("/api/v1/memspace/get_memory_context", s.handleGetMemoryContext)
	mux.HandleFunc("/api/v1/memspace/register_agent", s.handleRegisterAgent)
	mux.HandleFunc("/api/v1/memspace/unregister_agent", s.handleUnregisterAgent)
	mux.HandleFunc("/api/v1/memspace/send_message", s.handleSendMessage)
	mux.HandleFunc("/api/v1/memspace/list_agents", s.handleListAgents)
	mux.HandleFunc("/api/v1/memspace/shutdown", s.handleShutdown)
	mux.HandleFunc("/api/v1/memspace/health", s.handleHealth)
	mux.HandleFunc("/api/v1/memspace/bind_agent", s.handleBindAgent)
	mux.HandleFunc("/api/v1/memspace/unbind_agent", s.handleUnbindAgent)
	log.Infof("MemSpace HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
