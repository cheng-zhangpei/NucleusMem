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
	"sync"
	"time"
)

type AgentManager struct {
	agentCache *AgentCache // maintain the information of every agent which enable the manager to connect every agent
	//monitorCache *Mo
	agentMonitorClient map[uint64]*client.AgentMonitorClient
	monitorURLs        map[uint64]string
	failedMonitors     map[uint64]string // 失败的 Monitor (nodeID -> URL)
	mu                 sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAgentManager creates a new AgentManager with health check and retry mechanism
func NewAgentManager(agentManagerConfig *configs.AgentManagerConfig) (*AgentManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	monitorClients := make(map[uint64]*client.AgentMonitorClient)
	failedMonitors := make(map[uint64]string)
	monitorURLs := make(map[uint64]string)

	log.Infof("[manager] Initializing AgentManager with %d monitors", len(agentManagerConfig.MonitorURLs))

	// Step 1: 初始化时尝试连接所有 Monitor
	for id, addr := range agentManagerConfig.MonitorURLs {
		httpAddr := EnsureHTTPPrefix(addr)
		monitorURLs[id] = addr

		mc := client.NewAgentMonitorClient(httpAddr)

		// Health Check
		if _, err := mc.GetStatus(ctx); err != nil {
			log.Warnf("[manager] Monitor %d health check failed: %v (will retry)", id, err)
			failedMonitors[id] = addr
		} else {
			log.Infof("[manager] Monitor %d connected successfully", id)
			monitorClients[id] = mc
		}
	}

	agentCache := NewAgentCache()
	am := &AgentManager{
		agentCache:         agentCache,
		agentMonitorClient: monitorClients,
		monitorURLs:        monitorURLs,
		failedMonitors:     failedMonitors,
		ctx:                ctx,
		cancel:             cancel,
	}

	// Step 2: 启动后台重试线程
	am.startRetryWorker()

	// Step 3: 从已连接的 Monitor 拉取初始状态
	log.Infof("[agent_manager] Pulling initial status from %d connected monitors...", len(monitorClients))
	for id := range monitorClients {
		log.Infof("  → Fetching status from Monitor %d", id)
		nodeStatus, err := am.GetNodeStatus(ctx, id)
		if err != nil {
			log.Errorf("Failed to get status from Monitor %d: %v", id, err)
			continue
		}

		log.Infof("    ↳ Monitor %d: CPU=%.2f%%, Mem=%.2f%%, Agents=%d",
			id, nodeStatus.CPUUsage*100, nodeStatus.MemUsage*100, len(nodeStatus.Agents))

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

	log.Infof("[agent_manager]AgentManager initialized successfully with %d agents in cache", len(agentCache.agents))
	return am, nil
}

func (am *AgentManager) startRetryWorker() {
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()

		ticker := time.NewTicker(10 * time.Second) // 每 30 秒重试一次
		defer ticker.Stop()

		for {
			select {
			case <-am.ctx.Done():
				log.Info("[manager] Monitor retry worker stopped")
				return
			case <-ticker.C:
				am.retryFailedMonitors()
			}
		}
	}()
	log.Infof("[agent_manager] Monitor retry worker started")
}

// retryFailedMonitors retries all failed monitors
func (am *AgentManager) retryFailedMonitors() {
	am.mu.Lock()
	failedCopy := make(map[uint64]string, len(am.failedMonitors))
	for k, v := range am.failedMonitors {
		failedCopy[k] = v
	}
	am.mu.Unlock()

	if len(failedCopy) == 0 {
		return
	}

	log.Infof("[agent_manager] Retrying %d failed monitors...", len(failedCopy))

	for nodeID, addr := range failedCopy {
		select {
		case <-am.ctx.Done():
			return
		default:
		}

		httpAddr := EnsureHTTPPrefix(addr)
		mc := client.NewAgentMonitorClient(httpAddr)

		if _, err := mc.GetStatus(am.ctx); err != nil {
			log.Debugf("[manager] Retry Monitor %d failed: %v", nodeID, err)
			continue
		}

		// Success! Move from failed to active
		am.mu.Lock()
		if _, exists := am.failedMonitors[nodeID]; exists {
			delete(am.failedMonitors, nodeID)
			am.agentMonitorClient[nodeID] = mc
			log.Infof("[agent_manager] Monitor %d reconnected successfully", nodeID)

			// 拉取该 Monitor 的 Agent 状态
			go am.syncAgentFromMonitor(nodeID, mc)
		}
		am.mu.Unlock()
	}
}

// syncAgentFromMonitor loads agent info from a specific monitor
func (am *AgentManager) syncAgentFromMonitor(nodeID uint64, mc *client.AgentMonitorClient) {
	status, err := mc.GetStatus(am.ctx)
	if err != nil {
		log.Warnf("[manager] Failed to get status from monitor %d: %v", nodeID, err)
		return
	}

	for _, agentStatus := range status.Agents {
		agentID, err := strconv.ParseUint(agentStatus.AgentID, 10, 64)
		if err != nil {
			continue
		}
		am.agentCache.UpdateAgent(&AgentInfo{
			AgentID:  agentID,
			Status:   agentStatus.Phase,
			NodeID:   nodeID,
			NodeAddr: am.monitorURLs[nodeID],
			HTTPAddr: "",
		})
	}
	log.Infof("[agent_manager] Synced %d agents from monitor %d", len(status.Agents), nodeID)
}

// BingMemSpace binding the memspace and agent
func (*AgentManager) BingMemSpace() error {
	return nil
}

func (am *AgentManager) LaunchAgentOnNode(ctx context.Context, nodeID uint64, req *api.LaunchAgentRequestHTTP) (*AgentInfo, error) {
	monitorClient, ok := am.agentMonitorClient[nodeID]
	if !ok {
		return nil, fmt.Errorf("no client for node %d", nodeID)
	}
	resp, err := monitorClient.LaunchAgent(ctx, req)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("launch failed: %s", resp.ErrorMessage)
	}

	// 获取 Monitor 地址
	nodeAddr, exists := am.monitorURLs[nodeID]
	if !exists {
		nodeAddr = "unknown"
	}
	agentInfo := &AgentInfo{
		AgentID:  resp.AgentID, // uint64
		Status:   "Running",
		NodeID:   nodeID,        // uint64
		NodeAddr: nodeAddr,      // string
		HTTPAddr: resp.HttpAddr, // string
	}
	am.agentCache.UpdateAgent(agentInfo)

	log.Infof("[agent_manager] Launched Agent %d on Node %d", resp.AgentID, nodeID)
	return agentInfo, nil
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
		log.Infof("[agent_manager]Agent Cache is empty")
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

// pkg/controller/agent_manager/agent_manager.go
func (am *AgentManager) getMonitorAddr(nodeID uint64) string {
	if addr, ok := am.monitorURLs[nodeID]; ok {
		return addr
	}
	return "unknown"
}
func (am *AgentManager) syncCacheWithUpdate(update api.MonitorStatusUpdate) {
	// 构建活跃 Agent ID 集合
	activeIDs := make(map[uint64]bool)
	for _, a := range update.Agents {
		if id, err := strconv.ParseUint(a.AgentID, 10, 64); err == nil {
			activeIDs[id] = true
		}
	}
	// 清理已消失的 Agent
	allAgents := am.agentCache.GetAllAgents()
	for _, agent := range allAgents {
		if agent.NodeID == update.NodeID && !activeIDs[agent.AgentID] {
			am.agentCache.RemoveAgent(agent.AgentID)
		}
	}
}

// GenerateAgentService create agents resource in a cloud-native environment (distributed env)
func (*AgentManager) GenerateAgentService(config configs.AgentConfig) error {
	return nil
}

func (*AgentManager) destroyAgentService(config configs.AgentConfig) error {
	return nil
}
