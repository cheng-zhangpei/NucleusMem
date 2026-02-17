package agent_monitor

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AgentMonitor struct {
	id     uint64
	Config *configs.MonitorConfig

	// 状态保护
	mu                 sync.RWMutex
	agents             map[string]*api.AgentInfo
	clients            map[uint64]*client.AgentClient // For connected external agents
	agentManagerClient *client.AgentManagerClient
	// todo(cheng) WAL日志的思路，记录agent所绑定的memspace，这个做法是用于故障恢复，agent在绑定之前需要同步在monitor中更新缓存
}

// AgentMonitorInfo 本地节点的监控汇总信息
type AgentMonitorInfo struct {
	Id           uint64
	NodeID       string
	CpuUsage     float64
	MemUsage     float64
	ActiveAgents int32
}

func NewAgentMonitor(config *configs.MonitorConfig) *AgentMonitor {
	managerClient := client.NewAgentManagerClient(config.AgentManagerUrl)
	return &AgentMonitor{
		Config:             config,
		agents:             make(map[string]*api.AgentInfo),
		clients:            make(map[uint64]*client.AgentClient), // Initialize client registry
		agentManagerClient: managerClient,
	}
}

// DestroyAgent 负责停止并清理 Agent
func (am *AgentMonitor) DestroyAgent(agentID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	_, exists := am.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	fmt.Printf("[Monitor] Stopping Agent ID=%s...\n", agentID)

	// 1. TODO: 发送 SIGTERM 或 Docker Stop
	// agent.Process.Kill()

	// 2. 从 Map 移除
	delete(am.agents, agentID)
	return nil
}

func (am *AgentMonitor) GetMonitorInfo() *AgentMonitorInfo {
	// 1. 获取 CPU 使用率（需要两次采样）
	// 第一次采样（丢弃结果）
	cpu.Percent(0, false)
	// 等待 200ms
	time.Sleep(200 * time.Millisecond)
	// 第二次采样（获取真实值）
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil || len(cpuPercent) == 0 {
		cpuPercent = []float64{0.0}
	}

	// 2. 获取内存信息
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		vmStat = &mem.VirtualMemoryStat{
			UsedPercent: 0,
		}
	}

	am.mu.RLock()
	activeAgents := int32(len(am.agents))
	am.mu.RUnlock()

	return &AgentMonitorInfo{
		NodeID:       strconv.FormatUint(am.id, 10),
		CpuUsage:     cpuPercent[0] / 100.0, // 转为 0.0~1.0
		MemUsage:     vmStat.UsedPercent / 100.0,
		ActiveAgents: activeAgents,
	}
}

func (am *AgentMonitor) LaunchAgentInternal(req *LaunchAgentInternalRequest) (*api.AgentInfo, error) {
	// Step 1: 加载 Agent 配置文件
	agentCfg, err := configs.LoadAgentConfigFromYAML(req.ConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent config from %s: %w", req.ConfigFilePath, err)
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	agentKey := strconv.FormatUint(agentCfg.AgentId, 10)
	if _, exists := am.agents[agentKey]; exists {
		return nil, fmt.Errorf("agent %d is already running", agentCfg.AgentId)
	}

	log.Infof("[agent_monitor] Launching Agent ID=%d using config: %s", agentCfg.AgentId, req.ConfigFilePath)

	// Step 2: 启动 Agent 进程
	cmd := exec.Command(req.BinPath, "--config", req.ConfigFilePath)

	if req.Env != nil {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agent process: %w", err)
	}

	// Step 3: 等待启动
	time.Sleep(800 * time.Millisecond)

	// Step 4: 从配置文件中获取 HTTP 地址，创建客户端
	httpAddr := agentCfg.HttpAddr
	if !strings.HasPrefix(httpAddr, "http://") && !strings.HasPrefix(httpAddr, "https://") {
		httpAddr = "http://" + httpAddr
	}

	agentClient := client.NewAgentClient(httpAddr)
	_, err = agentClient.HealthCheckWithMonitor(am.id)
	if err != nil {
		log.Warnf("Agent %d health check failed: %v", agentCfg.AgentId, err)
	}

	// Step 5: 注册到 Monitor
	am.clients[agentCfg.AgentId] = agentClient
	agentInfo := &api.AgentInfo{
		AgentID: agentCfg.AgentId,
		Addr:    agentCfg.HttpAddr,
	}
	am.agents[agentKey] = agentInfo

	log.Infof("[agent_monitor] Successfully launched Agent %d at %s", agentCfg.AgentId, agentCfg.HttpAddr)
	return agentInfo, nil
}

// pkg/agent_monitor/agent_monitor.go
func (am *AgentMonitor) StopAgent(agentID uint64) error {
	am.mu.Lock()
	client, exists := am.clients[agentID]
	am.mu.Unlock()

	if !exists {
		return fmt.Errorf("agent %d not found", agentID)
	}

	// Step 1: 通知 Agent 自我销毁
	err := client.Shutdown()
	if err != nil {
		log.Warnf("[agent_monitor] Failed to shutdown Agent %d: %v", agentID, err)
		// 继续清理本地状态（避免僵尸记录）
	}
	// Step 2: 清理本地状态
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.agents, strconv.FormatUint(agentID, 10))
	delete(am.clients, agentID)
	log.Infof("[agent_monitor] Stopped Agent ID=%d", agentID)
	return nil
}

// GetNodeStatusInternal 返回节点状态的内部结构
func (am *AgentMonitor) GetNodeStatusInternal() *MonitorNodeStatusInternal {
	am.mu.RLock()
	defer am.mu.RUnlock()
	info := am.GetMonitorInfo()
	var agentStatuses []AgentRuntimeStatusInternal
	for _, _ = range am.agents {
		agentStatuses = append(agentStatuses, AgentRuntimeStatusInternal{})
	}

	return &MonitorNodeStatusInternal{
		NodeID:       info.NodeID,
		CPUUsage:     info.CpuUsage,
		MemUsage:     info.MemUsage,
		ActiveAgents: info.ActiveAgents,
		Agents:       agentStatuses,
		Timestamp:    time.Now().Unix(),
	}
}

// ConnectToAgent connects to an externally running Agent via HTTP
func (am *AgentMonitor) ConnectToAgent(agentID uint64, addr string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	if _, exists := am.clients[agentID]; exists {
		return fmt.Errorf("[monitor %d]agent %d already connected", am.id, agentID)
	}
	baseURL := addr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		baseURL = "http://" + addr
	}

	client := client.NewAgentClient(baseURL)

	// Use new health check with monitor binding
	_, err := client.HealthCheckWithMonitor(am.id)
	if err != nil {
		return fmt.Errorf("failed to connect to agent %d at %s: %w", agentID, addr, err)
	}

	am.clients[agentID] = client
	log.Infof("[AgentMonitor] Connected to external Agent ID=%d at %s (Monitor ID=%d)",
		agentID, addr, am.id)
	return nil
}

// pkg/agent_monitor/agent_monitor.go
type NodeSystemInfo struct {
	NodeID       string                   `json:"node_id"`
	CPUUsage     float64                  `json:"cpu_usage"`
	MemUsage     float64                  `json:"mem_usage"`
	ActiveAgents int32                    `json:"active_agents"`
	Agents       []api.AgentRuntimeStatus `json:"agents"` // ← 新增
	Timestamp    int64                    `json:"timestamp"`
}

func (am *AgentMonitor) GetNodeSystemInfo() *NodeSystemInfo {
	// CPU/Mem 采集（保持不变）
	cpu.Percent(0, false)
	time.Sleep(100 * time.Millisecond)
	cpuPerc, _ := cpu.Percent(0, false)
	vmStat, _ := mem.VirtualMemory()

	am.mu.RLock()
	defer am.mu.RUnlock()

	// 收集所有 Agent 的运行时状态
	var agents []api.AgentRuntimeStatus

	// 1. 外部连接的 Agents (am.clients)
	for agentID, client := range am.clients {
		agents = append(agents, api.AgentRuntimeStatus{
			AgentID: strconv.FormatUint(agentID, 10),
			Phase:   "Connected",
			Addr:    client.BaseURL(), // 需要添加这个方法
		})
	}
	// 2. 本地启动的 Agents (am.agents) - 如果有实现的话
	for _, agentInfo := range am.agents {
		agents = append(agents, api.AgentRuntimeStatus{
			AgentID: strconv.FormatUint(agentInfo.AgentID, 10),
			Phase:   "Running",
			Addr:    agentInfo.Addr,
		})
	}

	return &NodeSystemInfo{
		NodeID:       strconv.FormatUint(am.id, 10),
		CPUUsage:     safePercent(cpuPerc[0]),
		MemUsage:     safePercent(vmStat.UsedPercent),
		ActiveAgents: int32(len(agents)),
		Agents:       agents, // ← 新增字段
		Timestamp:    time.Now().Unix(),
	}
}

// pkg/agent_monitor/agent_monitor.go
func (am *AgentMonitor) notifyAgentManager() {
	if am.agentManagerClient == nil {
		return // 未配置
	}

	status := am.GetNodeSystemInfo()
	update := api.MonitorStatusUpdate{
		NodeID:       am.id,
		CPUUsage:     status.CPUUsage,
		MemUsage:     status.MemUsage,
		ActiveAgents: status.ActiveAgents,
		Agents:       status.Agents,
		Timestamp:    status.Timestamp,
	}
	am.agentManagerClient.SendStatusUpdate(update)
}

// POST /api/v1/agents/connect
func (s *AgentMonitorHTTPServer) handleConnectAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req api.ConnectAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.AgentID == 0 {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}
	if req.Addr == "" {
		http.Error(w, "addr is required", http.StatusBadRequest)
		return
	}

	err := s.monitor.ConnectToAgent(req.AgentID, req.Addr)
	resp := struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"error_message,omitempty"`
	}{
		Success: err == nil,
	}
	if err != nil {
		resp.ErrorMessage = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func safePercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 1.0
	}
	return v / 100.0
}
