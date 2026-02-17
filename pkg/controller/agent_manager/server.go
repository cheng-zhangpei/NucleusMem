package agent_manager

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"net/http"
	"strconv"
)

type AgentManagerHTTPServer struct {
	manager *AgentManager
}

func NewAgentManagerHTTPServer(manager *AgentManager) *AgentManagerHTTPServer {
	return &AgentManagerHTTPServer{manager: manager}
}

// GET /api/v1/agent_manager/listAgent
func (s *AgentManagerHTTPServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.manager.agentCache.GetAllAgents()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// POST /api/v1/agent_manager/launch
func (s *AgentManagerHTTPServer) handleLaunchAgent(w http.ResponseWriter, r *http.Request) {
	var req api.LaunchAgentRequestHTTP
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 默认在 Node 1 启动（可扩展）
	agentInfo, err := s.manager.LaunchAgentOnNode(r.Context(), 1, &req)
	resp := api.LaunchAgentResponseHTTP{
		AgentID:  agentInfo.AgentID,
		HttpAddr: agentInfo.HTTPAddr,
		NodeID:   agentInfo.NodeID,
		Success:  err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// DELETE /api/v1/agent_manager/destroy?agent_id=123
func (s *AgentManagerHTTPServer) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从 query 参数获取 agent_id
	agentIDStr := r.URL.Query().Get("agent_id")
	if agentIDStr == "" {
		http.Error(w, "Missing agent_id parameter", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(agentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	err = s.manager.StopAgentOnNode(r.Context(), 1, agentID)
	resp := api.StopAgentResponseHTTP{
		Success: err == nil,
		AgentID: agentID,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitors/status
func (s *AgentManagerHTTPServer) handleMonitorStatusUpdate(w http.ResponseWriter, r *http.Request) {
	var update api.MonitorStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	for _, agentStatus := range update.Agents {
		agentID, err := strconv.ParseUint(agentStatus.AgentID, 10, 64)
		if err != nil {
			continue
		}

		s.manager.agentCache.UpdateAgent(&AgentInfo{
			AgentID:  agentID,
			Status:   agentStatus.Phase,
			NodeID:   update.NodeID,
			NodeAddr: s.manager.getMonitorAddr(update.NodeID),
			HTTPAddr: agentStatus.Addr,
		})
	}

	s.manager.syncCacheWithUpdate(update)
	w.WriteHeader(http.StatusOK)
}

// Start server
func (s *AgentManagerHTTPServer) Start(addr string) error {
	mux := http.NewServeMux()

	// Updated paths as requested
	mux.HandleFunc("/api/v1/agent_manager/listAgent", s.handleListAgents)
	mux.HandleFunc("/api/v1/agent_manager/launch", s.handleLaunchAgent)
	mux.HandleFunc("/api/v1/agent_manager/destroy", s.handleStopAgent)
	mux.HandleFunc("/api/v1/agent_manager/update", s.handleMonitorStatusUpdate)

	return http.ListenAndServe(addr, mux)
}
