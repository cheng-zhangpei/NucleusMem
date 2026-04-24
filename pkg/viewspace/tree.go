// pkg/viewspace/tree.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"encoding/json"
	"fmt"
	"strconv"
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
	Growth       *GrowthLog
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

// pkg/viewspace/tree.go

type ViewSpaceNode struct {
	Name        string
	Type        string // "global" | "process" | "atomic"
	Tags        []string
	Description string
	Role        string // RepresentedAgent 的角色
	Tools       []string

	// === 树形结构 ===
	Parent        *ViewSpaceNode
	Children      []*ViewSpaceNode
	ChildDataflow []DataflowEdge

	// === 主资源（必须）===
	// RepresentedAgent: 这个 ViewSpace 的协调者
	AgentID     uint64
	AgentAddr   string
	AgentClient *client.AgentClient
	// 主 MemSpace: 这个 ViewSpace 的黑板
	MemSpaceID     uint64
	MemSpaceAddr   string
	MemSpaceClient *client.MemSpaceClient

	// === 协作资源（可选）===
	// Worker Agents: 在这个 ViewSpace 内协作的其他 Agent
	Workers        []*WorkerBinding
	workersSpecs   []WorkerSpec
	memSpaceMounts []MemSpaceMount
	// Mounted MemSpaces: 额外挂载的 MemSpace（继承父级上下文、共享知识库等）
	MountedMemSpaces []*MountedMemSpace

	// === 状态 ===
	Status   NodeStatus
	hasGrown bool
	mu       sync.RWMutex
	tree     *ViewSpaceTree
}

// WorkerBinding represents a worker agent within a ViewSpace
type WorkerBinding struct {
	AgentID   uint64
	Role      string
	AgentAddr string
	Client    *client.AgentClient
}

// MountedMemSpace represents an additional MemSpace mounted to a ViewSpace
type MountedMemSpace struct {
	MemSpaceID uint64
	Addr       string
	MountType  string // "inherited" | "shared" | "readonly"
	Source     string // 来源说明，如 "parent:global-root" 或 "tag:code-repo"
	Client     *client.MemSpaceClient
}

// pkg/viewspace/tree.go

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
		Growth:            &GrowthLog{},
	}

	log.Infof("╔══════════════════════════════════════════════════════════╗")
	log.Infof("║           🌳 ViewSpaceTree Starting                    ║")
	log.Infof("║  Task: %-48s║", truncate(taskDescription, 48))
	log.Infof("║  Tools: %-47s║", truncate(fmt.Sprintf("%v", availableTools), 47))
	log.Infof("╚══════════════════════════════════════════════════════════╝")

	// Phase 1: Seed
	tree.Growth.Record("seed", "global-root",
		fmt.Sprintf("Planting seed: %s", truncate(taskDescription, 60)), tree)

	root, err := tree.seed(taskDescription)
	if err != nil {
		tree.Growth.Record("failed", "global-root",
			fmt.Sprintf("Seed failed: %v", err), tree)
		return nil, fmt.Errorf("seed failed: %w", err)
	}
	tree.Root = root

	tree.Growth.Record("ready", root.Name,
		fmt.Sprintf("Root alive → agent:%d@%s, ms:%d@%s",
			root.AgentID, root.AgentAddr, root.MemSpaceID, root.MemSpaceAddr), tree)

	// Phase 2: Grow recursively
	tree.Growth.Record("grow", root.Name,
		"Beginning recursive growth from root...", tree)

	if err := tree.Grow(root); err != nil {
		tree.Growth.Record("failed", root.Name,
			fmt.Sprintf("Growth failed: %v", err), tree)
		return nil, fmt.Errorf("grow failed: %w", err)
	}

	// Final summary
	g, p, a := tree.GetNodeCount()
	log.Infof("\n")
	log.Infof("╔══════════════════════════════════════════════════════════╗")
	log.Infof("║           🌳 Tree Growth Complete                      ║")
	log.Infof("║  Global: %-3d  Process: %-3d  Atomic: %-3d               ║", g, p, a)
	log.Infof("╚══════════════════════════════════════════════════════════╝")
	log.Infof("\n%s", tree.PrintTree())

	// Print timeline
	tree.Growth.PrintGrowthSummary()

	return tree, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
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
	// ============ Phase 1: Primary MemSpace ============
	msID, msAddr, err := t.launchMemSpaceWithRetry(
		node,
		fmt.Sprintf("ms-%s", node.Name),
		"public",
		fmt.Sprintf("Primary blackboard for '%s'", node.Name),
	)
	if err != nil {
		return fmt.Errorf("[bringToLife] '%s': %w", node.Name, err)
	}

	node.MemSpaceID = msID
	node.MemSpaceAddr = msAddr
	node.MemSpaceClient = client.NewMemSpaceClient(msAddr)

	t.Growth.Record("bringToLife", node.Name,
		fmt.Sprintf("MemSpace launched → %d@%s", msID, msAddr), t)

	// ============ Phase 2: Represented Agent ============
	agentID, agentAddr, err := t.launchAgentWithRetry(node.Role)
	if err != nil {
		return fmt.Errorf("[bringToLife] '%s': %w", node.Name, err)
	}

	node.AgentID = agentID
	node.AgentAddr = agentAddr
	node.AgentClient = client.NewAgentClient(agentAddr)

	t.Growth.Record("bringToLife", node.Name,
		fmt.Sprintf("Agent launched → %d@%s (role: %s)", agentID, agentAddr, node.Role), t)

	// ============ Phase 3: Bind Agent ↔ MemSpace ============
	if err := t.bindAgentToMemSpace(
		node.AgentID, node.AgentAddr, node.Role,
		node.MemSpaceID, node.MemSpaceAddr, "public",
	); err != nil {
		return fmt.Errorf("[bringToLife] '%s': bind: %w", node.Name, err)
	}

	t.Growth.Record("bringToLife", node.Name,
		fmt.Sprintf("Bound agent %d ↔ memspace %d", node.AgentID, node.MemSpaceID), t)

	// ============ Phase 4: Workers ============
	if node.Workers == nil {
		node.Workers = make([]*WorkerBinding, 0)
	}

	for _, spec := range node.workersSpecs {
		wID, wAddr, err := t.launchAgentWithRetry(spec.Role)
		if err != nil {
			t.Growth.Record("worker", node.Name,
				fmt.Sprintf("⚠ Worker '%s' launch failed: %v (non-fatal)", spec.Role, err), t)
			continue
		}

		if err := t.bindAgentToMemSpace(
			wID, wAddr, spec.Role,
			node.MemSpaceID, node.MemSpaceAddr, "public",
		); err != nil {
			t.Growth.Record("worker", node.Name,
				fmt.Sprintf("⚠ Worker '%s' bind failed: %v (non-fatal)", spec.Role, err), t)
			continue
		}

		node.Workers = append(node.Workers, &WorkerBinding{
			AgentID:   wID,
			Role:      spec.Role,
			AgentAddr: wAddr,
			Client:    client.NewAgentClient(wAddr),
		})

		t.Growth.Record("worker", node.Name,
			fmt.Sprintf("Worker '%s' → %d@%s", spec.Role, wID, wAddr), t)
	}

	// ============ Phase 5: Mount Additional MemSpaces ============
	if node.MountedMemSpaces == nil {
		node.MountedMemSpaces = make([]*MountedMemSpace, 0)
	}

	for _, mount := range node.memSpaceMounts {
		mounted, err := t.resolveAndMountMemSpace(node, mount)
		if err != nil {
			t.Growth.Record("mount", node.Name,
				fmt.Sprintf("⚠ Mount failed (source=%s): %v (non-fatal)", mount.Source, err), t)
			continue
		}
		node.MountedMemSpaces = append(node.MountedMemSpaces, mounted)
		t.bindAllAgentsToMountedMemSpace(node, mounted)

		t.Growth.Record("mount", node.Name,
			fmt.Sprintf("Mounted %d@%s (source=%s)", mounted.MemSpaceID, mounted.Addr, mounted.Source), t)
	}

	// ============ Phase 6: Done ============
	node.Status = NodeStatusReady
	return nil
}

// bindAgentToMemSpace does point-to-point binding with exponential backoff retry.
// Retry is needed because MemSpace/Agent processes are launched asynchronously
// and may not be ready when binding is attempted.
func (t *ViewSpaceTree) bindAgentToMemSpace(
	agentID uint64, agentAddr string, role string,
	memSpaceID uint64, memSpaceAddr string, msType string,
) error {
	maxRetry := 5
	baseDelay := 1 * time.Second // 1s, 2s, 4s, 8s, 16s

	var lastErr error

	for attempt := 1; attempt <= maxRetry; attempt++ {
		// Step 1: Tell Agent to bind
		agentClient := client.NewAgentClient(agentAddr)
		bindReq := &api.BindMemSpaceRequest{
			MemSpaceID: strconv.FormatUint(memSpaceID, 10),
			AgentID:    agentID,
			Type:       msType,
			HttpAddr:   memSpaceAddr,
		}

		err := agentClient.BindMemSpace(bindReq)
		if err == nil {
			// Step 2: Tell MemSpace to register agent (CommRegion)
			msClient := client.NewMemSpaceClient(memSpaceAddr)
			if regErr := msClient.RegisterAgent(agentID, agentAddr, role); regErr != nil {
				log.Warnf("[bind] register agent %d in memspace %d comm region failed: %v (non-fatal)",
					agentID, memSpaceID, regErr)
			}

			if attempt > 1 {
				log.Infof("[bind] agent %d → memspace %d succeeded on attempt %d",
					agentID, memSpaceID, attempt)
			}
			return nil
		} else {
			log.Errorf("error occur in binding process! %v", err)

		}

		lastErr = err
		delay := baseDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s, 8s, 16s

		log.Warnf("[bind] agent %d → memspace %d attempt %d/%d failed: %v (retry in %s)",
			agentID, memSpaceID, attempt, maxRetry, err, delay)

		time.Sleep(delay)
	}

	return fmt.Errorf("agent %d bind to memspace %d failed after %d attempts: %w",
		agentID, memSpaceID, maxRetry, lastErr)
}

// bindAllAgentsToMountedMemSpace binds represented agent + all workers to a mounted memspace
func (t *ViewSpaceTree) bindAllAgentsToMountedMemSpace(node *ViewSpaceNode, mounted *MountedMemSpace) {
	msType := "public"
	if mounted.MountType == "readonly" {
		msType = "public" // readonly is enforced at application level, not storage level
	}

	// Bind represented agent
	if err := t.bindAgentToMemSpace(
		node.AgentID, node.AgentAddr, node.Role,
		mounted.MemSpaceID, mounted.Addr, msType,
	); err != nil {
		log.Warnf("[bringToLife] bind agent %d to mounted memspace %d: %v",
			node.AgentID, mounted.MemSpaceID, err)
	}

	// Bind all workers
	for _, w := range node.Workers {
		if err := t.bindAgentToMemSpace(
			w.AgentID, w.AgentAddr, w.Role,
			mounted.MemSpaceID, mounted.Addr, msType,
		); err != nil {
			log.Warnf("[bringToLife] bind worker %d to mounted memspace %d: %v",
				w.AgentID, mounted.MemSpaceID, err)
		}
	}
}

// resolveAndMountMemSpace 解析 MemSpace 来源并建立连接。
func (t *ViewSpaceTree) resolveAndMountMemSpace(node *ViewSpaceNode, mount MemSpaceMount) (*MountedMemSpace, error) {
	switch {
	case mount.Source == "parent":
		// Inherit parent's primary MemSpace
		if node.Parent == nil {
			return nil, fmt.Errorf("cannot mount 'parent': node has no parent")
		}
		mountType := "inherited"
		if mount.ReadOnly {
			mountType = "readonly"
		}
		return &MountedMemSpace{
			MemSpaceID: node.Parent.MemSpaceID,
			Addr:       node.Parent.MemSpaceAddr,
			MountType:  mountType,
			Source:     fmt.Sprintf("parent:%s", node.Parent.Name),
			Client:     client.NewMemSpaceClient(node.Parent.MemSpaceAddr),
		}, nil
	case mount.Source == "new":
		msID, msAddr, err := t.launchMemSpaceWithRetry(
			node,
			fmt.Sprintf("ms-%s-aux", node.Name),
			"public",
			fmt.Sprintf("Auxiliary memspace for '%s': %s", node.Name, mount.Purpose),
		)
		if err != nil {
			return nil, fmt.Errorf("launch new memspace: %w", err)
		}

		mountType := "shared"
		if mount.ReadOnly {
			mountType = "readonly"
		}

		return &MountedMemSpace{
			MemSpaceID: msID,
			Addr:       msAddr,
			MountType:  mountType,
			Source:     "new",
			Client:     client.NewMemSpaceClient(msAddr),
		}, nil

	case len(mount.Source) > 3 && mount.Source[:3] == "id:":
		// Mount existing MemSpace by ID, e.g. "id:12345"
		idStr := mount.Source[3:]
		targetID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid memspace id '%s': %w", idStr, err)
		}

		memspaces, err := t.MemSpaceMgrClient.ListMemSpaces()
		if err != nil {
			return nil, fmt.Errorf("list memspaces for id lookup: %w", err)
		}

		for _, ms := range memspaces {
			msID, _ := strconv.ParseUint(ms.MemSpaceID, 10, 64)
			if msID == targetID {
				mountType := "shared"
				if mount.ReadOnly {
					mountType = "readonly"
				}
				return &MountedMemSpace{
					MemSpaceID: msID,
					Addr:       ms.HttpAddr,
					MountType:  mountType,
					Source:     mount.Source,
					Client:     client.NewMemSpaceClient(ms.HttpAddr),
				}, nil
			}
		}
		return nil, fmt.Errorf("memspace with id %d not found", targetID)

	case len(mount.Source) > 5 && mount.Source[:5] == "name:":
		// Mount existing MemSpace by name, e.g. "name:shared-knowledge"
		targetName := mount.Source[5:]

		memspaces, err := t.MemSpaceMgrClient.ListMemSpaces()
		if err != nil {
			return nil, fmt.Errorf("list memspaces for name lookup: %w", err)
		}

		for _, ms := range memspaces {
			if ms.Name == targetName {
				msID, _ := strconv.ParseUint(ms.MemSpaceID, 10, 64)
				mountType := "shared"
				if mount.ReadOnly {
					mountType = "readonly"
				}
				return &MountedMemSpace{
					MemSpaceID: msID,
					Addr:       ms.HttpAddr,
					MountType:  mountType,
					Source:     mount.Source,
					Client:     client.NewMemSpaceClient(ms.HttpAddr),
				}, nil
			}
		}
		return nil, fmt.Errorf("memspace with name '%s' not found", targetName)

	default:
		return nil, fmt.Errorf("unknown mount source: '%s' (valid: 'parent', 'new', 'id:<num>', 'name:<str>')", mount.Source)
	}
}

// bindAllAgentsToMemSpace binds the represented agent and all workers to a memspace
func (t *ViewSpaceTree) bindAllAgentsToMemSpace(node *ViewSpaceNode, mounted *MountedMemSpace) {
	// Bind represented agent
	if err := t.MemSpaceMgrClient.BindMemSpace(
		node.AgentID, mounted.MemSpaceID, node.AgentAddr, node.Role,
	); err != nil {
		log.Warnf("[bringToLife] failed to bind agent %d to mounted memspace %d: %v",
			node.AgentID, mounted.MemSpaceID, err)
	}

	// Bind all workers
	for _, w := range node.Workers {
		if err := t.MemSpaceMgrClient.BindMemSpace(
			w.AgentID, mounted.MemSpaceID, w.AgentAddr, w.Role,
		); err != nil {
			log.Warnf("[bringToLife] failed to bind worker %d to mounted memspace %d: %v",
				w.AgentID, mounted.MemSpaceID, err)
		}
	}
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
func (t *ViewSpaceTree) shutdownNode(node *ViewSpaceNode) {
	// 先递归关闭子节点
	for _, child := range node.Children {
		t.shutdownNode(child)
	}

	// 关闭 Worker Agents
	for _, w := range node.Workers {
		if w.AgentID != 0 {
			t.AgentMgrClient.StopAgent(w.AgentID)
			log.Infof("[Shutdown] '%s' worker %d destroyed", node.Name, w.AgentID)
		}
	}

	// 关闭 Represented Agent
	if node.AgentID != 0 {
		t.AgentMgrClient.StopAgent(node.AgentID)
	}

	// 关闭额外创建的 MemSpace（不关闭 inherited/tag 的，因为它们属于别人）
	for _, m := range node.MountedMemSpaces {
		if m.MountType == "shared" && m.Source == "new" {
			t.MemSpaceMgrClient.ShutdownMemSpace(m.MemSpaceID)
			log.Infof("[Shutdown] '%s' aux memspace %d destroyed", node.Name, m.MemSpaceID)
		}
	}

	// 关闭主 MemSpace
	if node.MemSpaceID != 0 {
		t.MemSpaceMgrClient.ShutdownMemSpace(node.MemSpaceID)
	}

	log.Infof("[Shutdown] '%s' destroyed", node.Name)
}

// injectDefaultMounts ensures every child node has access to parent context
// This is a system-level policy: children always inherit parent's MemSpace (read-only)
func (t *ViewSpaceTree) injectDefaultMounts(child *ViewSpaceNode) {
	if child.Parent == nil {
		return
	}

	// Check if parent mount already declared by LLM
	for _, m := range child.memSpaceMounts {
		if m.Source == "parent" {
			return // already has it, don't duplicate
		}
	}

	// Auto-inject read-only parent mount
	child.memSpaceMounts = append(child.memSpaceMounts, MemSpaceMount{
		Source:   "parent",
		Purpose:  "Inherit parent context and task description",
		ReadOnly: true,
	})
}

const (
	maxLaunchRetry = 5
)

// launchMemSpaceWithRetry attempts to launch a MemSpace, retrying with new ports on failure
func (t *ViewSpaceTree) launchMemSpaceWithRetry(node *ViewSpaceNode, name, msType, description string) (uint64, string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxLaunchRetry; attempt++ {
		cfg := t.LaunchConfig
		msPort := cfg.AllocPort()
		msAddr := fmt.Sprintf("localhost:%d", msPort)
		msID := uint64(time.Now().UnixNano() % 1000000)

		msConfigPath, err := GenerateMemSpaceConfig(
			msID, name, msType, description,
			msAddr, cfg.PdAddr, cfg.EmbeddingAddr, cfg.LightModelAddr,
		)
		if err != nil {
			lastErr = fmt.Errorf("generate config: %w", err)
			continue
		}

		msReq := &api.LaunchMemSpaceRequestManager{
			BinPath:        cfg.MemSpaceBinPath,
			ConfigFilePath: msConfigPath,
		}

		if err := t.MemSpaceMgrClient.LaunchMemSpace(msReq); err != nil {
			lastErr = err
			log.Warnf("[launch] MemSpace '%s' attempt %d/%d failed on port %d: %v",
				name, attempt, maxLaunchRetry, msPort, err)
			continue
		}

		log.Infof("[launch] MemSpace '%s' launched → %d@%s (attempt %d)",
			name, msID, msAddr, attempt)
		return msID, msAddr, nil
	}

	return 0, "", fmt.Errorf("launch memspace '%s' failed after %d attempts: %w",
		name, maxLaunchRetry, lastErr)
}

// launchAgentWithRetry attempts to launch an Agent, retrying with new ports on failure
func (t *ViewSpaceTree) launchAgentWithRetry(role string) (uint64, string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxLaunchRetry; attempt++ {
		cfg := t.LaunchConfig
		agentPort := cfg.AllocPort()
		agentAddr := fmt.Sprintf("localhost:%d", agentPort)
		agentID := uint64(time.Now().UnixNano() % 1000000)

		agentConfigPath, err := GenerateAgentConfig(
			agentID, role, agentAddr,
			cfg.LightModelAddr, cfg.EmbeddingAddr,
			t.MemSpaceMgrClient.GetBaseURL(),
			t.AgentMgrClient.GetBaseURL(),
			false,
		)
		if err != nil {
			lastErr = fmt.Errorf("generate config: %w", err)
			continue
		}

		agentReq := &api.LaunchAgentRequestHTTP{
			BinPath:        cfg.AgentBinPath,
			ConfigFilePath: agentConfigPath,
		}

		agentResp, err := t.AgentMgrClient.LaunchAgent(agentReq)
		if err != nil {
			lastErr = err
			log.Warnf("[launch] Agent '%s' attempt %d/%d failed on port %d: %v",
				role, attempt, maxLaunchRetry, agentPort, err)
			continue
		}

		actualAddr := agentAddr
		if agentResp.HttpAddr != "" {
			actualAddr = agentResp.HttpAddr
		}

		log.Infof("[launch] Agent '%s' launched → %d@%s (attempt %d)",
			role, agentResp.AgentID, actualAddr, attempt)
		return agentResp.AgentID, actualAddr, nil
	}

	return 0, "", fmt.Errorf("launch agent '%s' failed after %d attempts: %w",
		role, maxLaunchRetry, lastErr)
}
