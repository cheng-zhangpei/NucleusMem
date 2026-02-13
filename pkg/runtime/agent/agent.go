package agent

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"sync"
)

// Agent represents an AI agent that connects to memory spaces and services
type Agent struct {
	AgentId               uint64
	memSpaceClients       map[uint64]*client.MemSpaceClient // memSpaceID -> HTTP client
	memSpaceManagerClient *client.MemSpaceManagerClient
	chatClient            *client.ChatServerClient
	embeddingClient       *client.EmbeddingServerClient
	mu                    sync.RWMutex
	isJob                 bool

	tempMemory     []client.ChatMessage // 内存中的对话历史
	maxHistory     int                  // 最大历史轮数（可配置）
	boundMonitorID uint64
	boundMu        sync.RWMutex
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
	}

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

	// Initialize with system message if empty
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

	// Safe truncation helper
	truncateHistory := func() {
		if a.maxHistory <= 0 {
			return
		}
		// Ensure we keep at least the system message
		minLen := 1
		if len(a.tempMemory) <= minLen {
			return
		}
		// Calculate how many messages to keep (including system)
		keepCount := a.maxHistory
		if keepCount < minLen {
			keepCount = minLen
		}
		// If we have more messages than we want to keep
		if len(a.tempMemory) > keepCount {
			// Keep system message + most recent (keepCount - 1) messages
			newHistory := make([]client.ChatMessage, keepCount)
			newHistory[0] = a.tempMemory[0] // System message
			copy(newHistory[1:], a.tempMemory[len(a.tempMemory)-(keepCount-1):])
			a.tempMemory = newHistory
		}
	}

	// Truncate before sending to LLM
	truncateHistory()

	// Prepare request
	req := client.ChatCompletionRequest{
		Messages:    a.tempMemory,
		Temperature: 0.7,
		MaxTokens:   512,
	}

	// Make LLM call (unlock during network call)
	a.mu.Unlock()
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

	// Final truncation after adding response
	truncateHistory()

	return response, nil
}

// Chat is the main chat interface
func (a *Agent) Chat(input string) (string, error) {
	// todo(cheng) after the memspace finished
	return "nil", nil
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
