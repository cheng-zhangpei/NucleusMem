package agent_manager

import (
	"context"
	"strconv"
	"testing"
	"time"

	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentManager_IntegrationWithRealMonitors(t *testing.T) {
	// 跳过测试（除非手动开启）
	// t.Skip("Skipping integration test")

	// 1. 手动创建 monitor clients（对应你启动的 3 个节点）
	monitorClients := map[uint64]*client.AgentMonitorClient{
		1: client.NewAgentMonitorClient("http://localhost:8081"),
		2: client.NewAgentMonitorClient("http://localhost:8082"),
		3: client.NewAgentMonitorClient("http://localhost:8083"),
	}

	// 2. 创建 AgentManager（绕过 YAML 加载）
	am := &AgentManager{
		agentCache:         NewAgentCache(),
		agentMonitorClient: monitorClients,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 3. 测试 GetStatus on all nodes
	for nodeID := range monitorClients {
		t.Run("GetStatus_Node_"+string(rune('0'+nodeID)), func(t *testing.T) {
			status, err := am.GetNodeStatus(ctx, nodeID)
			require.NoError(t, err, "GetStatus should succeed")
			t.Logf("Node %d status: CPU=%.2f, Mem=%.2f, Agents=%d",
				nodeID, status.CPUUsage, status.MemUsage, status.ActiveAgents)
		})
	}

	// 4. 测试 LaunchAgent on all nodes
	agentIDBase := uint64(1000)
	for nodeID := range monitorClients {
		t.Run("LaunchAgent_Node_"+string(rune('0'+nodeID)), func(t *testing.T) {
			req := &api.LaunchAgentRequestHTTP{
				AgentID:            agentIDBase + nodeID,
				Role:               "TestAgent",
				Image:              "test-image:v1",
				Env:                map[string]string{"TEST": "true"},
				MountMemSpaceNames: []string{"shared-test"},
			}

			err := am.LaunchAgentOnNode(ctx, nodeID, req)
			require.NoError(t, err, "LaunchAgent should succeed")

			// 验证 cache 是否更新（可选）
			info, ok := am.agentCache.GetAgent(strconv.FormatUint(req.AgentID, 10))
			if ok {
				assert.Equal(t, "Running", info.Status)
			}
		})
	}

	// 5. 测试 StopAgent on all nodes
	for nodeID := range monitorClients {
		t.Run("StopAgent_Node_"+string(rune('0'+nodeID)), func(t *testing.T) {
			agentID := agentIDBase + nodeID
			err := am.StopAgentOnNode(ctx, nodeID, agentID)
			require.NoError(t, err, "StopAgent should succeed")
		})
	}

	t.Log("✅ All integration tests passed!")
}
