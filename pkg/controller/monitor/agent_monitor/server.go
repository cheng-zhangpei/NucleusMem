package agent_monitor

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type AgentMonitorHTTPServer struct {
	monitor *AgentMonitor // 持有核心 service
}

func NewAgentMonitorHTTPServer(monitor *AgentMonitor) *AgentMonitorHTTPServer {
	return &AgentMonitorHTTPServer{monitor: monitor}
}

// POST /api/v1/agents
func (s *AgentMonitorHTTPServer) handleLaunchAgent(w http.ResponseWriter, r *http.Request) {
	var reqHTTP api.LaunchAgentRequestHTTP
	if err := json.NewDecoder(r.Body).Decode(&reqHTTP); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 转换为 internal request
	reqInternal := &LaunchAgentInternalRequest{
		AgentID:            reqHTTP.AgentID,
		Role:               reqHTTP.Role,
		Image:              reqHTTP.Image,
		BinPath:            reqHTTP.BinPath,
		MountMemSpaceNames: reqHTTP.MountMemSpaceNames,
		Env:                reqHTTP.Env,
	}

	err := s.monitor.LaunchAgentInternal(reqInternal)
	resp := api.LaunchAgentResponseHTTP{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(resp)
}

// DELETE /api/v1/agents/{agent_id}
func (s *AgentMonitorHTTPServer) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	// 从 URL 获取 agent_id（简单起见，用 query param 或 path var）
	// 这里用 query: ?agent_id=123
	agentIDStr := r.URL.Query().Get("agent_id")
	if agentIDStr == "" {
		http.Error(w, "Missing agent_id", http.StatusBadRequest)
		return
	}

	var agentID uint64

	fmt.Sscan(agentIDStr, &agentID)

	err := s.monitor.StopAgent(agentID)
	resp := api.StopAgentResponseHTTP{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(resp)
}

// GET /api/v1/status
func (s *AgentMonitorHTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	info := s.monitor.GetNodeStatusInternal()
	json.NewEncoder(w).Encode(info) // 假设 GetMonitorInfo 返回可 JSON 序列化的 struct
}

// Start 启动 HTTP 服务
func (s *AgentMonitorHTTPServer) Start() error {
	addr := s.monitor.Config.MonitorUrl
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/agents", s.handleLaunchAgent)
	mux.HandleFunc("DELETE /api/v1/agents", s.handleStopAgent)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	log.Printf("AgentMonitor HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
