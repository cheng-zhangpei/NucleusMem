package agent_manager

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	_ "github.com/pingcap-incubator/tinykv/log"
	"strconv"
)

type AgentManager struct {
	agentCache *AgentCache // maintain the information of every agent which enable the manager to connect every agent
	//monitorCache *Mo
	// todo (cheng proto define) monitorClient\agentClient
	agentMonitorClient map[uint64]*client.AgentMonitorClient
	monitorURLs        map[uint64]string
}

// pkg/manager/agent_manager.go
func NewAgentManager(agentManagerConfig *configs.AgentManagerConfig) (*AgentManager, error) {
	ctx := context.Background()
	monitorClients := make(map[uint64]*client.AgentMonitorClient)

	// 1. Build monitor clients with proper HTTP URLs
	log.Infof("[manager]Initializing AgentManager with %d monitors", len(agentManagerConfig.MonitorURLs))
	for id, addr := range agentManagerConfig.MonitorURLs {
		httpAddr := EnsureHTTPPrefix(addr)
		log.Infof("[manager]  → Monitor %d: %s (HTTP: %s)", id, addr, httpAddr)
		mc := client.NewAgentMonitorClient(httpAddr)
		monitorClients[id] = mc
	}

	agentCache := NewAgentCache()
	am := &AgentManager{
		agentCache:         agentCache,
		agentMonitorClient: monitorClients,
		monitorURLs:        make(map[uint64]string),
	}
	for id, addr := range agentManagerConfig.MonitorURLs {
		am.monitorURLs[id] = addr
	}
	// 2. Pull current status from each Monitor to populate cache
	log.Infof("[manager]Pulling initial status from all monitors...")
	for id := range monitorClients {
		log.Infof("  → Fetching status from Monitor %d", id)
		nodeStatus, err := am.GetNodeStatus(ctx, id)
		if err != nil {
			log.Errorf("Failed to get status from Monitor %d: %v", id, err)
			return nil, err
		}

		nodeIDStr := strconv.FormatUint(id, 10)
		log.Infof("    ↳ Monitor %s: CPU=%.2f%%, Mem=%.2f%%, Agents=%d",
			nodeIDStr, nodeStatus.CPUUsage*100, nodeStatus.MemUsage*100, len(nodeStatus.Agents))

		// Add each running agent to cache
		for _, agentStatus := range nodeStatus.Agents {
			agentID, err := strconv.ParseUint(agentStatus.AgentID, 10, 64)
			if err != nil {
				log.Warnf("Invalid agent ID format: %s, skipping", agentStatus.AgentID)
				continue
			}
			am.agentCache.UpdateAgent(&AgentInfo{
				AgentID:  agentID,
				Status:   agentStatus.Phase,
				NodeID:   id,
				NodeAddr: am.monitorURLs[id],
				HTTPAddr: "",
			})
			log.Infof("      → Cached Agent %d (Status: %s)", agentID, agentStatus.Phase)
		}
	}

	log.Infof("AgentManager initialized successfully with %d agents in cache", len(agentCache.agents))
	return am, nil
}

// BingMemSpace binding the memspace and agent
func (*AgentManager) BingMemSpace() error {
	return nil
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

	// 获取 Monitor 地址
	nodeAddr, exists := am.monitorURLs[nodeID]
	if !exists {
		nodeAddr = "unknown"
	}
	am.agentCache.UpdateAgent(&AgentInfo{
		AgentID:  resp.AgentID, // uint64
		Status:   "Running",
		NodeID:   nodeID,        // uint64
		NodeAddr: nodeAddr,      // string
		HTTPAddr: resp.HttpAddr, // string
	})

	log.Infof("[agent_manager] Launched Agent %d on Node %d", resp.AgentID, nodeID)
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
	am.agentCache.RemoveAgent(agentID)
	log.Infof("[agent_manager] Stoped agent %d on node %d", agentID, nodeID)

	return nil
}

func (am *AgentManager) GetNodeStatus(ctx context.Context, nodeID uint64) (*api.MonitorHeartbeatHTTP, error) {
	monitorClient, ok := am.agentMonitorClient[nodeID]
	if !ok {
		return nil, fmt.Errorf("no client for node %d", nodeID)
	}
	return monitorClient.GetStatus(ctx)
}
func (am *AgentManager) DisplayCache() {
	agents := am.agentCache.GetAllAgents()
	if len(agents) == 0 {
		log.Infof("Agent Cache is empty")
		return
	}
	// Print header
	log.Infof("Current Agent Cache (%d agents):", len(agents))
	log.Infof("%-8s %-12s %-8s %-15s %s", "AGENT_ID", "STATUS", "NODE_ID", "NODE_ADDR", "HTTP_ADDR")
	// Print each agent
	for _, a := range agents {
		log.Infof("%-8d %-12s %-8d %-15s %s",
			a.AgentID,
			a.Status,
			a.NodeID,
			a.NodeAddr,
			a.HTTPAddr,
		)
	}
}

// GenerateAgentService create agents resource in a cloud-native environment (distributed env)
func (*AgentManager) GenerateAgentService(config configs.AgentConfig) error {
	return nil
}

func (*AgentManager) destroyAgentService(config configs.AgentConfig) error {
	return nil
}
