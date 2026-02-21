package memspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

// test the conn with the kv and the unit test of the memspace
func TestMemSpace_Basic(t *testing.T) {
	filepath := "../../configs/file/memspace_1001.yaml"
	config, err := configs.LoadMemSpaceConfigFromYAML(filepath)
	assert.NoError(t, err)
	memspace, err := NewMemSpace(config)
	assert.NoError(t, err)
	httpServer := NewMemSpaceHTTPServer(memspace)
	err = httpServer.Start()
}

func TestMemSpaceClient_Basic(t *testing.T) {
	// 假设 MemSpace 已在本地运行
	baseURL := "http://localhost:8081"
	client := client.NewMemSpaceClient(baseURL)
	// 1. Health check
	t.Log("→ Health check")
	if _, err := client.HealthCheckWithInfo(); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	// 2. Register agent
	t.Log("→ Register agent")
	agentID := uint64(101)
	if err := client.RegisterAgent(agentID, "localhost:9001", "test-agent"); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	// 3. Write memory
	t.Log("→ Write memory")
	if err := client.WriteMemory("The sky is blue.", agentID); err != nil {
		t.Fatalf("WriteMemory failed: %v", err)
	}
	// 4. Get memory context
	t.Log("→ Get memory context")
	summary, memories, err := client.GetMemoryContext(time.Now().Unix(), "sky", 3)
	if err != nil {
		t.Fatalf("GetMemoryContext failed: %v", err)
	}
	t.Logf("Summary: %s", summary)
	t.Logf("Memories: %v", memories)

	// 5. List agents
	t.Log("→ List agents")
	agents, err := client.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	t.Logf("Registered agents: %d", len(agents))
}
func TestMemSpaceClient_SummaryCompression(t *testing.T) {
	baseURL := "http://localhost:8081"
	client := client.NewMemSpaceClient(baseURL)

	agentID := uint64(201)

	// 1. 注册 agent
	t.Log("→ Register agent for summary test")
	if err := client.RegisterAgent(agentID, "localhost:9003", "summary-tester"); err != nil {
		t.Fatalf("Register agent failed: %v", err)
	}

	// 2. 写入一批记忆（数量 > summary_threshold，假设阈值是 10）
	t.Log("→ Writing memories to trigger summarization...")
	memories := []string{
		"User completed onboarding step 1.",
		"User set profile picture.",
		"User connected GitHub account.",
		"User joined workspace 'NucleusMem'.",
		"User created first memory entry.",
		"User invited teammate Alice.",
		"User configured notification settings.",
		"User explored documentation page.",
		"User ran first integration test.",
		"User reported a bug in TinyKV.",
		"User fixed protobuf dependency issue.", // 第11条，应触发摘要
	}
	for i, mem := range memories {
		if err := client.WriteMemory(mem, agentID); err != nil {
			t.Fatalf("WriteMemory #%d failed: %v", i+1, err)
		}
		time.Sleep(50 * time.Millisecond) // 避免写入过快
	}

	// 3. 等待后台摘要生成（根据你的 worker ticker 间隔）
	t.Log("→ Waiting for background summarization (5 seconds)...")
	time.Sleep(6 * time.Second) // 确保超过 ticker 间隔（你设的是 5s）
	// 4. 获取上下文
	t.Log("→ Fetching memory context after summarization")
	summary, recentMems, err := client.GetMemoryContext(time.Now().Unix(), "user", 5)
	if err != nil {
		t.Fatalf("GetMemoryContext failed: %v", err)
	}
	// 5. 验证结果
	t.Logf("Summary:\n%s", summary)
	t.Logf("Recent memories (%d):\n%v", len(recentMems), recentMems)
	// 检查摘要是否生成（非空且不是简单拼接）
	if summary == "" {
		t.Error("Expected non-empty summary, got empty")
	} else if strings.Contains(summary, "- User") && len(summary) > 200 {
		t.Log("⚠️ Warning: Summary looks like raw concatenation (AI may not be working)")
	}

	// 检查最近记忆是否返回
	if len(recentMems) == 0 {
		t.Error("Expected recent memories to be returned")
	}

	t.Log("✅ Summary compression test passed!")
}
func TestMemSpaceClient_Communication(t *testing.T) {
	baseURL := "http://localhost:8081"
	//monitorURL := "http://localhost:9091"
	memspaceClient := client.NewMemSpaceClient(baseURL)
	//agentMonitorConfig, _ := configs.LoadAgentMonitorConfigFromYAML(monitorURL)
	//monitor := agent_monitor.NewAgentMonitor(agentMonitorConfig)
	//ctx := context.Background()
	agentConfigFile1 := "../../configs/file/agent_101.yaml"
	agentConfigFile2 := "../../configs/file/agent_102.yaml"
	//monitorClient := client.NewAgentMonitorClient(monitorURL)
	agent1, _ := configs.LoadAgentConfigFromYAML(agentConfigFile1)
	agent2, _ := configs.LoadAgentConfigFromYAML(agentConfigFile2)
	t.Log("→ Registering Agent 101")
	if err := memspaceClient.RegisterAgent(agent1.AgentId, "localhost:9001", "worker"); err != nil {
		t.Fatalf("RegisterAgent 101 failed: %v", err)
	}

	t.Log("→ Registering Agent 102")
	if err := memspaceClient.RegisterAgent(agent2.AgentId, "localhost:9002", "worker"); err != nil {
		t.Fatalf("RegisterAgent 102 failed: %v", err)
	}
	agentClient1 := client.NewAgentClient("localhost:9001")
	agentClient2 := client.NewAgentClient("localhost:9002")
	err := agentClient1.BindMemSpace(&api.BindMemSpaceRequest{"1001", "Public", baseURL})
	if err != nil {
		t.Fatalf("BindMemSpace failed: %v", err)
	}
	err = agentClient2.BindMemSpace(&api.BindMemSpaceRequest{"1001", "Public", baseURL})
	if err != nil {
		t.Fatalf("BindMemSpace failed: %v", err)
	}
	// agent and memspace have connected with each other
	t.Log("→ Listing agents")
	agents, err := memspaceClient.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	found101, found102 := false, false
	for _, a := range agents {
		if a.AgentID == "101" {
			found101 = true
		}
		if a.AgentID == "102" {
			found102 = true
		}
	}
	if !found101 || !found102 {
		t.Errorf("Expected both agents 101 and 102 to be registered, got: %+v", agents)
	}

	// 3. Agent 101 发送消息给 Agent 102
	t.Log("→ Agent 101 sends message to Agent 102")
	messageKey := "memory/101/5" // 假设这是某条记忆的 key
	refType := "memory"

	if err := memspaceClient.SendMessage(agent1.AgentId, agent2.AgentId, messageKey, refType); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	t.Log("✅ Communication test passed! Message sent successfully.")
}
