package agent

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/configs/test_utils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentHTTPServer_TempChat(t *testing.T) {
	// Create a mock agent config (minimal setup)
	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001", // Mock LLM server address
		IsJob:          false,
	}

	// Create agent instance
	agentInstance, err := NewAgent(config)
	assert.NoError(t, err)

	// Create HTTP server
	server := NewAgentHTTPServer(agentInstance)

	// Test cases
	tests := []struct {
		name          string
		inputMessage  string
		expectSuccess bool
	}{
		{
			name:          "valid message",
			inputMessage:  "Hello, how are you?",
			expectSuccess: true,
		},
		{
			name:          "empty message",
			inputMessage:  "",
			expectSuccess: true, // LLM should handle empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			reqBody := map[string]string{"message": tt.inputMessage}
			jsonBody, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/v1/chat/temp", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Record response
			w := httptest.NewRecorder()
			server.handleTempChat(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			var resp struct {
				Success      bool   `json:"success"`
				Response     string `json:"response"`
				ErrorMessage string `json:"error_message"`
			}
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)

			if tt.expectSuccess {
				assert.True(t, resp.Success)
				assert.NotEmpty(t, resp.Response)
				assert.Empty(t, resp.ErrorMessage)
			} else {
				assert.False(t, resp.Success)
				assert.NotEmpty(t, resp.ErrorMessage)
			}
		})
	}
}

// Test with Job Agent type
func TestAgentHTTPServer_TempChat_JobAgent(t *testing.T) {
	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001",
		IsJob:          true, // Job agent
	}

	agentInstance, err := NewAgent(config)
	assert.NoError(t, err)

	server := NewAgentHTTPServer(agentInstance)

	// Send request
	reqBody := map[string]string{"message": "What is AI?"}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/chat/temp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.handleTempChat(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Response string `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Response)
}

// TestAgent_TempChat_ContextContinuity verifies that the agent maintains conversation context across multiple TempChat calls
func TestAgent_TempChat_ContextContinuity(t *testing.T) {
	// Skip if you don't have a real LLM backend (or use a mock that simulates memory)
	// t.Skip("Skipping due to external LLM dependency")

	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001", // Your local chat server
		IsJob:          false,
	}

	agent, err := NewAgent(config)
	require.NoError(t, err)

	// Round 1: User introduces themselves
	resp1, err := agent.TempChat("你好！")
	require.NoError(t, err)
	assert.NotEmpty(t, resp1)

	// Round 2: User says their name
	resp2, err := agent.TempChat("我叫小明。")
	require.NoError(t, err)
	assert.NotEmpty(t, resp2)

	// Round 3: Ask the agent to recall the name
	resp3, err := agent.TempChat("我刚才说我的名字是什么？")
	require.NoError(t, err)
	assert.NotEmpty(t, resp3)

	// Verify context awareness (case-insensitive)
	assert.Contains(t, strings.ToLower(resp3), "小明",
		"LLM response should reference the user's name from history")
}

func TestAgent_ToolCall(t *testing.T) {
	agentConfigFilePath := "../../configs/file/agent_101.yaml"
	memspaceConfigFile := "../../configs/file/memspace_1001.yaml"
	config, err := configs.LoadAgentConfigFromYAML(agentConfigFilePath)
	assert.NoError(t, err)
	memspaceConfig, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	agent, err := NewAgent(config)
	assert.NoError(t, err)
	spaceClient := client.NewMemSpaceClient(memspaceConfig.HttpAddr)
	err = agent.bindingMemSpace(memspaceConfig)
	assert.NoError(t, err)
	mockTool := &configs.ToolDefinition{
		Name:        "mock_lint",
		Description: "A mock linter tool for testing",
		Endpoint:    "http://mock.tools.internal/lint",
		Parameters: []configs.ToolParam{
			{
				Name:     "path",
				Type:     "string",
				Required: true,
				Default:  "",
			},
			{
				Name:     "config",
				Type:     "string",
				Required: false,
				Default:  ".eslintrc",
			},
		},
		ReturnType: "object",
		Tags:       []string{"static-analysis", "code-tools", "test"},
		CreatedAt:  time.Now().Unix(),
	}
	err = spaceClient.RegisterTool(mockTool)
	assert.NoError(t, err)

	agentTask := &AgentTask{
		Content: "I need to execute the tool you can see in your memory," +
			"I just register one tool for you,you can see that and response " +
			"according to the formation define in prompt,if you see the tool," +
			"just let me kown(set the action to tool_call),and obey the formation I give.mock you gonna call the tool",
		Timestamp: time.Now().Unix(),
		Type:      TaskTypeChat,
	}
	err = agent.handleTask(agentTask)
	assert.NoError(t, err)

}

func TestAgent_ToolCallMock(t *testing.T) {
	// ============================================================
	// 1. 启动 Mock Tool Server（测试内嵌）
	// ============================================================
	mockServer := test_utils.NewMockToolServer()
	go mockServer.Start()

	defer mockServer.Stop(context.Background())

	t.Logf("Mock Tool Server started at %s", mockServer.BaseURL())

	// ============================================================
	// 2. 加载配置
	// ============================================================
	agentConfigFilePath := "../../configs/file/agent_101.yaml"
	memspaceConfigFile := "../../configs/file/memspace_1001.yaml"

	config, err := configs.LoadAgentConfigFromYAML(agentConfigFilePath)
	assert.NoError(t, err)

	memspaceConfig, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)

	// ============================================================
	// 3. 创建 Agent
	// ============================================================
	agent, err := NewAgent(config)
	assert.NoError(t, err)

	// ============================================================
	// 4. 绑定 MemSpace
	// ============================================================
	err = agent.bindingMemSpace(memspaceConfig)
	assert.NoError(t, err)
	t.Logf("Agent %d bound to MemSpace %d", agent.AgentId, memspaceConfig.MemSpaceID)

	// ============================================================
	// 5. 注册 Mock Tool（使用动态端口）
	// ============================================================
	memspaceClient := client.NewMemSpaceClient("http://" + memspaceConfig.HttpAddr)

	mockTool := test_utils.MockLintToolDefinition(mockServer.BaseURL() + "/lint")
	err = memspaceClient.RegisterTool(mockTool)
	assert.NoError(t, err)
	t.Logf("Mock tool 'mock_lint' registered at %s", mockTool.Endpoint)

	// ============================================================
	// 6. 验证工具已注册
	// ============================================================
	tool, err := memspaceClient.GetTool("mock_lint")
	assert.NoError(t, err)
	assert.Equal(t, "mock_lint", tool.Name)
	t.Logf("Tool verified: %s", tool.Description)

	// ============================================================
	// 7. 创建 Chat 任务（触发 Tool Call）
	// ============================================================
	agentTask := &AgentTask{
		ID:        fmt.Sprintf("test_tool_call_%d", time.Now().UnixNano()),
		Content:   "I need to execute the tool you can see in your memory. I just registered one tool for you called 'mock_lint'. Please use it to lint the code at /src/main.go",
		Timestamp: time.Now().Unix(),
		Type:      TaskTypeChat,
	}

	// ============================================================
	// 8. 提交任务并等待完成
	// ============================================================
	response, err := agent.SubmitTask(agentTask)
	assert.NoError(t, err)
	log.Infof("the response is %s", response)
	result, err := agent.GetTaskResult(agentTask.ID, 60*time.Second)
	assert.NoError(t, err, "Task should complete within 60 seconds")

	// ============================================================
	// 9. 验证结果
	// ============================================================
	t.Logf("📝 Task Result: %s", result)
	assert.NotEmpty(t, result)

	// 验证包含工具调用信息
	assert.True(t,
		containsAny(result, []string{"mock_lint", "lint", "tool", "violations"}),
		"Result should mention the tool or its output")

	// ============================================================
	// 10. 验证 Mock Server 收到了调用
	// ============================================================
	callCount := mockServer.GetCallCount()
	assert.Greater(t, callCount, 0, "Mock server should have received at least 1 call")

	lastReq := mockServer.GetLastRequest()
	assert.Equal(t, "/src/main.go", lastReq["path"])

	t.Logf("✅ Mock server received %d call(s), last request: %v", callCount, lastReq)
	t.Log("✅ TestAgent_ToolCall passed!")
}

// 辅助函数
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
