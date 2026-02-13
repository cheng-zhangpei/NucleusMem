package agent_monitor

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"github.com/pingcap-incubator/tinykv/log"
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

	reqInternal := &LaunchAgentInternalRequest{
		AgentID:            reqHTTP.AgentID,
		Role:               reqHTTP.Role,
		Image:              reqHTTP.Image,
		BinPath:            reqHTTP.BinPath,
		MountMemSpaceNames: reqHTTP.MountMemSpaceNames,
		Env:                reqHTTP.Env,
		HttpAddress:        reqHTTP.HttpAddr,
	}

	agentInfo, err := s.monitor.LaunchAgentInternal(reqInternal)
	resp := api.LaunchAgentResponseHTTP{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else {
		// 返回新 Agent 信息
		resp.AgentID = agentInfo.AgentID
		resp.HttpAddr = agentInfo.Addr
		resp.NodeID = s.monitor.id
	}

	json.NewEncoder(w).Encode(resp)
}

// GET /api/v1/status
func (s *AgentMonitorHTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeInfo := s.monitor.GetNodeSystemInfo()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeInfo)
}

// pkg/agent_monitor/http_server.go
// DELETE /api/v1/agents/{agent_id}

func (s *AgentMonitorHTTPServer) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AgentID uint64 `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := s.monitor.StopAgent(req.AgentID)
	resp := api.StopAgentResponseHTTP{
		Success: err == nil,
		AgentID: req.AgentID,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func (s *AgentMonitorHTTPServer) Start() error {
	mux := http.NewServeMux()
	addr := s.monitor.Config.MonitorUrl
	mux.HandleFunc("/api/v1/monitor/status", s.handleStatus)
	mux.HandleFunc("/api/v1/monitor/launch", s.handleLaunchAgent)
	mux.HandleFunc("/api/v1/monitor/destroy", s.handleStopAgent)

	log.Infof("AgentMonitor HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
