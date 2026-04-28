// Package agent provides the HTTP server for managing AI agents
package agent

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/configs"
	"encoding/json"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	task := &AgentTask{
		Type:    TaskTypeTempChat,
		Content: req.Message,
	}
	response, err := s.agent.SubmitTask(task)
	if err != nil {
		http.Error(w, "Failed to submit task", http.StatusInternalServerError)
		return
	}

	// Immediate ack (async processing)
	resp := api.TempChatResponse{Response: response, Success: true}
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
	task := &AgentTask{
		Type:    TaskTypeChat,
		Content: req.Message,
	}
	ID, err := s.agent.SubmitTask(task)
	result, err := s.agent.GetTaskResult(ID, 5*time.Minute)
	if err != nil {
		log.Errorf("Failed to get task result for task %d: %v", ID, err)
		http.Error(w, "Failed to get task result", http.StatusInternalServerError)
	}

	resp := api.ChatResponse{Response: result, Success: true}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/agent/notify
// POST /api/v1/agent/notify
func (s *AgentHTTPServer) handleNotify(w http.ResponseWriter, r *http.Request) {
	var req api.NotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// 创建 Comm 任务
	task := &AgentTask{
		Type:      TaskTypeComm,
		Key:       req.Key,
		Content:   req.Content,
		Timestamp: time.Now().Unix(),
	}

	// 提交任务并获取 taskID
	taskID, err := s.agent.SubmitTask(task)
	if err != nil {
		resp := api.NotifyResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// 等待任务完成（带超时）
	result, err := s.agent.GetTaskResult(taskID, 30*time.Second)

	resp := api.NotifyResponse{
		Success: err == nil,
		Result:  result,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)

	log.Infof("Agent %d notify completed for task %s: %v", s.agent.AgentId, taskID, err)
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

// POST /api/v1/agent/bind_memspace
func (s *AgentHTTPServer) handleBindMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.BindMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	config := &configs.MemSpaceConfig{
		MemSpaceID: memspaceID,
		Type:       strings.ToLower(req.Type),
		HttpAddr:   req.HttpAddr,
	}
	err = s.agent.bindingMemSpace(config)
	resp := map[string]interface{}{
		"success": err == nil,
	}
	if err != nil {
		resp["error"] = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/agent/unbind_memspace
func (s *AgentHTTPServer) handleUnbindMemSpace(w http.ResponseWriter, r *http.Request) {
	var req api.UnbindMemSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	memspaceID, err := strconv.ParseUint(req.MemSpaceID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid memspace_id", http.StatusBadRequest)
		return
	}

	err = s.agent.unBindingMemSpace(memspaceID)
	resp := map[string]interface{}{
		"success": err == nil,
	}
	if err != nil {
		resp["error"] = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}
func (s *AgentHTTPServer) handleCommunicate(w http.ResponseWriter, r *http.Request) {
	var req api.CommunicateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.TargetAgentID == "" {
		http.Error(w, "target_agent_id is required", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}
	targetAgentID, err := strconv.ParseUint(req.TargetAgentID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_agent_id", http.StatusBadRequest)
		return
	}
	result, err := s.agent.Communicate(targetAgentID, req.Key, req.Content)
	log.Debugf("[agent] Agent %d communicate result: %v", s.agent.AgentId, err)
	resp := api.CommunicateResponse{Result: result, Success: err == nil}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// POST /api/v1/agent/submit_task
func (s *AgentHTTPServer) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	var req api.SubmitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.SubmitTaskResponse{
			Success: false, ErrorMessage: "invalid request body",
		})
		return
	}

	task := &AgentTask{
		Type:             req.Type,
		Content:          req.Content,
		AvailableTools:   req.AvailableTools,
		AvailableMemTags: req.AvailableMemTags,
		MaxRetry:         req.MaxRetry,
		ToolName:         req.ToolName,
		Params:           req.Params,
	}

	taskID, err := s.agent.SubmitTask(task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.SubmitTaskResponse{
			Success: false, ErrorMessage: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, api.SubmitTaskResponse{
		Success: true, TaskID: taskID,
	})
}

// POST /api/v1/agent/task_result
func (s *AgentHTTPServer) handleGetTaskResult(w http.ResponseWriter, r *http.Request) {
	var req api.GetTaskResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.GetTaskResultResponse{
			Success: false, ErrorMessage: "invalid request body",
		})
		return
	}
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	result, err := s.agent.GetTaskResult(req.TaskID, timeout)
	if err != nil {
		writeJSON(w, http.StatusOK, api.GetTaskResultResponse{
			Success: false, TaskID: req.TaskID, ErrorMessage: err.Error(), Done: false,
		})
		return
	}
	writeJSON(w, http.StatusOK, api.GetTaskResultResponse{
		Success: true, TaskID: req.TaskID, Result: result, Done: true,
	})
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
	mux.HandleFunc("/api/v1/agent/bind_memspace", s.handleBindMemSpace)
	mux.HandleFunc("/api/v1/agent/unbind_memspace", s.handleUnbindMemSpace)
	mux.HandleFunc("/api/v1/agent/communicate", s.handleCommunicate)
	mux.HandleFunc("/api/v1/agent/submit_task", s.handleSubmitTask)
	mux.HandleFunc("/api/v1/agent/task_result", s.handleGetTaskResult)
	log.Infof("Agent HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
