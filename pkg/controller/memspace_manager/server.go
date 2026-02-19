// Package memspace_manager provides the HTTP server for MemSpaceManager
package memspace_manager

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/configs"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"strconv"
)

// MemSpaceManagerHTTPServer handles HTTP requests for MemSpaceManager
type MemSpaceManagerHTTPServer struct {
	manager *MemSpaceManager
}

// NewMemSpaceManagerHTTPServer creates a new HTTP server
func NewMemSpaceManagerHTTPServer(manager *MemSpaceManager) *MemSpaceManagerHTTPServer {
	return &MemSpaceManagerHTTPServer{manager: manager}
}

// POST /api/v1/manager/list_memspaces
func (s *MemSpaceManagerHTTPServer) handleListMemSpaces(w http.ResponseWriter, r *http.Request) {
	memspaces := s.manager.ListMemSpaceInfo()
	// Convert to API format
	apiMemspaces := make([]api.MemSpaceInfo, len(memspaces))
	for i, ms := range memspaces {
		apiMemspaces[i] = api.MemSpaceInfo{
			MemSpaceID:  fmt.Sprintf("%d", ms.MemSpaceID),
			Name:        ms.Name,
			OwnerID:     fmt.Sprintf("%d", ms.OwnerAgentID),
			Type:        ms.Type,
			Status:      ms.Status,
			HttpAddr:    ms.HttpAddr,
			Description: "", // Add if needed
			LastSeen:    ms.CreatedAt,
		}
	}
	resp := api.ListMemSpacesResponse{
		Success:   true,
		MemSpaces: apiMemspaces,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/manager/bind_memspace
func (s *MemSpaceManagerHTTPServer) handleBindMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.BindMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}
	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}
	err = s.manager.BindMemSpaceToAgent(agentID, memspaceID)
	resp := api.BindMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/manager/unbind_memspace
func (s *MemSpaceManagerHTTPServer) handleUnbindMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.UnbindMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseUint(req.AgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent_id", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	err = s.manager.UnBindMemSpaceWithAgent(agentID, memspaceID)
	resp := api.UnbindMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/manager/launch_memspace
func (s *MemSpaceManagerHTTPServer) handleLaunchMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.LaunchMemSpaceRequestManager
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Convert to config
	config := &configs.MemSpaceConfig{
		MemSpaceID:          req.MemSpaceID,
		Name:                req.Name,
		Type:                req.Type,
		OwnerID:             req.OwnerID,
		Description:         req.Description,
		HttpAddr:            req.HttpAddr,
		PdAddr:              req.PdAddr,
		EmbeddingClientAddr: req.EmbeddingClientAddr,
		LightModelAddr:      req.LightModelAddr,
		SummaryCnt:          req.SummaryCnt,
		SummaryThreshold:    req.SummaryThreshold,
		BinPath:             req.BinPath,
		ConfigFilePath:      req.ConfigFilePath,
	}
	err := s.manager.LaunchMemspace(config)
	resp := api.LaunchMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/manager/shutdown_memspace
func (s *MemSpaceManagerHTTPServer) handleShutdownMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.ShutdownMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	config := &configs.MemSpaceConfig{
		MemSpaceID: req.MemSpaceID,
	}

	err := s.manager.ShutDownMemspace(config)
	resp := api.ShutdownMemSpaceResponse{Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/manager/notify_memspaces (from monitors)
func (s *MemSpaceManagerHTTPServer) handleNotifyMemSpaces(w http.ResponseWriter, r *http.Request) {
	var req api.NotifyMemSpacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update cache
	for _, ms := range req.MemSpaces {
		memspaceID, _ := strconv.ParseUint(ms.MemSpaceID, 10, 64)
		ownerID, _ := strconv.ParseUint(ms.OwnerID, 10, 64)

		cacheInfo := &MemSpaceInfo{
			MemSpaceID:   memspaceID,
			Name:         ms.Name,
			OwnerAgentID: ownerID,
			Type:         ms.Type,
			NodeID:       0, // Could be extracted from request context
			HttpAddr:     ms.HttpAddr,
			Status:       ms.Status,
			CreatedAt:    ms.LastSeen,
			Metadata:     nil,
		}
		s.manager.memSpaceCache.UpdateMemSpace(cacheInfo)
	}

	resp := api.NotifyMemSpacesResponse{Success: true}
	json.NewEncoder(w).Encode(resp)
}

// Start initializes and starts the HTTP server
func (s *MemSpaceManagerHTTPServer) Start(addr string) error {
	mux := http.NewServeMux()

	// Agent-facing APIs
	mux.HandleFunc("/api/v1/manager/list_memspaces", s.handleListMemSpaces)
	mux.HandleFunc("/api/v1/manager/bind_memspace", s.handleBindMemSpace)
	mux.HandleFunc("/api/v1/manager/unbind_memspace", s.handleUnbindMemSpace)
	mux.HandleFunc("/api/v1/manager/launch_memspace", s.handleLaunchMemSpace)
	mux.HandleFunc("/api/v1/manager/shutdown_memspace", s.handleShutdownMemSpace)
	mux.HandleFunc("/api/v1/manager/notify_memspaces", s.handleNotifyMemSpaces)
	log.Infof("MemSpaceManager HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
