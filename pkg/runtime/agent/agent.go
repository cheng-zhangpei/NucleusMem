package agent

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/configs/prompt"
	tool_executors "NucleusMem/pkg/runtime/agent/executors"
	"NucleusMem/pkg/viewspace"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Agent represents an AI agent that connects to memory spaces and services
type Agent struct {
	AgentId                uint64
	memSpaceClients        map[uint64]*client.MemSpaceClient // memSpaceID -> HTTP client
	memSpaceManagerClient  *client.MemSpaceManagerClient
	chatClient             *client.ChatServerClient
	embeddingClient        *client.EmbeddingServerClient
	httpAddr               string
	role                   string
	mu                     sync.RWMutex
	isJob                  bool
	privateMemSpaceClients *client.MemSpaceClient
	publicMemSpaceClients  []*client.MemSpaceClient
	tempMemory             []client.ChatMessage // 内存中的对话历史
	maxHistory             int                  // 最大历史轮数（可配置）
	boundMonitorID         uint64
	boundMu                sync.RWMutex

	taskQueue         chan *AgentTask
	taskResults       map[string]*TaskResult // taskID -> TaskResult
	taskResultsMu     sync.RWMutex
	toolDispatcher    *tool_executors.Dispatcher
	standardExecutors *StandardExecutorRegistry
	attackConfig      *configs.AttackConfig
	attackLibrary     *configs.AttackLibrary
}

// NewAgent creates a new Agent and initializes all service clients
func NewAgent(config *configs.AgentConfig) (*Agent, error) {
	// tool executor dispatcher
	dispatcher := tool_executors.NewDispatcher()
	dispatcher.Register("http", tool_executors.NewHTTPExecutor())
	attackCfg := config.AttackConfig
	if attackCfg == nil {
		attackCfg = configs.DefaultAttackConfig()
	}
	// 加载攻击库
	libPath := attackCfg.AttackLibraryPath
	if libPath == "" {
		libPath = configs.GetDefaultAttackLibraryPath()
	}
	attackLib, err := configs.LoadAttackLibrary(libPath)
	if err != nil {
		fmt.Printf("Warning: failed to load attack library: %v\n", err)
	} else if attackLib != nil {
		fmt.Printf("Loaded attack library: %s v%s (%d methods)\n",
			attackLib.Name, attackLib.Version, len(libPath))
	}

	agent := &Agent{
		AgentId:               config.AgentId,
		memSpaceClients:       make(map[uint64]*client.MemSpaceClient),
		memSpaceManagerClient: client.NewMemSpaceManagerClient(config.MemSpaceManagerAddr),
		chatClient:            client.NewChatServerClient(config.ChatServerAddr),
		embeddingClient:       client.NewEmbeddingServerClient(config.VectorServerAddr),
		isJob:                 config.IsJob,
		publicMemSpaceClients: make([]*client.MemSpaceClient, 0),
		taskResults:           make(map[string]*TaskResult),
		httpAddr:              config.HttpAddr,
		role:                  config.Role,
		toolDispatcher:        dispatcher,
		standardExecutors:     NewStandardExecutorRegistry(),
		attackConfig:          attackCfg,
		attackLibrary:         attackLib,
	}
	//agent.bindingMemspace()
	ctx, _ := context.WithCancel(context.Background())
	agent.taskQueue = make(chan *AgentTask, 1000)
	agent.maxHistory = 10
	// Connect to private MemSpace (required)
	if config.PrivateMemSpaceInfo != nil {
		memClient := client.NewMemSpaceClient(config.PrivateMemSpaceInfo.MemSpaceAddr)
		// 健康检查
		_, err := memClient.HealthCheckWithInfo()
		if err != nil {
			log.Errorf("Private MemSpace health check failed: %v", err)
		} else {
			// 绑定
			if err := memClient.BindAgent(config.AgentId, config.HttpAddr, config.Role); err != nil {
				log.Errorf("Failed to bind agent to private MemSpace: %v", err)
			} else {
				log.Infof("Agent %d bound to private MemSpace %d at %s",
					config.AgentId, config.PrivateMemSpaceInfo.MemSpaceId, config.PrivateMemSpaceInfo.MemSpaceAddr)
			}
		}
		agent.privateMemSpaceClients = memClient
	}

	// start the task loop
	go func() {
		if err := agent.Start(ctx); err != nil {
			log.Errorf("Agent %d task loop exited: %v", agent.AgentId, err)
		}
	}()
	return agent, nil
}

// connectToMemSpace creates an HTTP client for a MemSpace and stores it
func (a *Agent) connectToMemSpace(info *configs.MemSpaceInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Avoid duplicate connections
	if _, exists := a.memSpaceClients[info.MemSpaceId]; exists {
		return nil
	}
	// Create HTTP client (no connection needed — HTTP is stateless)
	client := client.NewMemSpaceClient(info.MemSpaceAddr)
	a.memSpaceClients[info.MemSpaceId] = client
	return nil
}

// GetMemSpaceClient returns the client for a given MemSpace ID
func (a *Agent) GetMemSpaceClient(memSpaceID uint64) (*client.MemSpaceClient, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	client, ok := a.memSpaceClients[memSpaceID]
	return client, ok
}

// TempChat handles user input and returns LLM response
func (a *Agent) TempChat(input string) (string, error) {
	if a.isJob {
		resp, err := a.chatClient.QuickChat(input)
		if err != nil {
			return "", err
		}
		return resp.Response, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Initialize system message if needed
	if len(a.tempMemory) == 0 {
		a.tempMemory = append(a.tempMemory, client.ChatMessage{
			Role:    "system",
			Content: "You are a helpful AI assistant.",
		})
	}

	// Add user message
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "user",
		Content: input,
	})

	// Truncate helper
	truncateHistory := func() {
		if a.maxHistory <= 0 {
			return
		}
		minLen := 1 // keep system
		if len(a.tempMemory) <= minLen {
			return
		}
		keepCount := a.maxHistory
		if keepCount < minLen {
			keepCount = minLen
		}
		if len(a.tempMemory) > keepCount {
			newHist := make([]client.ChatMessage, keepCount)
			newHist[0] = a.tempMemory[0]
			copy(newHist[1:], a.tempMemory[len(a.tempMemory)-(keepCount-1):])
			a.tempMemory = newHist
		}
	}
	truncateHistory()

	// Unlock for LLM call
	a.mu.Unlock()
	req := client.ChatCompletionRequest{
		Messages:    a.tempMemory,
		Temperature: 0.7,
		MaxTokens:   512,
	}
	chatResp, err := a.chatClient.ChatCompletion(req)
	a.mu.Lock()
	if err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	response := chatResp.Choices[0].Message.Content

	// Add assistant response
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "assistant",
		Content: response,
	})
	truncateHistory()

	return response, nil
}

// Chat is the main chat interface
func (a *Agent) Chat(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("input cannot be empty")
	}

	// Step 1: Fetch context from all public MemSpaces
	var allSummaries []string
	var allMemories []string

	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()

	for _, client := range publicClients {
		if client == nil {
			continue
		}
		summary, memories, err := client.GetMemoryContext(time.Now().Unix(), input, 5)
		if err != nil {
			log.Warnf("Failed to get context from public memspace: %v", err)
			continue
		}
		if summary != "" {
			allSummaries = append(allSummaries, summary)
		}
		allMemories = append(allMemories, memories...)
	}
	combinedSummary := ""
	if len(allSummaries) > 0 {
		combinedSummary = strings.Join(allSummaries, "\n---\n")
	}

	// Step 2: Get current temp history
	a.mu.RLock()
	tempHistory := make([]client.ChatMessage, len(a.tempMemory))
	copy(tempHistory, a.tempMemory)
	a.mu.RUnlock()
	var availableTools []*configs.ToolDefinition
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		tools, err := msClient.ListTools()
		if err == nil && len(tools) > 0 {
			availableTools = append(availableTools, tools...)
		}
	}

	// Step 3: Build prompt
	sysMsg := "You are an intelligent agent with access to shared memory and conversation history. Use both to answer the user's query."
	promptObj := prompt.NewChatPrompt(sysMsg, combinedSummary, input, tempHistory, availableTools)
	promptStr, err := promptObj.Encode()
	if err != nil {
		return "", fmt.Errorf("failed to encode prompt: %w", err)
	}

	// Step 4: Call LLM
	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: 0.7,
		MaxTokens:   512,
	}

	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	response := resp.Choices[0].Message.Content

	// Step 5: Update temp memory
	a.mu.Lock()
	a.tempMemory = append(a.tempMemory, client.ChatMessage{Role: "user", Content: input})
	a.tempMemory = append(a.tempMemory, client.ChatMessage{Role: "assistant", Content: response})

	// Optional: truncate if too long
	if len(a.tempMemory) > a.maxHistory {
		// Keep system message (index 0) + latest messages
		newMem := make([]client.ChatMessage, a.maxHistory)
		newMem[0] = a.tempMemory[0]
		copy(newMem[1:], a.tempMemory[len(a.tempMemory)-(a.maxHistory-1):])
		a.tempMemory = newMem
	}
	a.mu.Unlock()

	log.Infof("Agent %d processed chat → %s", a.AgentId, response)
	return response, nil
}

// Close is a no-op for HTTP clients (no persistent connections)
func (a *Agent) Close() {

	// HTTP clients don't need explicit close
}

// SetBoundMonitor records which monitor this agent is bound to
func (a *Agent) SetBoundMonitor(monitorID uint64) {
	a.boundMu.Lock()
	defer a.boundMu.Unlock()
	log.Infof("the agent %d have been bound in monitor %d", a.AgentId, monitorID)
	a.boundMonitorID = monitorID
}

// GetBoundMonitor returns the current bound monitor ID
func (a *Agent) GetBoundMonitor() uint64 {
	a.boundMu.RLock()
	defer a.boundMu.RUnlock()
	return a.boundMonitorID
}
func (a *Agent) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task := <-a.taskQueue:
			log.Infof("the agent:%d start processing task:%s", a.AgentId, task.Type)
			if err := a.handleTask(task); err != nil {
				log.Errorf("Agent %d failed to handle task: %v", a.AgentId, err)
			}
		}
	}
}
func (a *Agent) handleTask(task *AgentTask) error {
	var result string
	var err error

	log.Infof("Agent %d start processing task: %s (ID: %s)", a.AgentId, task.Type, task.ID)

	switch task.Type {
	case TaskTypeComm:
		result, err = a.handleCommTask(task)
		if err != nil {
			// Comm 任务出错后继续触发 Chat
			log.Warnf("Comm task error, but continuing: %v", err)
		}
		// Enqueue as Chat task
		chatTask := &AgentTask{
			Type:      TaskTypeChat,
			Content:   result,
			Timestamp: time.Now().Unix(),
			ID:        task.ID,
		}
		select {
		case a.taskQueue <- chatTask:
		default:
			log.Warnf("Task queue full, dropping chat task")
		}
		// Comm 任务本身不设置结果，等待 Chat 任务完成
		return nil

	case TaskTypeTempChat:
		result, err = a.TempChat(task.Content)

	case TaskTypeChat:
		result, err = a.handleChatTask(task)
		if err == nil && result != "" {
			toolCall, parseErr := prompt.ParseToolCallFromResponse(result)
			if parseErr == nil && toolCall.Action == "tool_call" {
				// 创建 Tool 任务
				toolTask := &AgentTask{
					ID:        task.ID,
					Type:      TaskTypeTool,
					ToolName:  toolCall.ToolName,
					Params:    toolCall.Parameters,
					Content:   toolCall.Thought,
					ParentID:  task.ID,
					Timestamp: time.Now().Unix(),
				}
				select {
				case a.taskQueue <- toolTask:
					log.Infof("Agent %d queued tool task: %s", a.AgentId, toolCall.ToolName)
				default:
					log.Warnf("Task queue full, dropping tool task")
				}
				// Chat 任务不设置结果，等待 Tool 任务完成
				return nil
			}
		}
	case TaskTypeTool:
		result, err = a.handleToolTask(task)
	case TaskTypeDecompose:
		result, err = a.handleDecomposeTask(task)
	case TaskTypeToolDAG: // New case
		result, err = a.handleToolDAGTask(task)
	case TaskTypeStandardTool:
		result, err = a.handleStandardToolTask(task)
	case TaskTypeReAct:
		result, err = a.handleReActTask(task)
	case TaskTypeAttack:
		// Attack模式：非终止时返回空result，不设TaskResult
		result, err = a.handleAttackReActTask(task)
		if result == "" && err == nil {
			// 中间步骤，不设result，下一轮会继续
			return nil
		}
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if task.ID != "" {
		a.SetTaskResult(task.ID, result, err)
	}
	if err != nil {
		log.Errorf("Agent %d failed to handle task %s: %v", a.AgentId, task.ID, err)
	}

	return nil
}

func (a *Agent) executeShellCommand(input map[string]interface{}) string {
	command, _ := input["command"].(string)
	if command == "" {
		return "Error: 'command' parameter is required"
	}

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	outputStr := string(output)
	// 截断
	if len(outputStr) > 4096 {
		outputStr = outputStr[:4096] + "\n...[truncated]"
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("[exit_code=%d]\n%s", exitErr.ExitCode(), outputStr)
		}
		return fmt.Sprintf("[error=%v]\n%s", err, outputStr)
	}
	return fmt.Sprintf("[exit_code=0]\n%s", outputStr)
}

func (a *Agent) updateKnowledgeBase(state *ReActState, parsed ReActParsed, observation string) {
	cmd, _ := parsed.ActionInput["command"].(string)
	if cmd == "" {
		return
	}

	// 记录有价值的发现
	if !strings.Contains(observation, "Error") &&
		!strings.Contains(observation, "No such file") &&
		!strings.Contains(observation, "Permission denied") &&
		len(observation) > 10 {
		state.KnowledgeBase[cmd] = observation
	}
	// 记录失败路径
	if strings.Contains(observation, "No such file") ||
		strings.Contains(observation, "Permission denied") {
		state.FailedPaths = append(state.FailedPaths, cmd)
	}
	// 自动推进攻击阶段
	if strings.Contains(cmd, "ls") || strings.Contains(cmd, "find") || strings.Contains(cmd, "pgrep") {
		state.AttackPhase = "recon"
	} else if strings.Contains(cmd, "cat /tmp/tc_data") || strings.Contains(cmd, "/proc/") {
		state.AttackPhase = "direct_read"
	} else if strings.Contains(cmd, "dmsetup") || strings.Contains(cmd, "key") {
		state.AttackPhase = "key_extract"
	} else if strings.Contains(cmd, "gdb") || strings.Contains(cmd, "dump") {
		state.AttackPhase = "memory_dump"
	}
}

// SubmitTask submits a task and returns a taskID for tracking
func (a *Agent) SubmitTask(task *AgentTask) (string, error) {
	if task.Timestamp == 0 {
		task.Timestamp = time.Now().Unix()
	}
	// 生成唯一 taskID
	if task.ID == "" {
		task.ID = fmt.Sprintf("task_%d_%d", a.AgentId, time.Now().UnixNano())
	}
	// 创建结果追踪
	result := &TaskResult{
		Done: make(chan struct{}),
	}
	a.taskResultsMu.Lock()
	a.taskResults[task.ID] = result
	a.taskResultsMu.Unlock()
	// 提交任务
	select {
	case a.taskQueue <- task:
		log.Debugf("Agent %d submitted task %s", a.AgentId, task.ID)
		return task.ID, nil
	default:
		// 队列满，清理结果追踪
		a.taskResultsMu.Lock()
		delete(a.taskResults, task.ID)
		a.taskResultsMu.Unlock()
		return "", fmt.Errorf("task queue full")
	}
}

// GetTaskResult waits for task completion and returns the result
func (a *Agent) GetTaskResult(taskID string, timeout time.Duration) (string, error) {
	a.taskResultsMu.RLock()
	result, exists := a.taskResults[taskID]
	a.taskResultsMu.RUnlock()

	if !exists {
		return "", fmt.Errorf("task %s not found", taskID)
	}

	// 等待任务完成
	select {
	case <-result.Done:
		// 任务完成，清理追踪
		a.taskResultsMu.Lock()
		delete(a.taskResults, taskID)
		a.taskResultsMu.Unlock()
		return result.Result, result.Error
	case <-time.After(timeout):
		return "", fmt.Errorf("task %s timeout", taskID)
	}
}

// SetTaskResult sets the result for a completed task
func (a *Agent) SetTaskResult(taskID string, result string, err error) {
	a.taskResultsMu.RLock()
	taskResult, exists := a.taskResults[taskID]
	a.taskResultsMu.RUnlock()

	if exists {
		taskResult.Result = result
		taskResult.Error = err
		close(taskResult.Done) // 关闭通道表示完成
	}
}
func (a *Agent) handleChatTask(task *AgentTask) (string, error) {
	content := task.Content
	if content == "" {
		return "", fmt.Errorf("chat task requires content")
	}

	var allSummaries []string
	var allMemories []string

	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()

	for _, client := range publicClients {
		if client == nil {
			continue
		}
		summary, memories, err := client.GetMemoryContext(time.Now().Unix(), content, 5)
		if err != nil {
			log.Warnf("Failed to get context from public memspace: %v", err)
			continue
		}
		if summary != "" {
			allSummaries = append(allSummaries, summary)
		}
		allMemories = append(allMemories, memories...)
	}
	combinedSummary := ""
	if len(allSummaries) > 0 {
		combinedSummary = strings.Join(allSummaries, "\n---\n")
	}

	a.mu.RLock()
	tempHistory := make([]client.ChatMessage, len(a.tempMemory))
	copy(tempHistory, a.tempMemory)
	a.mu.RUnlock()
	var availableTools []*configs.ToolDefinition
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		tools, err := msClient.ListTools()
		if err == nil && len(tools) > 0 {
			availableTools = append(availableTools, tools...)
		}
	}
	sysMsg := "You are an intelligent agent with access to shared memory and conversation history. Use both to answer the user's query."
	promptObj := prompt.NewChatPrompt(
		sysMsg,
		combinedSummary,
		content,
		tempHistory,
		availableTools,
	)
	promptStr, err := promptObj.Encode()
	if err != nil {
		return "", fmt.Errorf("failed to encode prompt: %w", err)
	}

	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: 0.7,
		MaxTokens:   512,
	}

	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	response := resp.Choices[0].Message.Content

	a.mu.Lock()
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "user",
		Content: content,
	})
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "assistant",
		Content: response,
	})
	memoryContent := fmt.Sprintf("Q: %s\nA: %s", content, response)
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		if err := msClient.WriteMemory(memoryContent, a.AgentId); err != nil {
			log.Warnf("Agent %d failed to write memory: %v", a.AgentId, err)
		}
	}
	a.mu.Unlock()
	log.Infof("Agent %d processed chat task → %s", a.AgentId, response)
	return response, nil
}
func (a *Agent) handleTempChatTask(task *AgentTask) error {
	content := task.Content
	if content == "" {
		return fmt.Errorf("temp chat task requires direct content")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "user",
		Content: content,
	})
	sysMsg := "You are a helpful AI assistant."
	// history 不包含刚加入的 user message
	history := a.tempMemory[:len(a.tempMemory)-1]
	promptObj := prompt.NewTempChatPrompt(sysMsg, content, history)
	promptStr, err := promptObj.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode prompt: %w", err)
	}
	// Unlock during network call
	a.mu.Unlock()

	// Call LLM with full context
	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}}, // or parse as structured if your chat server supports it
		Temperature: 0.7,
		MaxTokens:   512,
	}

	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from LLM")
	}
	response := resp.Choices[0].Message.Content
	// Lock again to update memory
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "assistant",
		Content: response,
	})

	log.Infof("Agent %d processed temp chat: %s → %s", a.AgentId, content, response)
	return nil
}
func (a *Agent) handleCommTask(task *AgentTask) (string, error) {
	// 如果 Content 已提供，直接使用
	var content string
	var err error
	if task.Content != "" {
		content = task.Content
	} else {
		client, ok := a.GetMemSpaceClient(0) // 假设公共 MemSpace ID = 0
		if !ok {
			return "", fmt.Errorf("no memspace client available for comm task")
		}
		content, err = client.GetMemoryByKey([]byte(task.Key))
		log.Debugf("get the content from memspace: %s", content)
		if err != nil {
			return "", fmt.Errorf("failed to get memory by key: %w", err)
		}
	}
	log.Infof("the cotent: %s, req.Content:%s", content, task.Content)
	if err != nil {
		return content, err
	}
	return content, nil
}

// bindingMemSpace binds the agent to a MemSpace via MemSpaceManager
func (a *Agent) bindingMemSpace(memSpaceConfig *configs.MemSpaceConfig) error {
	log.Info("[agent] get the binding request from the server")
	if memSpaceConfig == nil {
		log.Fatal("[agent] get the binding request from the server is nil")
		return fmt.Errorf("memspace config is nil")
	}
	// 验证 Manager 客户端是否存在
	if a.memSpaceManagerClient == nil {
		log.Fatal("memspace manager client not initialized")
		return fmt.Errorf("memspace manager client not initialized")
	}
	// 获取 Agent 自身的 HTTP 地址（从 config 或默认）
	agentAddr := a.httpAddr
	agentRole := a.role
	// Step 1: 调用 MemSpaceManager 进行绑定
	log.Infof("[Agent %d] Binding to MemSpace %d via Manager...", a.AgentId, memSpaceConfig.MemSpaceID)
	err := a.memSpaceManagerClient.BindMemSpace(
		a.AgentId,
		memSpaceConfig.MemSpaceID,
		agentAddr,
		agentRole,
	)
	if err != nil {
		log.Errorf("can not bind memspace to agent through the memspace manager %d: %v", a.AgentId, err)
		return fmt.Errorf("failed to bind memspace via manager: %w", err)
	}
	log.Infof("[Agent %d] Successfully bound to MemSpace %d via Manager", a.AgentId, memSpaceConfig.MemSpaceID)
	// Step 2: 本地创建 MemSpaceClient（用于后续直接通信）
	baseURL := memSpaceConfig.HttpAddr
	msClient := client.NewMemSpaceClient(baseURL)
	// Step 3: 存储到本地 map
	a.mu.Lock()
	defer a.mu.Unlock()
	a.memSpaceClients[memSpaceConfig.MemSpaceID] = msClient

	// 更新 private/public references
	if memSpaceConfig.Type == "private" {
		a.privateMemSpaceClients = msClient
	} else if memSpaceConfig.Type == "public" {
		// Avoid duplicates
		exists := false
		for _, c := range a.publicMemSpaceClients {
			if c != nil && c.BaseURL == msClient.BaseURL {
				exists = true
				break
			}
		}
		if !exists {
			a.publicMemSpaceClients = append(a.publicMemSpaceClients, msClient)
		}
	}
	log.Infof("Agent %d bound to MemSpace %d (%s) locally", a.AgentId, memSpaceConfig.MemSpaceID, memSpaceConfig.Type)
	return nil
}
func (a *Agent) unBindingMemSpace(memID uint64) error {
	// 验证 Manager 客户端是否存在
	if a.memSpaceManagerClient == nil {
		return fmt.Errorf("memspace manager client not initialized")
	}
	// Step 1: 调用 MemSpaceManager 进行解绑
	log.Infof("[Agent %d] Unbinding from MemSpace %d via Manager...", a.AgentId, memID)
	err := a.memSpaceManagerClient.UnbindMemSpace(a.AgentId, memID)
	if err != nil {
		log.Warnf("Failed to unbind memspace via manager: %v", err)
		// 不返回错误，继续清理本地状态
	}
	log.Infof("[Agent %d] Successfully unbound from MemSpace %d via Manager", a.AgentId, memID)

	// Step 2: 清理本地状态
	a.mu.Lock()
	defer a.mu.Unlock()
	memspaceClient, exists := a.memSpaceClients[memID]
	if !exists {
		return fmt.Errorf("memspace %d not bound locally", memID)
	}
	// Remove from main map
	delete(a.memSpaceClients, memID)

	// Remove from private reference
	if a.privateMemSpaceClients != nil && a.privateMemSpaceClients == memspaceClient {
		a.privateMemSpaceClients = nil
	}

	// Remove from public slice
	newPublic := make([]*client.MemSpaceClient, 0, len(a.publicMemSpaceClients))
	for _, c := range a.publicMemSpaceClients {
		if c != memspaceClient {
			newPublic = append(newPublic, c)
		}
	}
	a.publicMemSpaceClients = newPublic
	log.Infof("Agent %d unbound from MemSpace %d locally", a.AgentId, memID)
	return nil
}

// pkg/runtime/agent/agent.go

// Communicate sends a message to another agent via public MemSpace
func (a *Agent) Communicate(targetAgentID uint64, key string, content string) (string, error) {
	log.Infof("[agent %d] communicating with key: %s,content:%s", a.AgentId, key, content)
	if content == "" {
		return "", fmt.Errorf("content cannot be empty")
	}
	// Step 1: 查找目标 Agent 在哪个 Public MemSpace 的通讯录中
	var targetMemSpaceClient *client.MemSpaceClient
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		// 查询通讯录
		agents, err := msClient.ListAgents()
		log.Debugf("get list")
		if err != nil {
			log.Warnf("Failed to list agents from memspace: %v", err)
			continue
		}
		// 查找目标 Agent
		for _, agent := range agents {
			agentID, err := strconv.ParseUint(agent.AgentID, 10, 64)
			if err != nil {
				continue
			}
			if agentID == targetAgentID {
				targetMemSpaceClient = msClient
				//targetAddr = agent.Addr
				break
			}
		}
		if targetMemSpaceClient != nil {
			log.Infof("find memspaceClient")
			break
		}
	}

	if targetMemSpaceClient == nil {
		return "", fmt.Errorf("target agent %d not found in any public memspace registry", targetAgentID)
	}
	// Step 2: update the persist region
	// Step 3: 通过通讯区发送消息（记录通讯元数据）
	result, err := targetMemSpaceClient.SendMessage(a.AgentId, targetAgentID, key, content)
	if err != nil {
		log.Warnf("Failed to send comm message: %v", err)
		// 不返回错误，消息已写入
	}
	log.Infof("Agent %d communicated with Agent %d via memspace (key: %s)", a.AgentId, targetAgentID, key)
	return result, nil
}

// pkg/runtime/agent/agent.go

// handleToolTask processes a tool invocation task
func (a *Agent) handleToolTask(task *AgentTask) (string, error) {
	if task.ToolName == "" {
		return "", fmt.Errorf("tool_name is required")
	}
	log.Infof("Agent %d executing tool: %s, params: %v", a.AgentId, task.ToolName, task.Params)
	// 查找工具定义
	var toolDef *configs.ToolDefinition
	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		tool, err := msClient.GetTool(task.ToolName)
		if err == nil {
			toolDef = tool
			break
		}
	}

	if toolDef == nil {
		return "", fmt.Errorf("tool '%s' not found", task.ToolName)
	}

	// 调用工具
	output, err := a.invokeTool(toolDef, task.Params)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// ✅ 返回格式化的结果（包含工具输出）
	response := fmt.Sprintf("Tool '%s' executed successfully. Results: %v",
		task.ToolName, output)

	log.Infof("Agent %d tool %s completed: %v", a.AgentId, task.ToolName, output)
	return response, nil
}

// recordToolExec records tool execution start in MemSpace
func (a *Agent) recordToolExec(toolName string, params map[string]interface{}) (uint64, error) {
	// 找到第一个有 ToolRegion 的 MemSpace
	for _, msClient := range a.publicMemSpaceClients {
		if msClient != nil {
			// 需要通过 HTTP API 调用，这里简化处理
			// 后续可以通过 MemSpaceClient 添加 RecordToolExec 方法
			return 0, nil
		}
	}
	return 0, nil
}

// completeToolExec records tool execution completion
func (a *Agent) completeToolExec(seq uint64, output map[string]interface{}, err error) {

}

// invokeTool calls the actual tool endpoint via the appropriate executor
// todo 多类型任务支持
func (a *Agent) invokeTool(tool *configs.ToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	if tool.Endpoint == "" && tool.ExecType == "" {
		// No endpoint and no exec type → mock mode
		log.Infof("Agent %d mock executing tool: %s", a.AgentId, tool.Name)
		return map[string]interface{}{"status": "mock_success"}, nil
	}
	execType := tool.ExecType
	if execType == "" {
		execType = "http" // default to HTTP
	}

	req := &tool_executors.ToolRequest{
		ToolName:   tool.Name,
		Endpoint:   tool.Endpoint,
		Parameters: params,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log.Infof("Agent %d dispatching tool '%s' to %s executor (endpoint: %s)",
		a.AgentId, tool.Name, execType, tool.Endpoint)
	resp, err := a.toolDispatcher.Dispatch(ctx, execType, req)
	if err != nil {
		log.Errorf("Agent %d tool '%s' execution failed: %v", a.AgentId, tool.Name, err)
		return nil, err
	}
	if !resp.Success {
		log.Warnf("Agent %d tool '%s' returned error: %s", a.AgentId, tool.Name, resp.Error)
		return map[string]interface{}{
			"status": "failed",
			"error":  resp.Error,
		}, nil
	}

	return resp.Data, nil
}
func (a *Agent) handleDecomposeTask(task *AgentTask) (string, error) {
	if task.Content == "" {
		return "", fmt.Errorf("decompose task requires content (the task description)")
	}

	maxRetry := task.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 5
	}

	availableTools := task.AvailableTools
	if len(availableTools) == 0 {
		availableTools = a.collectAvailableToolNames()
	}

	p := prompt.NewTaskDecomposePrompt(
		task.Content,
		availableTools,
		nil,
		task.AvailableMemTags,
	)

	msgs := p.BuildMessages()

	// 累积式对话历史：system + user 初始消息 + 后续的 assistant 回复和 user 纠错
	conversation := make([]client.ChatMessage, 0, len(msgs)+10)
	for _, m := range msgs {
		conversation = append(conversation, client.ChatMessage{
			Role:    m["role"],
			Content: m["content"],
		})
	}

	var definition *viewspace.TaskDefinition
	var lastErrors string

	for attempt := 1; attempt <= maxRetry; attempt++ {
		log.Infof("Agent %d decompose attempt %d/%d", a.AgentId, attempt, maxRetry)

		req := client.ChatCompletionRequest{
			Messages:    conversation,
			Temperature: 0.3,
			MaxTokens:   4096,
		}

		resp, err := a.chatClient.ChatCompletion(req)
		if err != nil {
			log.Warnf("Agent %d LLM call failed on attempt %d: %v", a.AgentId, attempt, err)
			lastErrors = fmt.Sprintf("LLM call failed: %v", err)
			continue
		}

		if len(resp.Choices) == 0 {
			lastErrors = "LLM returned no response."
			continue
		}

		response := resp.Choices[0].Message.Content

		conversation = append(conversation, client.ChatMessage{
			Role:    "assistant",
			Content: response,
		})

		// Extract JSON
		jsonStr := viewspace.ExtractJSON(response)
		if jsonStr == "" {
			lastErrors = "Your response did not contain valid JSON. Please output ONLY a JSON object with meta, viewspaces, and dependencies."
			log.Warnf("Agent %d attempt %d: no JSON found in response", a.AgentId, attempt)

			conversation = append(conversation, client.ChatMessage{
				Role:    "user",
				Content: lastErrors,
			})
			continue
		}

		// Parse and validate
		result := viewspace.Parse([]byte(jsonStr))

		if result.HasErrors() {
			lastErrors = result.FormatErrorsForLLM()
			log.Warnf("Agent %d attempt %d: %d validation errors", a.AgentId, attempt, len(result.Errors))
			for _, e := range result.Errors {
				log.Warnf("  %s", e.Error())
			}

			conversation = append(conversation, client.ChatMessage{
				Role:    "user",
				Content: lastErrors,
			})
			continue
		}

		// Warnings
		for _, w := range result.Warnings {
			log.Warnf("Agent %d decompose warning: %s", a.AgentId, w)
		}

		definition = result.Definition
		log.Infof("Agent %d decomposition succeeded on attempt %d: %d viewspaces",
			a.AgentId, attempt, len(definition.ViewSpaces))
		break
	}

	if definition == nil {
		return "", fmt.Errorf("task decomposition failed after %d attempts, last errors: %s", maxRetry, lastErrors)
	}

	resultJSON, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize task definition: %w", err)
	}

	a.taskResultsMu.Lock()
	if taskResult, exists := a.taskResults[task.ID]; exists {
		taskResult.TaskDefinition = definition
	}
	a.taskResultsMu.Unlock()

	log.Infof("Agent %d decompose completed: %s (%d viewspaces)",
		a.AgentId, definition.Meta.TaskID, len(definition.ViewSpaces))

	return string(resultJSON), nil
}
func (a *Agent) handleAttackReActTask(task *AgentTask) (string, error) {
	state := task.ReActState
	if state == nil {
		state = &ReActState{
			OriginalQuery: task.Content,
			MaxIterations: a.attackConfig.MaxIterations,
			ParentTaskID:  task.ID,
			KnowledgeBase: make(map[string]string),
			FailedPaths:   []string{},
			AttackPhase:   "recon",
		}
		fmt.Printf("\n[ATTACK] Starting attack: %s\n", task.Content)
		fmt.Printf("[ATTACK] Max iterations: %d\n\n", state.MaxIterations)
		// ★ 从 MemSpace 加载历史攻击报告
		historyContent := a.loadAttackHistory()
		log.Infof("The len of the history is %d", len(historyContent))
		if historyContent != "" {
			fmt.Printf("[ATTACK] Loaded previous attack history from MemSpace\n")
			state.PreviousHistory = historyContent
		} else {
			fmt.Printf("[ATTACK] No previous attack history found\n")
		}
	}

	// ── 终止条件1：达到最大轮数 ──
	if state.Iteration >= state.MaxIterations {
		fmt.Printf("\n[ATTACK] Max iterations reached (%d). Generating report.\n", state.MaxIterations)
		finalAnswer := "Max iterations reached without completing the attack."
		if len(state.Steps) > 0 {
			finalAnswer = state.Steps[len(state.Steps)-1].Observation
		}
		a.generateAndSaveReport(state, finalAnswer)
		fmt.Printf("[ATTACK] Final Answer received. Generating report...\n")
		report := a.BuildAttackReport(state, finalAnswer)
		reportJSON, _ := report.Serialize()
		memContent := "ATTACK_REPORT:" + reportJSON
		if a.privateMemSpaceClients != nil {
			if err := a.privateMemSpaceClients.WriteMemory(memContent, a.AgentId); err != nil {
				fmt.Printf("[ATTACK] Failed to save report to MemSpace: %v\n", err)
			} else {
				fmt.Printf("[ATTACK] Report saved to MemSpace (run_id: %s)\n", report.RunID)
			}
		}
		return finalAnswer, nil
	}

	// ── 构建 prompt ──
	promptStr := buildAttackReActPrompt(
		state.OriginalQuery,
		state.Steps,
		state.Iteration,
		state.MaxIterations,
		state.KnowledgeBase,
		state.FailedPaths,
		a.attackLibrary,
		state.PreviousHistory,
	)

	fmt.Printf("[ATTACK] Step %d/%d — Calling LLM...\n", state.Iteration+1, state.MaxIterations)

	// ── 调LLM，失败时把错误写入Observation继续循环 ──
	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: a.attackConfig.Temperature,
		MaxTokens:   a.attackConfig.MaxTokens,
	}
	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		fmt.Printf("[ATTACK] LLM call failed: %v\n", err)
		a.injectNextStep(task, state, &ReActParsed{
			Thought: "LLM call failed",
			Action:  "SYSTEM_ERROR",
		}, fmt.Sprintf("LLM call error: %v. Please retry the same command or try a different approach.", err))
		return "", nil
	}
	if len(resp.Choices) == 0 {
		fmt.Printf("[ATTACK] LLM returned empty response\n")
		a.injectNextStep(task, state, &ReActParsed{
			Thought: "LLM returned empty",
			Action:  "SYSTEM_ERROR",
		}, "LLM returned no response. Please retry.")
		return "", nil
	}
	response := resp.Choices[0].Message.Content

	// ── 打印原始输出 ──
	fmt.Printf("[ATTACK] LLM Raw Response:\n---\n%s\n---\n\n", response)

	// ── 解析 ──
	parsed := parseAttackReActResponse(response)
	fmt.Printf("[ATTACK] Parsed:\n")
	fmt.Printf("  Thought:       %s\n", truncate(parsed.Thought, 200))
	fmt.Printf("  Action:        %s\n", parsed.Action)
	fmt.Printf("  ActionInput:   %v\n", parsed.ActionInput)
	fmt.Printf("  IsFinalAnswer: %v\n\n", parsed.IsFinalAnswer)

	// ── 终止条件2：有效FinalAnswer ──
	if parsed.IsFinalAnswer && parsed.FinalAnswer != "" {
		fmt.Printf("[ATTACK] Final Answer received. Generating report...\n")
		a.generateAndSaveReport(state, parsed.FinalAnswer)
		report := a.BuildAttackReport(state, parsed.FinalAnswer)
		reportJSON, _ := report.Serialize()
		memContent := "ATTACK_REPORT:" + reportJSON
		if a.privateMemSpaceClients != nil {
			if err := a.privateMemSpaceClients.WriteMemory(memContent, a.AgentId); err != nil {
				fmt.Printf("[ATTACK] Failed to save report to MemSpace: %v\n", err)
			} else {
				fmt.Printf("[ATTACK] Report saved to MemSpace (run_id: %s)\n", report.RunID)
			}
		}

		return parsed.FinalAnswer, nil
	}

	// ── 解析失败：把错误注入Observation，继续循环 ──
	if parsed.Action == "" {
		fmt.Printf("[ATTACK] Parse failed: no valid Action found. Injecting retry hint.\n")
		a.injectNextStep(task, state, &ReActParsed{
			Thought: parsed.Thought,
			Action:  "PARSE_ERROR",
		}, "Your response did not contain a valid Action. Please respond in this exact format:\nThought: <reasoning>\nAction: exec_cmd\nAction Input: {\"command\": \"your_shell_command\"}")
		return "", nil
	}

	// ── 执行 Action ──
	var observation string
	switch parsed.Action {
	case "exec_cmd":
		observation = a.executeShellCommandWithTimeout(parsed.ActionInput, a.attackConfig.TimeoutPerStep)
	case "chat":
		query, _ := parsed.ActionInput["query"].(string)
		chatResp, chatErr := a.chatClient.QuickChat(query)
		if chatErr != nil {
			observation = fmt.Sprintf("Chat error: %v", chatErr)
		} else {
			observation = chatResp.Response
		}
	default:
		observation = fmt.Sprintf("Unknown action: %s. Use exec_cmd or chat.", parsed.Action)
	}

	fmt.Printf("[ATTACK] Observation:\n---\n%s\n---\n\n", truncate(observation, 500))

	// ── 更新知识图谱 ──
	a.updateKnowledgeBase(state, parsed, observation)

	// ── 注入下一步 ──
	a.injectNextStep(task, state, &parsed, observation)
	fmt.Printf("[ATTACK] Step %d complete. Proceeding to step %d.\n\n", state.Iteration, state.Iteration+1)

	return "", nil
}

// 注入下一步task，统一处理所有"非终止"情况
func (a *Agent) injectNextStep(task *AgentTask, state *ReActState, parsed *ReActParsed, observation string) {
	step := ReActStep{
		Thought:     parsed.Thought,
		Action:      parsed.Action,
		ActionInput: parsed.ActionInput,
		Observation: observation,
	}
	state.Steps = append(state.Steps, step)
	state.Iteration++

	nextTask := &AgentTask{
		ID:         task.ID,
		Type:       TaskTypeAttack,
		Content:    state.OriginalQuery,
		ReActState: state,
		Timestamp:  time.Now().Unix(),
	}
	select {
	case a.taskQueue <- nextTask:
		log.Infof("Agent %d attack step %d done, next round", a.AgentId, state.Iteration)
	default:
		fmt.Printf("[ATTACK] ERROR: task queue full at iteration %d\n", state.Iteration)
	}
}

// 生成报告并保存
func (a *Agent) generateAndSaveReport(state *ReActState, finalAnswer string) {
	if !a.attackConfig.EnableReport {
		return
	}
	report := a.generateAttackReport(state, finalAnswer)
	reportDir := a.attackConfig.ReportDir
	os.MkdirAll(reportDir, 0755)
	reportPath := fmt.Sprintf("%s/report_%s.md", reportDir, time.Now().Format("20060102_150405"))
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		fmt.Printf("[ATTACK] Failed to write report: %v\n", err)
	} else {
		fmt.Printf("[ATTACK] Report saved: %s\n", reportPath)
	}
}

func (a *Agent) executeShellCommandWithTimeout(input map[string]interface{}, timeoutSec int) string {
	command, _ := input["command"].(string)
	if command == "" {
		return "Error: 'command' parameter is required"
	}

	timeout := time.Duration(timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	outputStr := string(output)
	if len(outputStr) > 4096 {
		outputStr = outputStr[:4096] + "\n...[truncated]"
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("[exit_code=%d]\n%s", exitErr.ExitCode(), outputStr)
		}
		return fmt.Sprintf("[error=%v]\n%s", err, outputStr)
	}
	return fmt.Sprintf("[exit_code=0]\n%s", outputStr)
}

// collectAvailableToolNames gathers tool names from all bound MemSpaces
func (a *Agent) collectAvailableToolNames() []string {
	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()

	seen := map[string]bool{}
	var tools []string

	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		toolDefs, err := msClient.ListTools()
		if err != nil {
			continue
		}
		for _, t := range toolDefs {
			if !seen[t.Name] {
				seen[t.Name] = true
				tools = append(tools, t.Name)
			}
		}
	}

	return tools
}

func (a *Agent) handleToolDAGTask(task *AgentTask) (string, error) {
	log.Infof("Agent %d executing ToolDAG task: %s", a.AgentId, task.ID)

	// ============================================================
	// 1. Get memspace_id from task.Params (with JSON-safe type conversion)
	// ============================================================
	var memspaceID uint64

	// Option A: Direct graph provided in task
	if task.ToolGraph != nil {
		// Use provided graph directly
	} else {
		memspaceIDRaw, ok := task.Params["memspace_id"]
		if !ok {
			return "", fmt.Errorf("ToolDAG task missing tool_graph or memspace_id")
		}
		switch v := memspaceIDRaw.(type) {
		case uint64:
			memspaceID = v
		case float64:
			memspaceID = uint64(v)
		case int:
			memspaceID = uint64(v)
		case int64:
			memspaceID = uint64(v)
		case string:
			// Fallback: parse from string
			parsed, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return "", fmt.Errorf("memspace_id must be numeric, got string: %s", v)
			}
			memspaceID = parsed
		default:
			return "", fmt.Errorf("memspace_id has unexpected type %T: %v", memspaceIDRaw, memspaceIDRaw)
		}
	}
	// ============================================================
	// 2. Load ToolDAG from MemSpace (if not provided directly)
	// ============================================================
	var dag *configs.ToolDAG
	var err error
	if task.ToolGraph != nil {
		dag = task.ToolGraph
	} else if memspaceID > 0 {
		msClient, ok := a.GetMemSpaceClient(memspaceID)
		if !ok {
			return "", fmt.Errorf("memspace %d not bound to agent %d", memspaceID, a.AgentId)
		}
		dag, err = msClient.LoadToolDAG()
		if err != nil {
			return "", fmt.Errorf("failed to load ToolDAG from memspace %d: %w", memspaceID, err)
		}
	}

	if dag == nil || len(dag.Nodes) == 0 {
		return "", fmt.Errorf("ToolDAG is empty or not found")
	}

	log.Infof("Agent %d loaded ToolDAG: %d nodes, %d edges", a.AgentId, len(dag.Nodes), len(dag.Edges))

	// ============================================================
	// 3. Execute tools with topological scheduling
	// ============================================================
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	executor := NewToolDAGExecutor(a, dag, memspaceID, task.ID)
	results, err := executor.Execute(ctx)
	if err != nil {
		return "", fmt.Errorf("ToolDAG execution failed: %w", err)
	}
	// ============================================================
	// 4. Record audit to MemSpace (async, non-blocking)
	// ============================================================
	if memspaceID > 0 && len(results) > 0 {
		go func() {
			msClient, ok := a.GetMemSpaceClient(memspaceID)
			if !ok {
				return
			}
			// Use batch recording for efficiency
			_ = msClient.RecordToolExecBatch(results)
		}()
	}

	// ============================================================
	// 5. Return aggregated results as JSON string
	// ============================================================
	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	log.Infof("Agent %d ToolDAG task %s completed: %d tools executed",
		a.AgentId, task.ID, len(results))
	// In handleToolDAGTask, after executor.Execute():
	if memspaceID > 0 && len(results) > 0 {
		msClient, ok := a.GetMemSpaceClient(memspaceID)
		if ok {
			// For testing: respect TEST_SYNC_AUDIT env var
			if os.Getenv("TEST_SYNC_AUDIT") == "1" {
				// Sync mode for tests
				_ = msClient.RecordToolExecBatch(results)
			} else {
				// Async mode for production
				go func() {
					_ = msClient.RecordToolExecBatch(results)
				}()
			}
		}
	}
	return string(resultJSON), nil
}

// loadToolDAGFromMemSpace loads the tool dependency graph from MemSpace
func (a *Agent) loadToolDAGFromMemSpace(memSpaceID uint64) (*configs.ToolDAG, error) {
	msClient, ok := a.GetMemSpaceClient(memSpaceID)
	if !ok {
		return nil, fmt.Errorf("memspace %d not bound", memSpaceID)
	}

	// Call MemSpace HTTP API to load DAG
	// Note: Ensure MemSpaceClient has this method (see next section)
	return msClient.LoadToolDAG()
}

// recordToolExecToMemSpace records tool execution result for auditing
func (a *Agent) recordToolExecToMemSpace(memSpaceID uint64, result *configs.ToolExecResult) {
	msClient, ok := a.GetMemSpaceClient(memSpaceID)
	if !ok {
		return
	}

	go func() {
		msClient.RecordToolExec(result.ToolName, result.Output, result.Error)
	}()
}

// getToolDefinition retrieves tool definition from MemSpace
func (a *Agent) getToolDefinition(toolName string, memSpaceID uint64) (*configs.ToolDefinition, error) {
	msClient, ok := a.GetMemSpaceClient(memSpaceID)
	if !ok {
		return nil, fmt.Errorf("memspace %d not bound", memSpaceID)
	}
	return msClient.GetTool(toolName)
}

// handleStandardToolTask is the entry point for Standard Tool execution
func (a *Agent) handleStandardToolTask(task *AgentTask) (string, error) {
	if task.ToolName == "" {
		return "", fmt.Errorf("tool_name is required")
	}

	log.Infof("Agent %d executing STANDARD tool: %s", a.AgentId, task.ToolName)

	// 1. 查找标准工具定义 (使用 Standard 前缀的查找函数)
	stdToolDef, err := a.findStandardToolDefinition(task.ToolName)
	if err != nil {
		return "", fmt.Errorf("standard tool '%s' not found: %w", task.ToolName, err)
	}

	// 2. 执行标准工具 (使用 Standard 前缀的执行函数)
	output, err := a.invokeStandardTool(stdToolDef, task.Params)
	if err != nil {
		return "", fmt.Errorf("standard tool execution failed: %w", err)
	}

	// 3. 返回结果
	response := fmt.Sprintf("Standard Tool '%s' executed successfully. Results: %v", task.ToolName, output)
	log.Infof("Agent %d standard tool %s completed", a.AgentId, task.ToolName)
	return response, nil
}

// findStandardToolDefinition searches for a tool in all mounted MemSpaces
func (a *Agent) findStandardToolDefinition(toolName string) (*configs.StandardToolDefinition, error) {
	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()
	// todo emmm遍历效率会不会太低了但是自己玩无所谓了其实
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		// 调用 Client 的 Standard 接口
		tool, err := msClient.GetStandardTool(toolName)
		if err == nil {
			return tool, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// invokeStandardTool dispatches to the correct executor based on Type
func (a *Agent) invokeStandardTool(tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	// 设置超时
	timeout := time.Duration(tool.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Infof("Agent %d dispatching STANDARD tool '%s' (Type: %s)", a.AgentId, tool.Name, tool.Type)
	// 根据 Type 路由
	switch tool.Type {
	case configs.TypeHTTP:
		return a.standardExecutors.HTTP.Execute(ctx, tool, params)
	case configs.TypeShell:
		return a.standardExecutors.Shell.Execute(ctx, tool, params)
	case configs.TypeDelegate:
		// Claude Code 走这里
		return a.standardExecutors.Delegate.Execute(ctx, tool, params)
	case configs.TypeMCP:
		return a.standardExecutors.MCP.Execute(ctx, tool, params)
	default:
		return nil, fmt.Errorf("unsupported standard tool type: %s", tool.Type)
	}
}

func (a *Agent) handleReActTask(task *AgentTask) (string, error) {
	state := task.ReActState
	if state == nil {
		// 首次调用，初始化
		state = &ReActState{
			OriginalQuery: task.Content,
			MaxIterations: 5,
			ParentTaskID:  task.ID,
		}
	}

	// =========================================
	// 1. 每轮都拿最新的 memoryContext
	// =========================================
	a.mu.RLock()
	publicClients := make([]*client.MemSpaceClient, len(a.publicMemSpaceClients))
	copy(publicClients, a.publicMemSpaceClients)
	a.mu.RUnlock()

	var allSummaries []string
	var allMemories []string
	for _, msClient := range publicClients {
		if msClient == nil {
			continue
		}
		summary, memories, err := msClient.GetMemoryContext(
			time.Now().Unix(), state.OriginalQuery, 5,
		)
		if err != nil {
			continue
		}
		if summary != "" {
			allSummaries = append(allSummaries, summary)
		}
		allMemories = append(allMemories, memories...)
	}
	combinedSummary := strings.Join(allSummaries, "\n---\n")

	// =========================================
	// 2. 构建 ReAct prompt
	// =========================================
	promptStr := buildReActPrompt(
		state.OriginalQuery,
		combinedSummary,
		allMemories,
		state.Steps,
		state.Iteration,
		state.MaxIterations,
		a.attackLibrary,
	)

	// =========================================
	// 3. 调 LLM
	// =========================================
	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: 0.3, // ReAct 需要更确定性的输出
		MaxTokens:   1024,
	}
	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	response := resp.Choices[0].Message.Content

	// =========================================
	// 4. 解析 LLM 输出：Final Answer 还是 Action？
	// =========================================
	parsed := parseAttackReActResponse(response)

	if parsed.IsFinalAnswer || state.Iteration >= state.MaxIterations {
		// 到达最终答案或超过迭代上限
		// 写回记忆
		for _, msClient := range publicClients {
			if msClient == nil {
				continue
			}
			memContent := fmt.Sprintf("Query: %s\nFinal: %s", state.OriginalQuery, parsed.FinalAnswer)
			msClient.WriteMemory(memContent, a.AgentId)
		}
		return parsed.FinalAnswer, nil
	}

	// =========================================
	// 5. 执行 Action，拿到 Observation
	// =========================================
	// 这里我们的Action是工具的名称
	observation := a.executeReActAction(parsed.Action, parsed.ActionInput)

	// =========================================
	// 6. 记录这一步，注入下一轮 task
	// =========================================
	step := ReActStep{
		Thought:     parsed.Thought,
		Action:      parsed.Action,
		ActionInput: parsed.ActionInput,
		Observation: observation,
	}
	state.Steps = append(state.Steps, step)
	state.Iteration++

	nextTask := &AgentTask{
		ID:         task.ID, // 继承 ID，让外部 WaitTaskResult 一路追踪到底
		Type:       TaskTypeReAct,
		Content:    state.OriginalQuery,
		ReActState: state,
		Timestamp:  time.Now().Unix(),
	}

	select {
	case a.taskQueue <- nextTask:
		log.Infof("Agent %d ReAct step %d done, injected next round (action=%s)",
			a.AgentId, state.Iteration, parsed.Action)
	default:
		return "", fmt.Errorf("task queue full, ReAct aborted at iteration %d", state.Iteration)
	}

	// 不设 result，让下一轮的 task 来设
	return "", nil
}

func (a *Agent) executeReActAction(action string, input map[string]interface{}) string {
	// 检查是否是内置动作
	switch action {
	case "search_memory":
		// 语义搜索记忆
		//query, _ := input["query"].(string)
		//var results []string
		//for _, msClient := range a.publicMemSpaceClients {
		//	if msClient == nil {
		//		continue
		//	}
		//	_, memories, err := msClient.GetMemoryContext(time.Now().Unix(), query, 3)
		//	if err != nil {
		//		continue
		//	}
		//	results = append(results, memories...)
		//}
		//if len(results) == 0 {
		//	return "No relevant memories found."
		//}
		//return strings.Join(results, "\n---\n")
		return "current system do not support search"
	case "chat":
		// 直接调 LLM 不走 ReAct 循环
		query, _ := input["query"].(string)
		resp, err := a.chatClient.QuickChat(query)
		if err != nil {
			return fmt.Sprintf("Chat error: %v", err)
		}
		return resp.Response
	default:
		// 当作外部工具，复用现有的 invokeTool
		toolDef := a.findToolByName(action)
		if toolDef == nil {
			return fmt.Sprintf("Unknown action: %s", action)
		}
		output, err := a.invokeTool(toolDef, input)
		if err != nil {
			return fmt.Sprintf("Tool error: %v", err)
		}
		jsonOut, _ := json.Marshal(output)
		return string(jsonOut)
	}
}

func (a *Agent) findToolByName(name string) *configs.ToolDefinition {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, msClient := range a.publicMemSpaceClients {
		if msClient == nil {
			continue
		}
		tool, err := msClient.GetTool(name)
		if err == nil {
			return tool
		}
	}
	return nil
}
func (a *Agent) generateAttackReport(state *ReActState, finalAnswer string) string {
	var b strings.Builder
	b.WriteString("You are a security analyst. Generate a professional penetration test report in Markdown.\n\n")

	b.WriteString("## Context\n")
	b.WriteString("- Target: TrustCapsule sandbox (bwrap-based isolation)\n")
	b.WriteString("- Attacker: Host root user\n")
	b.WriteString(fmt.Sprintf("- Total steps: %d\n\n", state.Iteration))

	b.WriteString("## Attack Timeline\n")
	for i, s := range state.Steps {
		b.WriteString(fmt.Sprintf("### Step %d\n", i+1))
		b.WriteString(fmt.Sprintf("- Thought: %s\n", s.Thought))
		cmd, _ := s.ActionInput["command"].(string)
		if cmd != "" {
			b.WriteString(fmt.Sprintf("- Command: `%s`\n", cmd))
		}
		obs := s.Observation
		if len(obs) > 500 {
			obs = obs[:500] + "..."
		}
		b.WriteString(fmt.Sprintf("- Result: %s\n\n", obs))
	}

	if len(state.KnowledgeBase) > 0 {
		b.WriteString("## Key Findings\n")
		for k, v := range state.KnowledgeBase {
			val := v
			if len(val) > 300 {
				val = val[:300] + "..."
			}
			b.WriteString(fmt.Sprintf("- `%s`: %s\n", k, val))
		}
		b.WriteString("\n")
	}

	if len(state.FailedPaths) > 0 {
		b.WriteString("## Blocked Paths\n")
		for _, p := range state.FailedPaths {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Conclusion\n")
	b.WriteString(finalAnswer)
	b.WriteString("\n\n")

	b.WriteString("Generate a Markdown report with these sections:\n")
	b.WriteString("1. Executive Summary\n2. Target Environment\n3. Attack Methodology\n4. Critical Findings (with severity)\n5. Evidence\n6. Recommendations\n7. Conclusion\n")
	b.WriteString("Output ONLY the Markdown.\n")

	promptStr := b.String()

	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: 0.3,
		MaxTokens:   4096,
	}
	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		fmt.Printf("[ATTACK] Report generation via LLM failed: %v. Using fallback.\n", err)
		return generateFallbackReport(state, finalAnswer)
	}
	if len(resp.Choices) == 0 {
		return generateFallbackReport(state, finalAnswer)
	}

	return resp.Choices[0].Message.Content
}

func generateFallbackReport(state *ReActState, finalAnswer string) string {
	var b strings.Builder
	b.WriteString("# TrustCapsule Sandbox Penetration Test Report\n\n")
	b.WriteString(fmt.Sprintf("- **Date**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- **Total Steps**: %d\n", state.Iteration))
	b.WriteString(fmt.Sprintf("- **Attack Phase**: %s\n\n", state.AttackPhase))

	b.WriteString("## Attack Steps\n\n")
	for i, s := range state.Steps {
		b.WriteString(fmt.Sprintf("### Step %d\n", i+1))
		cmd, _ := s.ActionInput["command"].(string)
		b.WriteString(fmt.Sprintf("Command: `%s`\n", cmd))
		b.WriteString(fmt.Sprintf("Result: %s\n\n", s.Observation))
	}

	b.WriteString("## Conclusion\n\n")
	b.WriteString(finalAnswer)
	b.WriteString("\n")
	return b.String()
}
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
func (a *Agent) BuildAttackReport(state *ReActState, finalAnswer string) *AttackReport {
	report := &AttackReport{
		RunID:        fmt.Sprintf("run_%s", time.Now().Format("20060102_150405")),
		Timestamp:    time.Now(),
		Query:        state.OriginalQuery,
		TotalSteps:   state.Iteration,
		Findings:     []AttackFinding{},
		DeniedPaths:  []DeniedPath{},
		PendingTasks: []string{},
	}

	// 从知识图谱提取发现
	findingID := 0
	for cmd, output := range state.KnowledgeBase {
		finding := AttackFinding{
			ID:          fmt.Sprintf("f_%d", findingID),
			Category:    categorizeCommand(cmd),
			Title:       extractTitle(cmd),
			Description: truncate(output, 300),
			Evidence:    truncate(output, 500),
			Severity:    estimateSeverity(cmd, output),
			Exploitable: estimateExploitable(cmd, output),
			ExploitHint: estimateExploitHint(cmd, output),
		}
		report.Findings = append(report.Findings, finding)
		findingID++
	}

	// 从步骤中提取Kernel和Sandbox信息
	for _, step := range state.Steps {
		cmd, _ := step.ActionInput["command"].(string)
		if strings.Contains(cmd, "uname") || strings.Contains(cmd, "/proc/version") {
			report.KernelInfo = truncate(step.Observation, 100)
		}
		if strings.Contains(cmd, "pgrep") || strings.Contains(cmd, "bwrap") {
			report.SandboxInfo = truncate(step.Observation, 200)
		}
	}

	// 失败路径
	for _, path := range state.FailedPaths {
		denied := DeniedPath{
			Command: path,
			Reason:  guessDenialReason(path, state),
			Times:   1,
		}
		// 去重统计
		found := false
		for i, d := range report.DeniedPaths {
			if d.Command == path {
				report.DeniedPaths[i].Times++
				found = true
				break
			}
		}
		if !found {
			report.DeniedPaths = append(report.DeniedPaths, denied)
		}
	}

	// 评估成功等级
	report.SuccessLevel = evaluateSuccessLevel(state, finalAnswer)
	report.Conclusion = truncate(finalAnswer, 500)
	report.NextSuggestion = generateNextSuggestion(state, finalAnswer)

	return report
}

// SaveAttackReport 存入MemSpace
func (a *Agent) SaveAttackReport(report *AttackReport) error {
	data, err := report.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize report: %w", err)
	}

	// 存入private MemSpace（如果有）
	if a.privateMemSpaceClients != nil {
		key := fmt.Sprintf("attack_report/%s", report.RunID)
		if err := a.privateMemSpaceClients.WriteMemory(data, a.AgentId); err != nil {
			fmt.Printf("[ATTACK] Failed to save report to MemSpace: %v\n", err)
		} else {
			fmt.Printf("[ATTACK] Report saved to MemSpace: %s\n", key)
		}
	}

	// 同时存本地文件（双保险）
	reportDir := a.attackConfig.ReportDir
	os.MkdirAll(reportDir, 0755)
	reportPath := fmt.Sprintf("%s/report_%s.json", reportDir, report.RunID)
	os.WriteFile(reportPath, []byte(data), 0644)

	return nil
}

// LoadPreviousReports 从MemSpace加载历史报告
func (a *Agent) LoadPreviousReports(limit int) ([]*AttackReport, error) {
	var reports []*AttackReport

	// 优先从MemSpace读
	if a.privateMemSpaceClients != nil {
		// 这里需要MemSpaceClient支持ListByPrefix
		// 如果不支持，从本地文件兜底
	}

	// 从本地文件兜底
	reportDir := a.attackConfig.ReportDir
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		return nil, nil
	}

	// 按时间倒序，取最近limit个
	jsonFiles := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") && strings.HasPrefix(entry.Name(), "report_") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}
	// 倒序
	for i, j := 0, len(jsonFiles)-1; i < j; i, j = i+1, j-1 {
		jsonFiles[i], jsonFiles[j] = jsonFiles[j], jsonFiles[i]
	}

	count := 0
	for _, filename := range jsonFiles {
		if count >= limit {
			break
		}
		path := filepath.Join(reportDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		report, err := DeserializeReport(string(data))
		if err != nil {
			continue
		}
		reports = append(reports, report)
		count++
	}

	return reports, nil
}

// FormatPreviousReportsForPrompt 格式化历史报告注入prompt
func (a *Agent) FormatPreviousReportsForPrompt() string {
	reports, err := a.LoadPreviousReports(3) // 只取最近3次
	if err != nil || len(reports) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Previous Attack Runs (Learn from these)\n")
	b.WriteString("These are results from previous attack sessions. DO NOT repeat completed work.\n\n")

	for _, r := range reports {
		b.WriteString(r.FormatForNextRun())
	}

	return b.String()
}
func (a *Agent) loadAttackHistory() string {
	if a.privateMemSpaceClients == nil {
		log.Warnf("the private memspace client is nil!")
		return ""
	}

	contents, err := a.privateMemSpaceClients.GetAllMemoryContents()
	if err != nil {
		log.Infof("[ATTACK] Failed to load history from MemSpace: %v\n", err)
		return ""
	}

	if contents == nil || len(contents.Contents) == 0 {
		log.Warnf("the content in the memspace is empty!")
		return ""
	}

	// 筛选出攻击报告（以 "ATTACK_REPORT:" 开头的记忆条目）
	var reports []string
	for _, mem := range contents.Contents {
		if strings.HasPrefix(mem, "ATTACK_REPORT:") {
			reportJSON := strings.TrimPrefix(mem, "ATTACK_REPORT:")
			report := parseStoredReport(reportJSON)
			if report != "" {
				reports = append(reports, report)
			}
		}
	}

	if len(reports) == 0 {
		return ""
	}

	// 只取最近3条
	if len(reports) > 3 {
		reports = reports[len(reports)-3:]
	}

	var b strings.Builder
	b.WriteString("## Previous Attack Runs (Learn from these)\n")
	b.WriteString("These are results from previous attack sessions. DO NOT repeat completed work.\n\n")
	for _, r := range reports {
		b.WriteString(r)
	}

	return b.String()
}

// 解析存储的报告JSON，返回格式化的prompt内容
func parseStoredReport(jsonStr string) string {
	report, err := DeserializeReport(jsonStr)
	if err != nil {
		// 如果解析失败，直接截取前500字符作为摘要
		if len(jsonStr) > 500 {
			return jsonStr[:500] + "...\n\n"
		}
		return jsonStr + "\n\n"
	}
	return report.FormatForNextRun()
}
