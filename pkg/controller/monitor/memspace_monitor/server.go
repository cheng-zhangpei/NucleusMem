// Package memspace provides the HTTP server for MemSpaceMonitor
package memspace_monitor

import (
	"NucleusMem/pkg/api"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"strconv"
)

// MemSpaceMonitorHTTPServer handles HTTP requests for MemSpaceMonitor
type MemSpaceMonitorHTTPServer struct {
	monitor *MemSpaceMonitor
}

// NewMemSpaceMonitorHTTPServer creates a new HTTP server
func NewMemSpaceMonitorHTTPServer(monitor *MemSpaceMonitor) *MemSpaceMonitorHTTPServer {
	return &MemSpaceMonitorHTTPServer{monitor: monitor}
}

// POST /api/v1/monitor/launch_memspace
func (s *MemSpaceMonitorHTTPServer) handleLaunchMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.LaunchMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	info, err := s.monitor.LaunchMemSpace(&req)
	resp := api.LaunchMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		resp.MemSpaceID = info.MemSpaceID
		resp.Addr = info.Addr
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitor/stop_memspace
func (s *MemSpaceMonitorHTTPServer) handleStopMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.StopMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	err = s.monitor.StopMemSpace(memspaceID)
	resp := api.StopMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitor/register_memspace
func (s *MemSpaceMonitorHTTPServer) handleRegisterMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	ownerID, err := strconv.ParseUint(req.OwnerID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid owner_id", http.StatusBadRequest)
		return
	}

	err = s.monitor.RegisterMemSpace(memspaceID, req.Name, ownerID, req.Type, req.Addr)
	resp := api.RegisterMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitor/connect_memspace
func (s *MemSpaceMonitorHTTPServer) handleConnectMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.ConnectMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	err = s.monitor.ConnectToMemSpace(memspaceID, req.Addr)
	resp := api.ConnectMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitor/unregister_memspace
func (s *MemSpaceMonitorHTTPServer) handleUnregisterMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.UnregisterMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	err = s.monitor.UnregisterMemSpace(memspaceID)
	resp := api.UnregisterMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/monitor/list_memspaces
func (s *MemSpaceMonitorHTTPServer) handleListMemSpaces(w http.ResponseWriter, r *http.Request) {
	infos := s.monitor.ListMemSpaces()
	resp := api.ListMemSpacesResponse{
		Success:   true,
		MemSpaces: make([]api.MemSpaceInfo, len(infos)),
	}

	for i, info := range infos {
		resp.MemSpaces[i] = api.MemSpaceInfo{
			MemSpaceID:  fmt.Sprintf("%d", info.MemSpaceID),
			Name:        info.Name,
			OwnerID:     fmt.Sprintf("%d", info.OwnerID),
			Type:        info.Type,
			Status:      info.Status,
			HttpAddr:    info.Addr,
			Description: info.Description,
			LastSeen:    info.LastSeen,
		}
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /health
func (s *MemSpaceMonitorHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "healthy"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Start initializes and starts the HTTP server
func (s *MemSpaceMonitorHTTPServer) Start(addr string) error {
	mux := http.NewServeMux()

	// MemSpace management
	mux.HandleFunc("/api/v1/monitor/launch_memspace", s.handleLaunchMemSpace)
	mux.HandleFunc("/api/v1/monitor/stop_memspace", s.handleStopMemSpace)
	mux.HandleFunc("/api/v1/monitor/register_memspace", s.handleRegisterMemSpace)
	mux.HandleFunc("/api/v1/monitor/connect_memspace", s.handleConnectMemSpace)
	mux.HandleFunc("/api/v1/monitor/unregister_memspace", s.handleUnregisterMemSpace)
	mux.HandleFunc("/api/v1/monitor/list_memspaces", s.handleListMemSpaces)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	log.Infof("MemSpaceMonitor HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
