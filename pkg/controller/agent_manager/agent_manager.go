package agent_manager

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	_ "github.com/pingcap-incubator/tinykv/log"
)

type AgentManager struct {
	agentCache *AgentCache // maintain the information of every agent which enable the manager to connect every agent
	//monitorCache *Mo
	// todo (cheng proto define) monitorClient\agentClient
	agentMonitorClient map[uint64]*client.AgentMonitorClient
}

func NewAgentManager(agentManagerConfig *configs.AgentManagerConfig) (*AgentManager, error) {
	// 1, build monitor conn
	am := &AgentManager{nil, nil}
	ctx := context.Background()
	monitorClients := make(map[uint64]*client.AgentMonitorClient)
	for id, monitorConfig := range agentManagerConfig.MonitorURLs {
		mc := client.NewAgentMonitorClient(monitorConfig)
		//id, _ := strconv.ParseUint(id, 10, 64)
		monitorClients[id] = mc
	}
	am.agentMonitorClient = monitorClients
	agentCache := NewAgentCache()
	// 2. 从每个 Monitor 拉取当前状态，填充 cache
	for id, _ := range monitorClients {
		nodeStatus, err := am.GetNodeStatus(ctx, id)
		if err != nil {
			return nil, err
		}
		// 将每个 running agent 加入 cache
		for _, agentStatus := range nodeStatus.Agents {
			agentCache.UpdateAgent(&AgentInfo{
				AgentID:  agentStatus.AgentID,
				Status:   agentStatus.Phase,
				NodeAddr: nodeStatus.NodeID, // 或存储 nodeID
				// MemSpaceID 暂时无法从 status 获取，后续可扩展
			})
		}
	}
	return &AgentManager{agentCache, monitorClients}, nil
}

// BingMemSpace binding the memspace and agent
func (*AgentManager) BingMemSpace() error {
	return nil
}

// GenerateAgentService create agents resource in a cloud-native environment (distributed env)
func (*AgentManager) GenerateAgentService(config configs.AgentConfig) error {
	return nil
}

func (*AgentManager) destroyAgentService(config configs.AgentConfig) error {
	return nil
}

// LoadAgent load the agentCache for each monitors
func (*AgentManager) LoadAgent() {

}
func (am *AgentManager) LaunchAgentOnNode(ctx context.Context, nodeID uint64, req *api.LaunchAgentRequestHTTP) error {
	monitorClient, ok := am.agentMonitorClient[nodeID]
	if !ok {
		return fmt.Errorf("no client for node %d", nodeID)
	}
	resp, err := monitorClient.LaunchAgent(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("launch failed: %s", resp.ErrorMessage)
	}
	log.Infof("[agent_manager]launch agent on node %d", nodeID)
	return nil
}

func (am *AgentManager) StopAgentOnNode(ctx context.Context, nodeID uint64, agentID uint64) error {
	monitorClient, ok := am.agentMonitorClient[nodeID]
	if !ok {
		return fmt.Errorf("no client for node %d", nodeID)
	}
	resp, err := monitorClient.StopAgent(ctx, agentID)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("stop failed: %s", resp.ErrorMessage)
	}
	log.Infof("[agent_manager] stop agent on node %d", nodeID)

	return nil
}

func (am *AgentManager) GetNodeStatus(ctx context.Context, nodeID uint64) (*api.MonitorHeartbeatHTTP, error) {
	monitorClient, ok := am.agentMonitorClient[nodeID]
	if !ok {
		return nil, fmt.Errorf("no client for node %d", nodeID)
	}
	return monitorClient.GetStatus(ctx)
}
