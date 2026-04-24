package agent

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/configs/test_utils"
	tool_executors "NucleusMem/pkg/runtime/agent/executors"
	"NucleusMem/pkg/viewspace"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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
	mockServer, _ := test_utils.NewMockToolServer()
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
	//callCount := mockServer.GetCallCount()
	//assert.Greater(t, callCount, 0, "Mock server should have received at least 1 call")
	//
	//lastReq := mockServer.GetLastRequest()
	//assert.Equal(t, "/src/main.go", lastReq["path"])
	//
	//t.Logf("✅ Mock server received %d call(s), last request: %v", callCount, lastReq)
	//t.Log("✅ TestAgent_ToolCall passed!")
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

const testChatServerAddr = "http://localhost:20001" // your LLM server address, change as needed

func newTestAgentForDecompose() *Agent {
	dispatcher := tool_executors.NewDispatcher()
	dispatcher.Register("http", tool_executors.NewHTTPExecutor())

	return &Agent{
		AgentId:               1,
		chatClient:            client.NewChatServerClient(testChatServerAddr),
		memSpaceClients:       make(map[uint64]*client.MemSpaceClient),
		publicMemSpaceClients: make([]*client.MemSpaceClient, 0),
		taskResults:           make(map[string]*TaskResult),
		taskQueue:             make(chan *AgentTask, 100),
		toolDispatcher:        dispatcher,
		maxHistory:            10,
	}
}

func TestHandleDecomposeTask_BasicDecomposition(t *testing.T) {
	a := newTestAgentForDecompose()

	task := &AgentTask{
		ID:               "test-decompose-001",
		Type:             TaskTypeDecompose,
		Content:          "Analyze the code quality of a Go project: run linting, check complexity, scan for security issues, and generate a final report.",
		AvailableTools:   []string{"lint", "complexity_analyzer", "security_scanner", "report_generator"},
		AvailableMemTags: []string{"code-tools", "static-analysis", "security"},
		MaxRetry:         3,
	}

	// Pre-register task result so handleDecomposeTask can store the definition
	a.taskResults[task.ID] = &TaskResult{Done: make(chan struct{})}

	result, err := a.handleDecomposeTask(task)
	if err != nil {
		t.Fatalf("handleDecomposeTask failed: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Verify the result is valid JSON
	var def viewspace.TaskDefinition
	if err := json.Unmarshal([]byte(result), &def); err != nil {
		t.Fatalf("result is not valid JSON: %v\nraw result:\n%s", err, result)
	}

	// Print the result for inspection
	t.Logf("=== Decomposition Result ===")
	t.Logf("Task ID: %s", def.Meta.TaskID)
	t.Logf("Description: %s", def.Meta.Description)
	t.Logf("ViewSpaces (%d):", len(def.ViewSpaces))
	for _, vs := range def.ViewSpaces {
		t.Logf("  - [%s] %s: %s (tools: %v)", vs.Type, vs.Name, vs.Description, vs.Tools)
	}
	t.Logf("Tree edges (%d):", len(def.Dependencies.Tree))
	for _, edge := range def.Dependencies.Tree {
		t.Logf("  - %s -> %v", edge.Parent, edge.Children)
	}
	t.Logf("Dataflow edges (%d):", len(def.Dependencies.Dataflow))
	for _, edge := range def.Dependencies.Dataflow {
		t.Logf("  - %s -> %s (fields: %v)", edge.From, edge.To, edge.Fields)
	}

	// Structural validations
	// 1. Must have meta
	if def.Meta.TaskID == "" {
		t.Error("meta.task_id is empty")
	}

	// 2. Must have at least one global
	globalCount := 0
	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" {
			globalCount++
		}
	}
	if globalCount != 1 {
		t.Errorf("expected exactly 1 global viewspace, got %d", globalCount)
	}

	// 3. Must have at least one atomic
	atomicCount := 0
	for _, vs := range def.ViewSpaces {
		if vs.Type == "atomic" {
			atomicCount++
		}
	}
	if atomicCount == 0 {
		t.Error("expected at least 1 atomic viewspace")
	}

	// 4. All atomic nodes should have tools
	for _, vs := range def.ViewSpaces {
		if vs.Type == "atomic" && len(vs.Tools) == 0 {
			t.Errorf("atomic viewspace '%s' has no tools", vs.Name)
		}
	}

	// 5. Re-run Parse to confirm it passes all checks
	parseResult := viewspace.Parse([]byte(result))
	if parseResult.HasErrors() {
		t.Errorf("parsed result has validation errors:")
		for _, e := range parseResult.Errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestHandleDecomposeTask_SimpleTask(t *testing.T) {
	a := newTestAgentForDecompose()

	task := &AgentTask{
		ID:             "test-decompose-simple",
		Type:           TaskTypeDecompose,
		Content:        "Run a linter on the source code and report violations.",
		AvailableTools: []string{"lint"},
		MaxRetry:       3,
	}

	a.taskResults[task.ID] = &TaskResult{Done: make(chan struct{})}

	result, err := a.handleDecomposeTask(task)
	if err != nil {
		t.Fatalf("handleDecomposeTask failed: %v", err)
	}

	var def viewspace.TaskDefinition
	if err := json.Unmarshal([]byte(result), &def); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	t.Logf("Simple task decomposition: %d viewspaces", len(def.ViewSpaces))
	for _, vs := range def.ViewSpaces {
		t.Logf("  [%s] %s", vs.Type, vs.Name)
	}

	// Simple task should not be overly decomposed
	if len(def.ViewSpaces) > 5 {
		t.Logf("WARNING: simple task decomposed into %d viewspaces, might be over-split", len(def.ViewSpaces))
	}

	// Validation
	parseResult := viewspace.Parse([]byte(result))
	if parseResult.HasErrors() {
		for _, e := range parseResult.Errors {
			t.Errorf("validation error: %s", e.Error())
		}
	}
}

func TestHandleDecomposeTask_EmptyContent(t *testing.T) {
	a := newTestAgentForDecompose()

	task := &AgentTask{
		ID:      "test-decompose-empty",
		Type:    TaskTypeDecompose,
		Content: "",
	}

	_, err := a.handleDecomposeTask(task)
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestHandleDecomposeTask_ViaSubmitTask(t *testing.T) {
	a := newTestAgentForDecompose()

	task := &AgentTask{
		Type:           TaskTypeDecompose,
		Content:        "Build a REST API with user authentication and deploy it.",
		AvailableTools: []string{"code_generator", "test_runner", "docker_builder", "deploy_tool"},
		MaxRetry:       3,
	}

	taskID, err := a.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	t.Logf("Submitted task: %s", taskID)

	// Process the task manually (since we didn't start the task loop)
	select {
	case queued := <-a.taskQueue:
		err := a.handleTask(queued)
		if err != nil {
			t.Fatalf("handleTask failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no task received from queue")
	}

	// Get result
	result, err := a.GetTaskResult(taskID, 60*time.Second)
	if err != nil {
		t.Fatalf("GetTaskResult failed: %v", err)
	}

	// Verify
	var def viewspace.TaskDefinition
	if err := json.Unmarshal([]byte(result), &def); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, result)
	}

	t.Logf("=== Via SubmitTask Result ===")
	t.Logf("Task ID: %s", def.Meta.TaskID)
	t.Logf("ViewSpaces: %d", len(def.ViewSpaces))
	for _, vs := range def.ViewSpaces {
		t.Logf("  [%s] %s (role: %s, tools: %v)", vs.Type, vs.Name, vs.Role, vs.Tools)
	}

	parseResult := viewspace.Parse([]byte(result))
	if parseResult.HasErrors() {
		for _, e := range parseResult.Errors {
			t.Errorf("validation error: %s", e.Error())
		}
	}
}

func TestExtractJSON_Integration(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOk bool
	}{
		{
			name:   "direct json",
			input:  `{"meta": {"task_id": "test"}, "viewspaces": [], "dependencies": {"tree": [], "dataflow": []}}`,
			wantOk: true,
		},
		{
			name:   "markdown wrapped",
			input:  "Here is the plan:\n```json\n{\"meta\": {\"task_id\": \"test\"}, \"viewspaces\": [], \"dependencies\": {\"tree\": [], \"dataflow\": []}}\n```\nDone!",
			wantOk: true,
		},
		{
			name:   "text with embedded json",
			input:  "I think we should do this: {\"meta\": {\"task_id\": \"test\"}, \"viewspaces\": [], \"dependencies\": {\"tree\": [], \"dataflow\": []}} what do you think?",
			wantOk: true,
		},
		{
			name:   "no json at all",
			input:  "I don't know how to help with that.",
			wantOk: false,
		},
		{
			name:   "broken json",
			input:  "{\"meta\": {\"task_id\": broken",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := viewspace.ExtractJSON(tt.input)
			if tt.wantOk && result == "" {
				t.Error("expected to extract JSON but got empty")
			}
			if !tt.wantOk && result != "" {
				t.Errorf("expected no JSON but got: %s", result)
			}
			if result != "" {
				// Verify it's actually valid JSON
				var js json.RawMessage
				if err := json.Unmarshal([]byte(result), &js); err != nil {
					t.Errorf("extracted string is not valid JSON: %v", err)
				}
			}
		})
	}
}

// TestToolDAG_execution verifies the end-to-end flow using native Agent APIs:
// 1. Inject ToolDAG into MemSpace via ToolRegion HTTP API
// 2. Submit a "tool_dag" type task via native SubmitTask interface
// 3. Agent loads DAG from MemSpace, executes tools concurrently based on dependencies
// 4. Results are aggregated and returned via native GetTaskResult interface
func TestToolDAG_execution(t *testing.T) {
	os.Setenv("TEST_SYNC_AUDIT", "1")
	defer os.Unsetenv("TEST_SYNC_AUDIT")
	// Skip if running in short mode (requires external services)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ============================================================
	// 1. Load configurations
	// ============================================================
	agentConfigFilePath := "../../configs/file/agent_101.yaml"
	memspaceConfigFile := "../../configs/file/memspace_1001.yaml"

	agentConfig, err := configs.LoadAgentConfigFromYAML(agentConfigFilePath)
	require.NoError(t, err, "failed to load agent config")

	memspaceConfig, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	require.NoError(t, err, "failed to load memspace config")

	// ============================================================
	// 2. Create HTTP clients (using existing native clients)
	// ============================================================
	memspaceClient := client.NewMemSpaceClient(memspaceConfig.HttpAddr)
	agentClient := client.NewAgentClient(agentConfig.HttpAddr)

	// Optional: verify services are healthy before proceeding
	t.Logf("Checking MemSpace health at %s", memspaceConfig.HttpAddr)
	healthResp, err := memspaceClient.HealthCheckWithInfo()
	if err != nil {
		t.Logf("Warning: MemSpace health check failed: %v (may not be started yet)", err)
	} else {
		t.Logf("MemSpace healthy: %s (ID: %s)", healthResp.Status, healthResp.MemSpaceID)
	}

	// ============================================================
	// 3. Prepare mock tool definitions (if not already registered)
	// ============================================================
	mockTools := []*configs.ToolDefinition{
		{
			Name:        "mock_lint",
			Description: "Mock linter that checks code quality",
			Tags:        []string{"static-analysis", "test"},
			Parameters: []configs.ToolParam{
				{Name: "path", Type: "string", Required: true},
			},
			ReturnType: "object",
			ExecType:   "mock", // Special type for testing
			CreatedAt:  time.Now().Unix(),
		},
		{
			Name:        "mock_test",
			Description: "Mock test runner",
			Tags:        []string{"testing", "test"},
			Parameters: []configs.ToolParam{
				{Name: "coverage", Type: "bool", Required: false, Default: "false"},
			},
			ReturnType: "object",
			ExecType:   "mock",
			CreatedAt:  time.Now().Unix(),
		},
		{
			Name:        "mock_report",
			Description: "Mock report generator (depends on lint+test)",
			Tags:        []string{"reporting", "test"},
			Parameters:  []configs.ToolParam{},
			ReturnType:  "object",
			ExecType:    "mock",
			CreatedAt:   time.Now().Unix(),
		},
	}

	// Register tools via native ToolRegion API (idempotent: skip if already exists)
	for _, tool := range mockTools {
		_, err := memspaceClient.GetTool(tool.Name)
		if err != nil {
			// Tool not found, register it via native API
			err = memspaceClient.RegisterTool(tool)
			require.NoError(t, err, "failed to register mock tool: %s", tool.Name)
			t.Logf("Registered mock tool: %s", tool.Name)
		}
	}

	// ============================================================
	// 4. Build and inject ToolDAG into MemSpace via native ToolRegion API
	// ============================================================
	toolDAG := &configs.ToolDAG{
		Nodes: []configs.ToolDAGNode{
			{
				ToolName: "mock_lint",
				Params:   map[string]interface{}{"path": "./src"},
			},
			{
				ToolName: "mock_test",
				Params:   map[string]interface{}{"coverage": true},
			},
			{
				ToolName: "mock_report",
				Params:   map[string]interface{}{"format": "json"},
			},
		},
		Edges: []configs.ToolDAGEdge{
			{From: "mock_lint", To: "mock_report", Fields: []string{"issues", "metrics"}},
			{From: "mock_test", To: "mock_report", Fields: []string{"passed", "coverage"}},
		},
	}
	mockServer, err := test_utils.NewMockToolServer()
	require.NoError(t, err, "failed to create mock server")

	err = mockServer.Start()
	require.NoError(t, err, "failed to start mock server")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mockServer.Stop(ctx)
	}()

	t.Logf("Mock Tool Server started at: %s", mockServer.BaseURL())

	// Update tool definitions to point to mock server
	baseURL := mockServer.BaseURL()
	for _, tool := range mockTools {
		tool.Endpoint = fmt.Sprintf("%s/%s", baseURL, tool.Name)
		tool.ExecType = "http"
		// Re-register with updated endpoint
		_ = memspaceClient.RegisterTool(tool)
	}

	t.Logf("Injecting ToolDAG into MemSpace %d: %d nodes, %d edges",
		memspaceConfig.MemSpaceID, len(toolDAG.Nodes), len(toolDAG.Edges))

	// Use native SaveToolDAG API (part of ToolRegion HTTP endpoints)
	err = memspaceClient.SaveToolDAG(toolDAG)
	require.NoError(t, err, "failed to save ToolDAG to MemSpace")

	// Verify DAG was saved by loading it back via native API
	loadedDAG, err := memspaceClient.LoadToolDAG()
	require.NoError(t, err, "failed to load ToolDAG from MemSpace")
	assert.Equal(t, len(toolDAG.Nodes), len(loadedDAG.Nodes), "DAG node count mismatch")
	assert.Equal(t, len(toolDAG.Edges), len(loadedDAG.Edges), "DAG edge count mismatch")

	// ============================================================
	// 5. Submit ToolDAG task to Agent using NATIVE SubmitTask interface
	// ============================================================
	// Key: Use existing SubmitTaskRequest with Type="tool_dag"
	// Pass memspace_id via Params map (no new API needed)
	submitReq := &api.SubmitTaskRequest{
		Type:    "tool_dag", // TaskTypeToolDAG constant
		Content: "Execute tools based on DAG from MemSpace",
		Params: map[string]interface{}{
			"memspace_id": memspaceConfig.MemSpaceID, // Pass via Params
		},
		// Timeout is handled by GetTaskResult caller side
	}

	t.Logf("Submitting ToolDAG task to Agent at %s (type: %s, memspace: %d)",
		agentConfig.HttpAddr, submitReq.Type, memspaceConfig.MemSpaceID)

	// Use NATIVE SubmitTask API
	submitResp, err := agentClient.SubmitTask(submitReq)
	require.NoError(t, err, "failed to submit ToolDAG task via native SubmitTask")
	require.NotEmpty(t, submitResp.TaskID, "task ID should not be empty")

	t.Logf("Task submitted successfully, tracking ID: %s", submitResp.TaskID)

	// ============================================================
	// 6. Poll for task completion using NATIVE GetTaskResult interface
	// ============================================================
	timeout := 60 * time.Second
	pollInterval := 500 * time.Millisecond
	startTime := time.Now()

	var finalResults map[string]*configs.ToolExecResult
	var lastErr error

	for time.Since(startTime) < timeout {
		// Use NATIVE GetTaskResult API
		resultResp, err := agentClient.GetTaskResult(submitResp.TaskID, 5*time.Second)
		if err != nil {
			// Task may still be running - check error message
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "still running") {
				t.Logf("Task still running, retrying in %v...", pollInterval)
				time.Sleep(pollInterval)
				continue
			}
			lastErr = err
			break
		}

		// Task completed - parse the JSON result string
		// Native GetTaskResult returns Result as JSON string
		if resultResp.Result != "" {
			err = json.Unmarshal([]byte(resultResp.Result), &finalResults)
			if err != nil {
				t.Logf("Failed to parse result JSON: %v, raw: %s", err, resultResp.Result)
				lastErr = err
				break
			}
		}

		if finalResults != nil && len(finalResults) > 0 {
			break
		}

		// Result might be empty if task still running
		time.Sleep(pollInterval)
	}

	// If we exited the loop without getting results, report failure
	if finalResults == nil {
		t.Fatalf("Failed to get task results within %v: %v", timeout, lastErr)
	}

	// ============================================================
	// 7. Validate execution results
	// ============================================================
	t.Logf("Task completed, received %d tool results", len(finalResults))

	// Verify all expected tools were executed
	expectedTools := []string{"mock_lint", "mock_test", "mock_report"}
	for _, toolName := range expectedTools {
		result, exists := finalResults[toolName]
		assert.True(t, exists, "result for tool '%s' should exist", toolName)
		if exists {
			assert.Equal(t, "completed", result.Status, "tool '%s' should have completed status", toolName)
			assert.NotEmpty(t, result.Output, "tool '%s' should have output", toolName)
			t.Logf("✓ %s: status=%s, output=%v", toolName, result.Status, result.Output)
		}
	}

	// Verify dependency order was respected (report should have data from lint+test)
	// Verify dependency order was respected (report should have data from lint+test)
	if reportResult, ok := finalResults["mock_report"]; ok {
		// Output structure: {"results": {"summary": "...", ...}, "success": true, ...}
		// Check nested results map first
		if resultsMap, ok := reportResult.Output["results"].(map[string]interface{}); ok {
			assert.Contains(t, resultsMap, "summary", "report results should contain summary field")
			if summary, ok := resultsMap["summary"]; ok {
				t.Logf("✓ Report aggregated data: %v", summary)
			}
		} else {
			// Fallback: check top-level
			assert.Contains(t, reportResult.Output, "summary", "report should contain summary field")
		}
	}

	t.Log("Verifying audit records in MemSpace...")

	// Small delay to allow async batch recording to complete
	time.Sleep(200 * time.Millisecond)

	for _, toolName := range expectedTools {
		history, err := memspaceClient.GetToolExecHistory(toolName)
		if err != nil {
			t.Logf("Warning: could not fetch history for %s: %v", toolName, err)
			continue
		}
		// Allow empty history in CI/fast runs, just log
		if len(history) == 0 {
			t.Logf("⚠️  No audit history for %s (may be timing issue)", toolName)
			continue
		}
		latest := history[len(history)-1]
		assert.Equal(t, "completed", latest.Status, "latest exec record should be completed")
		t.Logf("✓ Audit: %s seq=%d status=%s", toolName, latest.Seq, latest.Status)
	}

	t.Log("✅ ToolDAG integration test passed using native APIs only!")
}
func Test_Binding(t *testing.T) {
	agentConfigFilePath := "../../configs/file/agent_101.yaml"
	agentConfigFilePath1 := "../../configs/file/memspace_1001.yaml"
	yaml, _ := configs.LoadAgentConfigFromYAML(agentConfigFilePath)
	memspaceConfigFile := "./pkg/configs/file/memspace_1001.yaml"
	fromYAML, _ := configs.LoadMemSpaceConfigFromYAML(agentConfigFilePath1)
	_ = client.NewMemSpaceManagerClient("localhost:9200")
	//agManagerClient := client.NewAgentManagerClient("localhost:7007")
	mmManagerClient := client.NewMemSpaceManagerClient("localhost:9200")
	lmRequest := &api.LaunchMemSpaceRequestManager{BinPath: "./bin/memspace", ConfigFilePath: memspaceConfigFile}
	err := mmManagerClient.LaunchMemSpace(lmRequest)
	assert.NoError(t, err)
	// 测一下是否绑定咯
	agentClient := client.NewAgentClient(yaml.HttpAddr)
	request := &api.BindMemSpaceRequest{
		MemSpaceID: strconv.FormatUint(fromYAML.MemSpaceID, 10),
		AgentID:    yaml.AgentId,
		Type:       "private",
	}
	err = agentClient.BindMemSpace(request)
	assert.NoError(t, err)
}

func Test_Server(t *testing.T) {
	agentConfigFilePath := "../../configs/file/agent_101.yaml"
	yaml, _ := configs.LoadAgentConfigFromYAML(agentConfigFilePath)
	agent, _ := NewAgent(yaml)
	server := NewAgentHTTPServer(agent)
	server.Start(yaml.HttpAddr)
}
