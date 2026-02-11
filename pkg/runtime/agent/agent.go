package agent

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"fmt"
	"sync"
	"time"
)

// Agent represents an AI agent that connects to memory spaces and services
type Agent struct {
	memSpaceClients       map[uint64]*client.MemSpaceClient // memSpaceID -> HTTP client
	memSpaceManagerClient *client.MemSpaceManagerClient
	chatClient            *client.ChatServerClient
	embeddingClient       *client.EmbeddingServerClient
	mu                    sync.RWMutex
	isJob                 bool

	tempMemory []client.ChatMessage // 内存中的对话历史
	maxHistory int                  // 最大历史轮数（可配置）

}

// NewAgent creates a new Agent and initializes all service clients
func NewAgent(config *configs.AgentConfig) (*Agent, error) {
	agent := &Agent{
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

func (a *Agent) TempChat(input string) (string, error) {
	if a.isJob {
		// Job Agent 不维护记忆，直接调用 QuickChat
		resp, err := a.chatClient.QuickChat(input)
		if err != nil {
			return "", err
		}
		return resp.Response, nil
	}

	// 常驻 Agent：追加用户消息
	a.mu.Lock()
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "user",
		Content: input,
	})

	// 截断历史（保留最近 maxHistory 条，必须是偶数，user+assistant 成对）
	if len(a.tempMemory) > a.maxHistory {
		// 从前面截掉多余的（保留最近的）
		a.tempMemory = a.tempMemory[len(a.tempMemory)-a.maxHistory:]
	}
	history := make([]client.ChatMessage, len(a.tempMemory))
	copy(history, a.tempMemory)
	a.mu.Unlock()

	// 调用 LLM
	req := client.ChatCompletionRequest{
		Messages:    history,
		Temperature: 0.7,
		MaxTokens:   512,
	}
	chatResp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	response := chatResp.Choices[0].Message.Content

	// 保存助手回复到临时记忆
	a.mu.Lock()
	a.tempMemory = append(a.tempMemory, client.ChatMessage{
		Role:    "assistant",
		Content: response,
	})
	// 再次截断（因为刚加了一条）
	if len(a.tempMemory) > a.maxHistory {
		a.tempMemory = a.tempMemory[len(a.tempMemory)-a.maxHistory:]
	}
	a.mu.Unlock()

	return response, nil
}

// Chat is the main chat interface
func (a *Agent) Chat(input string) (string, error) {
	return "nil", nil
}

// Close is a no-op for HTTP clients (no persistent connections)
func (a *Agent) Close() {
	// HTTP clients don't need explicit close
}
