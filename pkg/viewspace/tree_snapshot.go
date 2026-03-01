// pkg/viewspace/tree_snapshot.go

package viewspace

import (
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"strings"
)

func (t *ViewSpaceTree) PrintSnapshot() {
	if t.Root == nil {
		return
	}

	lines := []string{
		"",
		"🌳 ViewSpaceTree Snapshot",
	}

	lines = append(lines, snapshotNode(t.Root, "", true)...)

	// 统计
	g, p, a := t.GetNodeCount()
	total := g + p + a
	msCount := total // 每个节点一个primary memspace
	mountCount := countMounts(t.Root)

	lines = append(lines, fmt.Sprintf(
		"\n  📊 Total: %d nodes | %d agents | %d memspaces | %d mounts",
		total, total, msCount, mountCount,
	))

	log.Infof(strings.Join(lines, "\n"))
}

func snapshotNode(node *ViewSpaceNode, prefix string, isLast bool) []string {
	connector := "├── "
	childPrefix := prefix + "│     "
	if isLast {
		connector = "└── "
		childPrefix = prefix + "      "
	}

	statusIcon := "✅"
	if node.Status == NodeStatusFailed {
		statusIcon = "❌"
	} else if node.Status == NodeStatusGrowing {
		statusIcon = "⏳"
	}

	lines := []string{
		fmt.Sprintf("%s%s[%s] %s  agent:%d@%s  ms:%d@%s  %s",
			prefix, connector,
			node.Type, node.Name,
			node.AgentID, node.AgentAddr,
			node.MemSpaceID, node.MemSpaceAddr,
			statusIcon,
		),
	}

	// tools
	if len(node.Tools) > 0 {
		lines = append(lines, fmt.Sprintf("%s🔧 tools:  %v", childPrefix, node.Tools))
	}

	// workers
	if len(node.Workers) > 0 {
		workerDescs := make([]string, len(node.Workers))
		for i, w := range node.Workers {
			workerDescs[i] = fmt.Sprintf("%d(%s)", w.AgentID, w.Role)
		}
		lines = append(lines, fmt.Sprintf("%s👥 workers: [%s]", childPrefix, strings.Join(workerDescs, ", ")))
	}

	// mounts
	if len(node.MountedMemSpaces) > 0 {
		mountDescs := make([]string, len(node.MountedMemSpaces))
		for i, m := range node.MountedMemSpaces {
			mountDescs[i] = fmt.Sprintf("%d@%s %s", m.MemSpaceID, m.Addr, m.MountType)
		}
		lines = append(lines, fmt.Sprintf("%s📎 mounts: [%s]", childPrefix, strings.Join(mountDescs, ", ")))
	}

	// 递归子节点
	node.mu.RLock()
	children := make([]*ViewSpaceNode, len(node.Children))
	copy(children, node.Children)
	node.mu.RUnlock()

	for i, child := range children {
		childLines := snapshotNode(child, prefix+"    ", i == len(children)-1)
		lines = append(lines, childLines...)
	}

	return lines
}

func countMounts(node *ViewSpaceNode) int {
	if node == nil {
		return 0
	}
	count := len(node.MountedMemSpaces)
	for _, child := range node.Children {
		count += countMounts(child)
	}
	return count
}
