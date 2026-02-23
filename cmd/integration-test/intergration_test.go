package test

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/agent_manager"
	"NucleusMem/pkg/controller/memspace_manager"
	"NucleusMem/pkg/controller/monitor/agent_monitor"
	"NucleusMem/pkg/controller/monitor/memspace_monitor"
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const (
	// Config files
	memspaceManagerConfigFile = "../../pkg/configs/file/memspace_manager.yaml"
	memspaceMonitorConfigFile = "../../pkg/configs/file/memspace_monitor_1.yaml"
	agentManagerConfigFile    = "../../pkg/configs/file/agent_manager.yaml"
	agentMonitorConfigFile    = "../../pkg/configs/file/agent_monitor_1.yaml"

	// Resource configs
	memspaceConfigFile1001 = "../../pkg/configs/file/memspace_1001.yaml"
	memspaceConfigFile1002 = "../../pkg/configs/file/memspace_1002.yaml"
	agentConfigFile101     = "../../pkg/configs/file/agent_101.yaml"
	agentConfigFile102     = "../../pkg/configs/file/agent_102.yaml"

	// Binary paths
	agentBinPath    = "../../bin/agent"
	memspaceBinPath = "../../bin/memspace"
)

func TestStartManagerAndMonitor(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============================================
	// 1. Start MemSpaceManager + HTTP Server
	// ============================================
	t.Log("→ Starting MemSpaceManager...")
	memspaceManagerConfig, err := configs.LoadMemSpaceManagerConfigFromYAML(memspaceManagerConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, memspaceManagerConfig)

	memspaceManager, err := memspace_manager.NewMemSpaceManager(memspaceManagerConfig)
	assert.Nil(t, err)
	assert.NotNil(t, memspaceManager)

	memspaceManagerServer := memspace_manager.NewMemSpaceManagerHTTPServer(memspaceManager)
	go func() {
		err := memspaceManagerServer.Start(memspaceManagerConfig.ListenAddr)
		if err != nil && err != http.ErrServerClosed {
			t.Logf("MemSpaceManager server error: %v", err)
		}
	}()

	// Wait for MemSpaceManager to be ready
	// ============================================
	// 2. Start MemSpaceMonitor + HTTP Server
	// ============================================
	t.Log("→ Starting MemSpaceMonitor...")
	memspaceMonitorConfig, err := configs.LoadMemSpaceMonitorConfigFromYAML(memspaceMonitorConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, memspaceMonitorConfig)

	memspaceMonitor := memspace_monitor.NewMemSpaceMonitor(memspaceMonitorConfig)
	assert.NotNil(t, memspaceMonitor)

	memspaceMonitorServer := memspace_monitor.NewMemSpaceMonitorHTTPServer(memspaceMonitor)
	go func() {
		err := memspaceMonitorServer.Start(memspaceMonitorConfig.MonitorUrl)
		if err != nil && err != http.ErrServerClosed {
			t.Logf("MemSpaceMonitor server error: %v", err)
		}
	}()

	// Wait for MemSpaceMonitor to be ready
	// ============================================
	// 3. Start AgentManager + HTTP Server
	// ============================================
	t.Log("→ Starting AgentManager...")
	agentManagerConfig, err := configs.LoadAgentManagerConfigFromYAML(agentManagerConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, agentManagerConfig)

	agentManager, err := agent_manager.NewAgentManager(agentManagerConfig)
	assert.Nil(t, err)
	assert.NotNil(t, agentManager)
	agentManagerServer := agent_manager.NewAgentManagerHTTPServer(agentManager)
	go func() {
		err := agentManagerServer.Start(agentManagerConfig.HttpAddr)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("AgentManager server error: %v", err)
		}
	}()

	// ============================================
	// 4. Start AgentMonitor + HTTP Server
	// ============================================
	t.Log("→ Starting AgentMonitor...")
	agentMonitorConfig, err := configs.LoadAgentMonitorConfigFromYAML(agentMonitorConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, agentMonitorConfig)

	agentMonitor := agent_monitor.NewAgentMonitor(agentMonitorConfig)
	assert.NotNil(t, agentMonitor)

	agentMonitorServer := agent_monitor.NewAgentMonitorHTTPServer(agentMonitor)
	go func() {
		err := agentMonitorServer.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Logf("AgentMonitor server error: %v", err)
		}
	}()

	// Wait for AgentMonitor to be ready
	// ============================================
	// 5. Verify Manager-Monitor Connection
	// ============================================
	t.Log("→ Verifying Manager-Monitor connection...")
	time.Sleep(2 * time.Second) // Wait for health check sync
	// ============================================
	// 6. Display Cache Status
	// ============================================
	t.Log("→ Displaying cache status...")
	memspaceManager.DisplayMemSpaces()
	agentManager.DisplayCache()
	t.Log("All managers and monitors started successfully!")
	select {}
}

// trimHTTPPrefix removes http:// or https:// prefix
func trimHTTPPrefix(addr string) string {
	if len(addr) > 7 && addr[:7] == "http://" {
		return addr[7:]
	}
	if len(addr) > 8 && addr[:8] == "https://" {
		return addr[8:]
	}
	return addr
}

func TestLaunchAgent(t *testing.T) {
	managerConfig, err := configs.LoadAgentManagerConfigFromYAML(agentManagerConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, managerConfig)

	managerClient := client.NewAgentManagerClient(managerConfig.HttpAddr)
	Lreq1 := &api.LaunchAgentRequestHTTP{
		BinPath:        agentBinPath,
		ConfigFilePath: agentConfigFile101,
	}
	resp, err := managerClient.LaunchAgent(Lreq1)
	assert.Nil(t, err)
	assert.NotNil(t, resp)
	Lreq2 := &api.LaunchAgentRequestHTTP{
		BinPath:        agentBinPath,
		ConfigFilePath: agentConfigFile102,
	}
	resp2, err := managerClient.LaunchAgent(Lreq2)
	assert.Nil(t, err)
	assert.NotNil(t, resp2)
}

func TestLaunchMemspace(t *testing.T) {
	managerConfig, err := configs.LoadMemSpaceManagerConfigFromYAML(memspaceManagerConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, managerConfig)
	managerClient := client.NewMemSpaceManagerClient(managerConfig.ListenAddr)
	Lreq1 := &api.LaunchMemSpaceRequestManager{
		BinPath:        memspaceBinPath,
		ConfigFilePath: memspaceConfigFile1001,
	}
	err = managerClient.LaunchMemSpace(Lreq1)
	assert.Nil(t, err)
	Lreq2 := &api.LaunchMemSpaceRequestManager{
		BinPath:        memspaceBinPath,
		ConfigFilePath: memspaceConfigFile1002,
	}
	err = managerClient.LaunchMemSpace(Lreq2)
	assert.Nil(t, err)

}

func TestAgentBinding(t *testing.T) {
	// memspaceConfig
	configMemspace_1001, _ := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile1001)
	configMemspace_1002, _ := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile1002)

	managerConfig, err := configs.LoadMemSpaceManagerConfigFromYAML(memspaceManagerConfigFile)
	assert.Nil(t, err)
	assert.NotNil(t, managerConfig)
	agentConfig101, _ := configs.LoadAgentConfigFromYAML(agentConfigFile101)
	agentConfig102, _ := configs.LoadAgentConfigFromYAML(agentConfigFile102)
	agentClient101 := client.NewAgentClient(agentConfig101.HttpAddr)
	agentClient102 := client.NewAgentClient(agentConfig102.HttpAddr)

	req1 := &api.BindMemSpaceRequest{
		strconv.FormatUint(configMemspace_1001.MemSpaceID, 10),
		agentConfig101.AgentId,
		configMemspace_1001.Type,
		configMemspace_1001.HttpAddr,
	}
	err = agentClient101.BindMemSpace(req1)
	assert.Nil(t, err)
	req2 := &api.BindMemSpaceRequest{
		strconv.FormatUint(configMemspace_1002.MemSpaceID, 10),
		agentConfig102.AgentId,
		configMemspace_1002.Type,
		configMemspace_1002.HttpAddr,
	}
	err = agentClient102.BindMemSpace(req2)
	assert.Nil(t, err)
	req3 := &api.BindMemSpaceRequest{
		strconv.FormatUint(configMemspace_1001.MemSpaceID, 10),
		agentConfig102.AgentId,
		configMemspace_1001.Type,
		configMemspace_1001.HttpAddr,
	}
	err = agentClient102.BindMemSpace(req3)
	assert.Nil(t, err)
	// this is the private memspace
}

func TestChatAndComm(t *testing.T) {
	// 1. 加载配置
	agentConfig101, err := configs.LoadAgentConfigFromYAML(agentConfigFile101)
	assert.Nil(t, err)
	agentConfig102, err := configs.LoadAgentConfigFromYAML(agentConfigFile102)
	assert.Nil(t, err)
	// 2. 创建客户端
	agentClient101 := client.NewAgentClient(agentConfig101.HttpAddr)
	agentClient102 := client.NewAgentClient(agentConfig102.HttpAddr)
	// 3. 初始化结果存储
	results := make([]string, 2)
	topic := "hello, Both of you gonna chat the topic: the future of the AIOps, I am agent too."

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("aiops_%d", i) // ✅ 修复 key 生成

		// --- 第一轮：101 发给 102 ---
		var content string
		if i == 0 {
			content = topic
		} else {
			content = results[1] // 使用 102 上一轮的回复
		}

		// ✅ 修复：Communicate 应返回 (result, error)
		resp102, err := agentClient101.Communicate(agentConfig102.AgentId, key, content)
		if err != nil {
			t.Fatalf("Round %d: Agent 101 -> 102 failed: %v", i, err)
		}
		results[0] = resp102
		t.Logf("Round %d: 102 replied: %s", i, results[0])

		// --- 第二轮：102 发给 101 ---
		key = fmt.Sprintf("aiops_%d", i) // ✅ 修复 key 生成

		resp101, err := agentClient102.Communicate(agentConfig101.AgentId, key, results[0])
		if err != nil {
			t.Fatalf("Round %d: Agent 102 -> 101 failed: %v", i, err)
		}
		results[1] = resp101
		t.Logf("Round %d: 101 replied: %s", i, results[1])
	}

	t.Log("ChatAndComm test passed!")
}
