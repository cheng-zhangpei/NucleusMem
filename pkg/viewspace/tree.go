// pkg/viewspace/tree.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

type ViewSpaceTree struct {
	Meta  Meta
	Root  *ViewSpaceNode
	Nodes map[string]*ViewSpaceNode
	mu    sync.RWMutex

	AgentMgrClient    *client.AgentManagerClient
	MemSpaceMgrClient *client.MemSpaceManagerClient

	AvailableTools []string

	// Config for launching resources
	LaunchConfig *TreeLaunchConfig
}

// TreeLaunchConfig holds the config needed to launch agents and memspaces
// These are infrastructure details that the tree itself shouldn't hardcode
type TreeLaunchConfig struct {
	// Agent launch config
	AgentBinPath    string
	AgentConfigPath string
	AgentBasePort   int // will increment for each agent

	// MemSpace launch config
	MemSpaceBinPath    string
	MemSpaceConfigPath string
	MemSpaceBasePort   int
	PdAddr             string
	EmbeddingAddr      string
	LightModelAddr     string

	// Port counter (thread-safe)
	portMu   sync.Mutex
	nextPort int
}

func (c *TreeLaunchConfig) AllocPort() int {
	c.portMu.Lock()
	defer c.portMu.Unlock()
	port := c.nextPort
	c.nextPort++
	return port
}

type ViewSpaceNode struct {
	Name        string
	Type        string
	Tags        []string
	Description string
	Role        string
	Tools       []string

	Parent   *ViewSpaceNode
	Children []*ViewSpaceNode
	mu       sync.RWMutex

	ChildDataflow []DataflowEdge

	MemSpaceID   uint64
	MemSpaceAddr string
	AgentID      uint64
	AgentAddr    string
	AgentClient  *client.AgentClient

	Status         NodeStatus
	hasGrown       bool
	tree           *ViewSpaceTree
	MemSpaceClient *client.MemSpaceClient `json:"-"`

	RepresentedAgentID uint64   `json:"represented_agent_id"`
	WorkerAgentIDs     []uint64 `json:"worker_agent_ids,omitempty"`
	SharedMemSpaceIDs  []uint64 `json:"shared_memspace_ids,omitempty"`
}

// Run is the entry point
func Run(
	taskDescription string,
	availableTools []string,
	agentMgrAddr string,
	memSpaceMgrAddr string,
	launchConfig *TreeLaunchConfig,
) (*ViewSpaceTree, error) {

	if launchConfig.nextPort == 0 {
		launchConfig.nextPort = launchConfig.AgentBasePort
		if launchConfig.nextPort == 0 {
			launchConfig.nextPort = 20000
		}
	}

	tree := &ViewSpaceTree{
		Meta: Meta{
			TaskID:      fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Description: taskDescription,
		},
		Nodes:             make(map[string]*ViewSpaceNode),
		AgentMgrClient:    client.NewAgentManagerClient(agentMgrAddr),
		MemSpaceMgrClient: client.NewMemSpaceManagerClient(memSpaceMgrAddr),
		AvailableTools:    availableTools,
		LaunchConfig:      launchConfig,
	}

	log.Infof("=== ViewSpaceTree Run ===")
	log.Infof("Task: %s", taskDescription)
	log.Infof("Tools: %v", availableTools)

	// Phase 1: Seed
	root, err := tree.seed(taskDescription)
	if err != nil {
		return nil, fmt.Errorf("seed failed: %w", err)
	}
	tree.Root = root

	// Phase 2: Grow recursively
	if err := tree.grow(root); err != nil {
		return nil, fmt.Errorf("grow failed: %w", err)
	}

	// Summary
	g, p, a := tree.GetNodeCount()
	log.Infof("=== Tree Complete: %d global, %d process, %d atomic ===", g, p, a)
	log.Infof("\n%s", tree.PrintTree())

	return tree, nil
}

func (t *ViewSpaceTree) seed(taskDescription string) (*ViewSpaceNode, error) {
	log.Infof("[Seed] Creating Global ViewSpace...")

	node := &ViewSpaceNode{
		Name:        "global-root",
		Type:        "global",
		Tags:        []string{"root", "coordination"},
		Description: taskDescription,
		Role:        "Global Coordinator",
		Children:    make([]*ViewSpaceNode, 0),
		Status:      NodeStatusPending,
		tree:        t,
	}

	if err := t.bringToLife(node); err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.Nodes[node.Name] = node
	t.mu.Unlock()

	log.Infof("[Seed] Root alive: agent=%d@%s, ms=%d@%s",
		node.AgentID, node.AgentAddr, node.MemSpaceID, node.MemSpaceAddr)

	return node, nil
}

func (t *ViewSpaceTree) bringToLife(node *ViewSpaceNode) error {
	cfg := t.LaunchConfig

	// 1. Allocate a port for the memspace
	msPort := cfg.AllocPort()
	msAddr := fmt.Sprintf("127.0.0.1:%d", msPort)

	// Generate a unique memspace ID based on timestamp
	msID := uint64(time.Now().UnixNano() % 1000000)

	// Launch MemSpace
	msReq := &api.LaunchMemSpaceRequestManager{
		MemSpaceID:          msID,
		Name:                fmt.Sprintf("ms-%s", node.Name),
		Type:                "public",
		Description:         fmt.Sprintf("MemSpace for ViewSpace '%s'", node.Name),
		HttpAddr:            msAddr,
		PdAddr:              cfg.PdAddr,
		EmbeddingClientAddr: cfg.EmbeddingAddr,
		LightModelAddr:      cfg.LightModelAddr,
		BinPath:             cfg.MemSpaceBinPath,
		ConfigFilePath:      cfg.MemSpaceConfigPath,
	}

	if err := t.MemSpaceMgrClient.LaunchMemSpace(msReq); err != nil {
		return fmt.Errorf("launch memspace for '%s': %w", node.Name, err)
	}

	node.MemSpaceID = msID
	node.MemSpaceAddr = msAddr

	// 2. Allocate a port for the agent
	agentPort := cfg.AllocPort()
	agentAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)

	// Generate agent ID
	agentID := uint64(time.Now().UnixNano() % 1000000)

	// Launch Agent
	agentReq := &api.LaunchAgentRequestHTTP{
		AgentID:        agentID,
		Role:           node.Role,
		BinPath:        cfg.AgentBinPath,
		HttpAddr:       agentAddr,
		ConfigFilePath: cfg.AgentConfigPath,
		IsJob:          false,
	}

	agentResp, err := t.AgentMgrClient.LaunchAgent(agentReq)
	if err != nil {
		return fmt.Errorf("launch agent for '%s': %w", node.Name, err)
	}

	node.AgentID = agentResp.AgentID
	node.AgentAddr = agentAddr
	if agentResp.HttpAddr != "" {
		node.AgentAddr = agentResp.HttpAddr
	}

	// 3. Bind
	if err := t.MemSpaceMgrClient.BindMemSpace(node.AgentID, node.MemSpaceID, node.AgentAddr, node.Role); err != nil {
		return fmt.Errorf("bind for '%s': %w", node.Name, err)
	}
	// 4. Create client
	node.AgentClient = client.NewAgentClient(node.AgentAddr)

	node.Status = NodeStatusReady
	return nil
}

func (t *ViewSpaceTree) grow(node *ViewSpaceNode) error {
	// Guard: atomic cannot grow(exist condition)
	if node.Type == "atomic" {
		log.Infof("[Grow] '%s' is atomic, skip", node.Name)
		return nil
	}

	// Guard: only grow once
	node.mu.Lock()
	if node.hasGrown {
		node.mu.Unlock()
		log.Warnf("[Grow] '%s' already grown, skip", node.Name)
		return nil
	}
	node.hasGrown = true
	node.Status = NodeStatusGrowing
	node.mu.Unlock()

	log.Infof("[Grow] '%s' (agent:%d) decomposing...", node.Name, node.AgentID)

	// Step 1: Submit decompose task
	submitReq := &api.SubmitTaskRequest{
		Type:           "decompose",
		Content:        node.Description,
		AvailableTools: t.AvailableTools,
		MaxRetry:       3,
	}

	submitResp, err := node.AgentClient.SubmitTask(submitReq)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("submit to '%s': %w", node.Name, err)
	}

	log.Infof("[Grow] '%s' task=%s, waiting...", node.Name, submitResp.TaskID)

	// Step 2: Wait for result
	resultResp, err := node.AgentClient.GetTaskResult(submitResp.TaskID, 120*time.Second)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("wait for '%s': %w", node.Name, err)
	}

	if resultResp.ErrorMessage != "" {
		node.Status = NodeStatusFailed
		return fmt.Errorf("decompose '%s' failed: %s", node.Name, resultResp.ErrorMessage)
	}

	// Step 3: Parse
	spawnResult, err := parseAgentDecomposeResult(resultResp.Result)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("parse '%s': %w", node.Name, err)
	}

	log.Infof("[Grow] '%s' → %d children, %d dataflow",
		node.Name, len(spawnResult.Children), len(spawnResult.Dataflow))

	// Step 4: Create children
	for _, childDef := range spawnResult.Children {
		child := &ViewSpaceNode{
			Name:        childDef.Name,
			Type:        childDef.Type,
			Tags:        childDef.Tags,
			Description: childDef.Description,
			Role:        childDef.Role,
			Tools:       childDef.Tools,
			Parent:      node,
			Children:    make([]*ViewSpaceNode, 0),
			Status:      NodeStatusPending,
			tree:        t,
		}

		if err := t.bringToLife(child); err != nil {
			node.Status = NodeStatusFailed
			return fmt.Errorf("create '%s': %w", child.Name, err)
		}

		t.mu.Lock()
		t.Nodes[child.Name] = child
		t.mu.Unlock()

		node.mu.Lock()
		node.Children = append(node.Children, child)
		node.mu.Unlock()

		log.Infof("[Grow] '%s' → [%s] '%s' (agent:%d, ms:%d)",
			node.Name, child.Type, child.Name, child.AgentID, child.MemSpaceID)
	}

	// Step 5: Store dataflow
	node.mu.Lock()
	// todo(cheng) update the message in the memspace of this viewSpace
	node.ChildDataflow = spawnResult.Dataflow
	node.Status = NodeStatusReady
	node.mu.Unlock()

	// Step 6: Recursive grow for process children
	for _, child := range node.Children {
		if child.Type == "process" {
			log.Infof("[Grow] Recursing into process '%s'...", child.Name)
			if err := t.grow(child); err != nil {
				return err
			}
		}
	}

	log.Infof("[Grow] '%s' complete: %d children", node.Name, len(node.Children))
	return nil
}

func parseAgentDecomposeResult(resultJSON string) (*SpawnResult, error) {
	var def TaskDefinition
	if err := json.Unmarshal([]byte(resultJSON), &def); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	result := &SpawnResult{
		Children: make([]SpawnChild, 0),
		Dataflow: def.Dependencies.Dataflow,
	}

	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" {
			continue
		}
		result.Children = append(result.Children, SpawnChild{
			Name:        vs.Name,
			Type:        vs.Type,
			Tags:        vs.Tags,
			Description: vs.Description,
			Role:        vs.Role,
			Tools:       vs.Tools,
		})
	}

	if len(result.Children) == 0 {
		return nil, fmt.Errorf("no children in agent output")
	}

	return result, nil
}

func (t *ViewSpaceTree) Shutdown() {
	if t.Root == nil {
		return
	}
	log.Infof("=== Shutdown ===")
	t.shutdownNode(t.Root)
}

func (t *ViewSpaceTree) shutdownNode(node *ViewSpaceNode) {
	for _, child := range node.Children {
		t.shutdownNode(child)
	}
	if node.AgentID != 0 {
		t.AgentMgrClient.StopAgent(node.AgentID)
	}
	if node.MemSpaceID != 0 {
		t.MemSpaceMgrClient.ShutdownMemSpace(node.MemSpaceID)
	}
	log.Infof("[Shutdown] '%s' destroyed", node.Name)
}

func (t *ViewSpaceTree) PrintTree() string {
	if t.Root == nil {
		return "<empty>"
	}
	return printNode(t.Root, "", true)
}

func printNode(node *ViewSpaceNode, prefix string, isLast bool) string {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	line := fmt.Sprintf("%s%s[%s] %s (agent:%d, ms:%d, %s)",
		prefix, connector, node.Type, node.Name,
		node.AgentID, node.MemSpaceID, node.Status)

	if len(node.Tools) > 0 {
		line += fmt.Sprintf(" tools:%v", node.Tools)
	}
	if node.hasGrown {
		line += " [grown]"
	}
	line += "\n"

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	node.mu.RLock()
	children := make([]*ViewSpaceNode, len(node.Children))
	copy(children, node.Children)
	node.mu.RUnlock()

	for i, child := range children {
		line += printNode(child, childPrefix, i == len(children)-1)
	}

	return line
}

func (t *ViewSpaceTree) GetNode(name string) (*ViewSpaceNode, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.Nodes[name]
	return n, ok
}

func (t *ViewSpaceTree) GetAllAtomicNodes() []*ViewSpaceNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var atomics []*ViewSpaceNode
	for _, n := range t.Nodes {
		if n.Type == "atomic" {
			atomics = append(atomics, n)
		}
	}
	return atomics
}

func (t *ViewSpaceTree) GetNodeCount() (global, process, atomic int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, n := range t.Nodes {
		switch n.Type {
		case "global":
			global++
		case "process":
			process++
		case "atomic":
			atomic++
		}
	}
	return
}
