package monitor

import (
	"NucleusMem/pkg/configs"
	"fmt"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type AgentMonitor struct {
	id         uint64
	config     *configs.MonitorConfig
	grpcServer *grpc.Server

	// 状态保护
	mu     sync.RWMutex
	agents map[string]*RunningAgent
}

// RunningAgent 描述一个正在该 Monitor 节点上运行的 Agent 实例
type RunningAgent struct {
	ID           string // 统一转为 String 作为 Map Key
	RawID        uint64
	Role         string
	StartTime    time.Time
	Status       string // "Running", "Failed", "Stopped"
	RestartCount int32
	// Process *os.Process // 如果是进程模式，持有进程句柄
	// ContainerID string  // 如果是 Docker 模式，持有容器 ID
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
	return &AgentMonitor{
		config: config,
		agents: make(map[string]*RunningAgent),
	}
}

// CreateAgent 负责实际启动一个 Agent (进程或容器)
func (am *AgentMonitor) CreateAgent(config *configs.AgentConfig) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agentKey := strconv.FormatUint(config.AgentId, 10)

	// 1. 幂等检查
	if _, exists := am.agents[agentKey]; exists {
		return fmt.Errorf("agent %s is already running", agentKey)
	}

	fmt.Printf("[Monitor] Launching Agent ID=%d, Role=%s, Image=%s, Path=%s\n",
		config.AgentId, config.Role, config.Image, config.Path)

	// 2. TODO: 这里对接 Docker SDK 或 os/exec
	// cmd := exec.Command(config.BinPath, ...)
	// err := cmd.Start()

	// 3. 更新内存状态
	am.agents[agentKey] = &RunningAgent{
		ID:        agentKey,
		RawID:     config.AgentId,
		Role:      config.Role,
		StartTime: time.Now(),
		Status:    "Running",
	}

	return nil
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

// GetMonitorInfo 获取当前节点监控快照
func (am *AgentMonitor) GetMonitorInfo() *AgentMonitorInfo {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// TODO: 集成 gopsutil 获取真实系统负载
	return &AgentMonitorInfo{
		Id:           am.id,
		CpuUsage:     0.15, // Mock
		MemUsage:     0.45, // Mock
		ActiveAgents: int32(len(am.agents)),
	}
}

// -----------------------------------------------------------------------------
// gRPC Handler Implementation (实现 Proto 接口)
// -----------------------------------------------------------------------------
//
//// LaunchAgent 接收 Manager 的启动指令
//func (am *AgentMonitor) LaunchAgent(ctx context.Context, req *pb.LaunchAgentRequest) (*pb.LaunchAgentResponse, error) {
//	// 1. Proto -> Internal Config 转换
//	cfg := &configs.AgentConfig{
//		AgentId:     req.Config.AgentId,
//		Role:        req.Config.Role,
//		Image:       req.Config.Image,
//		BinPath:     req.Config.BinPath,
//		MountMemIds: req.Config.MountMemIds,
//	}
//
//	// 2. 调用业务逻辑
//	err := am.CreateAgent(cfg)
//	if err != nil {
//		return &pb.LaunchAgentResponse{
//			Success:      false,
//			ErrorMessage: err.Error(),
//		}, nil
//	}
//
//	return &pb.LaunchAgentResponse{Success: true}, nil
//}
//
//// StopAgent 接收 Manager 的停止指令
//func (am *AgentMonitor) StopAgent(ctx context.Context, req *pb.StopAgentRequest) (*pb.StopAgentResponse, error) {
//	err := am.DestroyAgent(req.AgentId)
//	if err != nil {
//		// 注意：即使停止失败，通常也返回 success=false，而不是 grpc error，除非是网络层面的错误
//		return &pb.StopAgentResponse{Success: false}, nil
//	}
//	return &pb.StopAgentResponse{Success: true}, nil
//}
//
//// GetNodeStatus 接收心跳或状态查询
//func (am *AgentMonitor) GetNodeStatus(ctx context.Context, req *pb.Empty) (*pb.MonitorHeartbeat, error) {
//	am.mu.RLock()
//	defer am.mu.RUnlock()
//
//	// 1. 获取基本信息
//	info := am.GetMonitorInfo()
//
//	// 2. 构建 Agent 详情列表
//	var agentStatuses []*pb.AgentRuntimeStatus
//	for _, a := range am.agents {
//		agentStatuses = append(agentStatuses, &pb.AgentRuntimeStatus{
//			AgentId:      a.ID, // 注意 Proto 里定义的是 string agent_id
//			Phase:        a.Status,
//			StartTime:    a.StartTime.Unix(),
//			RestartCount: a.RestartCount,
//		})
//	}
//
//	// 3. 组装响应
//	return &pb.MonitorHeartbeat{
//		NodeId:       info.NodeID,
//		CpuUsage:     info.CpuUsage,
//		MemUsage:     info.MemUsage,
//		ActiveAgents: info.ActiveAgents,
//		Agents:       agentStatuses,
//	}, nil
//}
//
//// -----------------------------------------------------------------------------
//// Server Lifecycle
//// -----------------------------------------------------------------------------
//
//// Start 启动 gRPC Server (阻塞模式)
//func (am *AgentMonitor) Start() error {
//	addr := am.config.GrpcServerAddr
//	lis, err := net.Listen("tcp", addr)
//	if err != nil {
//		return fmt.Errorf("failed to listen on %s: %v", addr, err)
//	}
//
//	am.grpcServer = grpc.NewServer()
//
//	// 注册服务
//	pb.RegisterAgentMonitorServiceServer(am.grpcServer, am)
//
//	// 注册反射服务 (可选，方便用 grpcurl 调试)
//	reflection.Register(am.grpcServer)
//
//	fmt.Printf("AgentMonitor is listening on %s\n", addr)
//
//	// 阻塞运行
//	return am.grpcServer.Serve(lis)
//}
//
//// Stop 优雅关闭
//func (am *AgentMonitor) Stop() {
//	if am.grpcServer != nil {
//		am.grpcServer.GracefulStop()
//	}
//}
