// Package memspace provides the HTTP server for MemSpace operations
package memspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/storage"
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

	responseContent, err := s.memSpace.SendMessage(fromAgent, toAgent, req.Key, req.Content)
	resp := api.SendMessageResponse{Response: responseContent, Success: err == nil}
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

	// 验证必填字段
	if req.Addr == "" {
		http.Error(w, "addr is required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		http.Error(w, "role is required", http.StatusBadRequest)
		return
	}

	// 调用 MemSpace.BindAgent
	err = s.memSpace.BindAgent(agentID, req.Addr, req.Role)
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
func (s *MemSpaceHTTPServer) handleGetByKey(w http.ResponseWriter, r *http.Request) {
	var req api.GetByKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.RawKey == "" {
		http.Error(w, "raw_key is required", http.StatusBadRequest)
		return
	}

	rawKey := []byte(req.RawKey) // 直接当作 raw key

	value, err := s.memSpace.GetByKey(rawKey)
	resp := api.GetByKeyResponse{Success: err == nil}
	if err != nil {
		resp.Error = err.Error()
		w.WriteHeader(http.StatusNotFound)
	} else {
		// ✅ 直接转为 string（假设存储的是 JSON 或文本）
		resp.Value = string(value)
	}

	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/get
func (s *MemSpaceHTTPServer) handleGetTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "tool name is required", http.StatusBadRequest)
		return
	}

	tool, err := s.memSpace.ToolRegion.GetTool(req.Name)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Tool not found: %v", err),
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(resp)
		return
	}
	resp := struct {
		Success bool                    `json:"success"`
		Tool    *configs.ToolDefinition `json:"tool"`
	}{
		Success: true,
		Tool:    tool,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tools/list
func (s *MemSpaceHTTPServer) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools, err := s.memSpace.ToolRegion.ListTools()
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to list tools: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool                      `json:"success"`
		Tools   []*configs.ToolDefinition `json:"tools"`
	}{
		Success: true,
		Tools:   tools,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/register
func (s *MemSpaceHTTPServer) handleRegisterTool(w http.ResponseWriter, r *http.Request) {
	var req configs.ToolDefinition
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := s.memSpace.ToolRegion.RegisterTool(&req)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to register tool: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/delete
func (s *MemSpaceHTTPServer) handleDeleteTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := s.memSpace.ToolRegion.DeleteTool(req.Name)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to delete tool: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/find_by_tags
func (s *MemSpaceHTTPServer) handleFindToolsByTags(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	tools, err := s.memSpace.ToolRegion.FindToolsByTags(req.Tags)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to find tools: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool                      `json:"success"`
		Tools   []*configs.ToolDefinition `json:"tools"`
	}{
		Success: true,
		Tools:   tools,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *MemSpaceHTTPServer) handleLoadToolDAG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Empty request body - DAG is identified by MemSpaceID in the server context
	var req struct{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	dag, err := s.memSpace.ToolRegion.LoadToolDAG()
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error,omitempty"`
		}{
			Success: false,
			Error:   fmt.Sprintf("failed to load tool DAG: %v", err),
		}
		// Return 404 if DAG not found, 500 for other errors
		if err.Error() == "key not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool             `json:"success"`
		DAG     *configs.ToolDAG `json:"dag,omitempty"`
	}{
		Success: true,
		DAG:     dag,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/exec/record
// handleRecordToolExec records a single tool execution result for auditing
// This endpoint is called after each tool completes (for per-tool audit mode)
func (s *MemSpaceHTTPServer) handleRecordToolExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ToolName string                 `json:"tool_name"`
		Output   map[string]interface{} `json:"output,omitempty"`
		Error    string                 `json:"error,omitempty"`
		Seq      uint64                 `json:"seq,omitempty"` // Optional: execution sequence number
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ToolName == "" {
		http.Error(w, "tool_name is required", http.StatusBadRequest)
		return
	}

	var err error
	if req.Seq > 0 {
		// Update existing record by sequence number
		err = s.memSpace.ToolRegion.CompleteToolExec(req.Seq, req.Output, req.Error)
	} else {
		// For batch mode or fallback: record by tool name (less precise)
		// Note: This is a simplified path; prefer using seq for accurate tracking
		execKey := fmt.Sprintf("tool/exec/batch/%s", req.ToolName)
		parseUint, _ := strconv.ParseUint(s.memSpace.ID, 10, 64)
		rawKey := configs.EncodeKey(configs.ZoneTool, parseUint, []byte(execKey))

		record := configs.ToolExecRecord{
			ToolName: req.ToolName,
			Output:   req.Output,
			Status:   configs.ToolExecCompleted,
			Error:    req.Error,
			DoneAt:   time.Now().Unix(),
		}
		if req.Error != "" {
			record.Status = configs.ToolExecFailed
		}

		data, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			err = fmt.Errorf("failed to marshal record: %w", marshalErr)
		} else {
			err = s.memSpace.kvClient.Update(func(txn storage.Transaction) error {
				return txn.Put(rawKey, data)
			})
		}
	}

	resp := struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}{
		Success: err == nil,
		Error:   errMsg(err),
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/tool/exec/batch
// handleRecordToolExecBatch records multiple tool execution results atomically
// This endpoint is called after an entire ToolDAG completes (for batch audit mode)
func (s *MemSpaceHTTPServer) handleRecordToolExecBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Results map[string]*configs.ToolExecResult `json:"results"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Results) == 0 {
		http.Error(w, "results cannot be empty", http.StatusBadRequest)
		return
	}

	// Convert configs.ToolExecResult to memspace_region.ToolExecRecord
	results := make(map[string]*configs.ToolExecResult)
	for name, res := range req.Results {
		results[name] = &configs.ToolExecResult{
			ToolName: res.ToolName,
			Output:   res.Output,
			Error:    res.Error,
			Status:   res.Status,
			DoneAt:   res.DoneAt,
		}
	}

	err := s.memSpace.ToolRegion.RecordToolExecBatch(results)
	resp := struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}{
		Success: err == nil,
		Error:   errMsg(err),
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}
func errMsg(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// POST /api/v1/memspace/tool/dag/save
// handleSaveToolDAG persists a tool dependency graph for an Atomic ViewSpace
// This endpoint is called during the Grow phase to inject the execution plan
func (s *MemSpaceHTTPServer) handleSaveToolDAG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DAG *configs.ToolDAG `json:"dag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DAG == nil {
		http.Error(w, "dag is required", http.StatusBadRequest)
		return
	}

	// Optional: validate DAG structure before saving
	if err := validateToolDAG(req.DAG); err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("invalid DAG: %v", err),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	err := s.memSpace.SaveToolDAG(req.DAG)
	resp := struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}{
		Success: err == nil,
		Error:   errMsg(err),
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// validateToolDAG performs basic structural validation on a ToolDAG
func validateToolDAG(dag *configs.ToolDAG) error {
	if dag == nil {
		return fmt.Errorf("dag cannot be nil")
	}
	if len(dag.Nodes) == 0 {
		return fmt.Errorf("dag must have at least one node")
	}

	// Check for duplicate node names
	names := make(map[string]bool)
	for _, node := range dag.Nodes {
		if names[node.ToolName] {
			return fmt.Errorf("duplicate tool name: %s", node.ToolName)
		}
		names[node.ToolName] = true
	}

	// Check that all edges reference existing nodes
	for _, edge := range dag.Edges {
		if !names[edge.From] {
			return fmt.Errorf("edge 'from' references unknown node: %s", edge.From)
		}
		if !names[edge.To] {
			return fmt.Errorf("edge 'to' references unknown node: %s", edge.To)
		}
	}

	return nil
}

func (s *MemSpaceHTTPServer) handleGetToolExecHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ToolName string `json:"tool_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ToolName == "" {
		http.Error(w, "tool_name is required", http.StatusBadRequest)
		return
	}

	// Get internal records from ToolRegion
	internalRecords, err := s.memSpace.ToolRegion.GetToolExecHistory(req.ToolName)

	// Convert to configs.ToolExecResult for HTTP API
	var results []*configs.ToolExecResult
	for _, rec := range internalRecords {
		results = append(results, &configs.ToolExecResult{
			ToolName: rec.ToolName,
			Output:   rec.Output,
			Error:    rec.Error,
			Status:   string(rec.Status), // ToolExecStatus -> string
			Seq:      rec.Seq,
			DoneAt:   rec.DoneAt,
		})
	}

	// Always return success=true with potentially empty list
	resp := struct {
		Success bool                      `json:"success"`
		Records []*configs.ToolExecResult `json:"records"`
		Error   string                    `json:"error,omitempty"`
	}{
		Success: true,
		Records: results,
	}

	if err != nil {
		resp.Error = fmt.Sprintf("query failed: %v", err)
		log.Warnf("GetToolExecHistory warning: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ============================================================
// Standard Tool Handlers (New Interface)
// ============================================================

// POST /api/v1/memspace/standard_tool/register
func (s *MemSpaceHTTPServer) handleRegisterStandardTool(w http.ResponseWriter, r *http.Request) {
	var req configs.StandardToolDefinition
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := s.memSpace.ToolRegion.RegisterStandardTool(&req)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to register standard tool: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/standard_tool/get
func (s *MemSpaceHTTPServer) handleGetStandardTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "tool name is required", http.StatusBadRequest)
		return
	}

	tool, err := s.memSpace.ToolRegion.GetStandardTool(req.Name)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Standard tool not found: %v", err),
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool                            `json:"success"`
		Tool    *configs.StandardToolDefinition `json:"tool"`
	}{
		Success: true,
		Tool:    tool,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/standard_tool/list
func (s *MemSpaceHTTPServer) handleListStandardTools(w http.ResponseWriter, r *http.Request) {
	tools, err := s.memSpace.ToolRegion.ListStandardTools()
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to list standard tools: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool                              `json:"success"`
		Tools   []*configs.StandardToolDefinition `json:"tools"`
	}{
		Success: true,
		Tools:   tools,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/standard_tool/delete
func (s *MemSpaceHTTPServer) handleDeleteStandardTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := s.memSpace.ToolRegion.DeleteStandardTool(req.Name)
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to delete standard tool: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/metadata
func (s *MemSpaceHTTPServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	meta, err := s.memSpace.GetMetadata()
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to get metadata: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success  bool              `json:"success"`
		Metadata *MemSpaceMetadata `json:"metadata"`
	}{
		Success:  true,
		Metadata: meta,
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/memspace/memory/contents
func (s *MemSpaceHTTPServer) handleAllMemoryContents(w http.ResponseWriter, r *http.Request) {
	contents, err := s.memSpace.GetAllMemoryContents()
	if err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}{
			Success: false,
			Error:   fmt.Sprintf("Failed to get memory contents: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := struct {
		Success  bool     `json:"success"`
		Contents []string `json:"contents"`
	}{
		Success:  true,
		Contents: contents,
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
	mux.HandleFunc("/api/v1/memspace/get_by_key", s.handleGetByKey)
	// tool operation
	mux.HandleFunc("/api/v1/memspace/tool/get", s.handleGetTool)
	mux.HandleFunc("/api/v1/memspace/tools/list", s.handleListTools)
	mux.HandleFunc("/api/v1/memspace/tool/register", s.handleRegisterTool)
	mux.HandleFunc("/api/v1/memspace/tool/delete", s.handleDeleteTool)
	mux.HandleFunc("/api/v1/memspace/tool/find_by_tags", s.handleFindToolsByTags)
	// task operation
	// mux.HandleFunc("/api/v1/memspace/task/dag/load", s.handleLoadTaskDAG)
	// mux.HandleFunc("/api/v1/memspace/task/complete", s.handleCompleteTask)
	// dependency operation
	mux.HandleFunc("/api/v1/memspace/tool/dag/load", s.handleLoadToolDAG)
	mux.HandleFunc("/api/v1/memspace/tool/exec/record", s.handleRecordToolExec)
	mux.HandleFunc("/api/v1/memspace/tool/exec/batch", s.handleRecordToolExecBatch)
	mux.HandleFunc("/api/v1/memspace/tool/dag/save", s.handleSaveToolDAG)
	mux.HandleFunc("/api/v1/memspace/tool/exec/history", s.handleGetToolExecHistory)

	mux.HandleFunc("/api/v1/memspace/standard_tool/register", s.handleRegisterStandardTool)
	mux.HandleFunc("/api/v1/memspace/standard_tool/get", s.handleGetStandardTool)
	mux.HandleFunc("/api/v1/memspace/standard_tool/list", s.handleListStandardTools)
	mux.HandleFunc("/api/v1/memspace/standard_tool/delete", s.handleDeleteStandardTool)
	mux.HandleFunc("/api/v1/memspace/metadata", s.handleMetadata)
	mux.HandleFunc("/api/v1/memspace/memory/contents", s.handleAllMemoryContents)
	log.Infof("MemSpace HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
