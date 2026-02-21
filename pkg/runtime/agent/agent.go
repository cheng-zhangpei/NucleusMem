package agent

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/configs/prompt"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
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
	mu                     sync.RWMutex
	isJob                  bool
	privateMemSpaceClients *client.MemSpaceClient
	publicMemSpaceClients  []*client.MemSpaceClient
	tempMemory             []client.ChatMessage // 内存中的对话历史
	maxHistory             int                  // 最大历史轮数（可配置）
	boundMonitorID         uint64
	boundMu                sync.RWMutex

	taskQueue chan *AgentTask
}

// NewAgent creates a new Agent and initializes all service clients
func NewAgent(config *configs.AgentConfig) (*Agent, error) {
	agent := &Agent{
		AgentId:               config.AgentId,
		memSpaceClients:       make(map[uint64]*client.MemSpaceClient),
		memSpaceManagerClient: client.NewMemSpaceManagerClient(config.MemSpaceManagerAddr),
		chatClient:            client.NewChatServerClient(config.ChatServerAddr),
		embeddingClient:       client.NewEmbeddingServerClient(config.VectorServerAddr),
		isJob:                 config.IsJob,
		publicMemSpaceClients: make([]*client.MemSpaceClient, 0),
	}
	//agent.bindingMemspace()
	ctx, _ := context.WithCancel(context.Background())
	agent.taskQueue = make(chan *AgentTask, 1000)
	agent.maxHistory = 10
	// Connect to private MemSpace (required)
	if !agent.isJob {
		if config.PrivateMemSpaceInfo != nil {
			err := agent.connectToMemSpace(config.PrivateMemSpaceInfo)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to private memspace: %w", err)
			}
		}
		// Connect to public MemSpaces (optional)
		for _, info := range config.PublicMemSpaceInfo {
			err := agent.connectToMemSpace(info)
			if err != nil {
				// Log but don't fail — public spaces are optional
				fmt.Printf("Warning: failed to connect to public memspace %d: %v\n", info.MemSpaceId, err)
			}
		}
	} else {
		// todo(cheng).. if the agent is job what should I do?
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

	// Step 3: Build prompt
	sysMsg := "You are an intelligent agent with access to shared memory and conversation history. Use both to answer the user's query."
	promptObj := prompt.NewChatPrompt(sysMsg, combinedSummary, input, tempHistory)
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
	switch task.Type {
	case TaskTypeComm:
		msg, err := a.handleCommTask(task)
		if err != nil {
			return err
		}
		// Enqueue as Chat task
		chatTask := &AgentTask{
			Type:      TaskTypeChat,
			Content:   msg,
			Timestamp: time.Now().Unix(),
		}
		select {
		case a.taskQueue <- chatTask:
		default:
			log.Warnf("Task queue full, dropping chat task")
		}
		return nil

	case TaskTypeTempChat:
		_, err := a.TempChat(task.Content)
		return err

	case TaskTypeChat:
		_, err := a.Chat(task.Content)
		return err

	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

func (a *Agent) SubmitTask(task *AgentTask) error {
	if task.Timestamp == 0 {
		task.Timestamp = time.Now().Unix()
	}
	a.taskQueue <- task
	return nil
}
func (a *Agent) handleChatTask(task *AgentTask) error {
	content := task.Content
	if content == "" {
		return fmt.Errorf("chat task requires content")
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

	sysMsg := "You are an intelligent agent with access to shared memory and conversation history. Use both to answer the user's query."
	promptObj := prompt.NewChatPrompt(
		sysMsg,
		combinedSummary,
		content,
		tempHistory,
	)

	promptStr, err := promptObj.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode prompt: %w", err)
	}

	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: promptStr}},
		Temperature: 0.7,
		MaxTokens:   512,
	}

	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from LLM")
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
	// 可加长度限制：if len(a.tempMemory) > a.maxHistory { ... }
	a.mu.Unlock()

	log.Infof("Agent %d processed chat task → %s", a.AgentId, response)
	return nil
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

	if err != nil {
		return content, err
	}
	return content, nil
}

// bindingMemSpace binds the agent to a MemSpace (local only, no manager call)
func (a *Agent) bindingMemSpace(memSpaceConfig *configs.MemSpaceConfig) error {
	if memSpaceConfig == nil {
		return fmt.Errorf("memspace config is nil")
	}

	// Create client
	baseURL := memSpaceConfig.HttpAddr
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	client := client.NewMemSpaceClient(baseURL)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Store in main map
	a.memSpaceClients[memSpaceConfig.MemSpaceID] = client

	// Update private/public references
	if memSpaceConfig.Type == "private" {
		a.privateMemSpaceClients = client
	} else if memSpaceConfig.Type == "public" {
		// Avoid duplicates
		exists := false
		for _, c := range a.publicMemSpaceClients {
			if c != nil && c.BaseURL == client.BaseURL {
				exists = true
				break
			}
		}
		if !exists {
			a.publicMemSpaceClients = append(a.publicMemSpaceClients, client)
		}
	}

	log.Infof("Agent %d bound to MemSpace %d (%s)", a.AgentId, memSpaceConfig.MemSpaceID, memSpaceConfig.Type)
	return nil
}

// unBindingMemSpace unbinds the agent from a MemSpace (local only)
func (a *Agent) unBindingMemSpace(memID uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	memspaceClient, exists := a.memSpaceClients[memID]
	if !exists {
		return fmt.Errorf("memspace %d not bound", memID)
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

	log.Infof("Agent %d unbound from MemSpace %d", a.AgentId, memID)
	return nil
}
