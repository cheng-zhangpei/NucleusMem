// pkg/viewspace/tree_visual.go

package viewspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

// GrowthEvent records a single step in the tree's growth
type GrowthEvent struct {
	Timestamp time.Time
	Phase     string // "seed" | "grow" | "spawn" | "bringToLife" | "mount" | "worker"
	NodeName  string
	Message   string
	TreeSnap  string // snapshot of tree at this moment
}

// GrowthLog collects all growth events for replay/display
type GrowthLog struct {
	Events []GrowthEvent
}

func (gl *GrowthLog) Record(phase, nodeName, message string, tree *ViewSpaceTree) {
	snap := ""
	if tree != nil && tree.Root != nil {
		snap = tree.PrintTree()
	}
	event := GrowthEvent{
		Timestamp: time.Now(),
		Phase:     phase,
		NodeName:  nodeName,
		Message:   message,
		TreeSnap:  snap,
	}
	gl.Events = append(gl.Events, event)

	// Real-time display
	printGrowthEvent(event, len(gl.Events))
}

func printGrowthEvent(e GrowthEvent, seq int) {
	separator := strings.Repeat("─", 60)
	phaseIcon := map[string]string{
		"seed":        "🌱",
		"grow":        "🌿",
		"decompose":   "🔍",
		"spawn":       "🍃",
		"bringToLife": "⚡",
		"mount":       "📎",
		"worker":      "👷",
		"ready":       "✅",
		"failed":      "❌",
	}

	icon := phaseIcon[e.Phase]
	if icon == "" {
		icon = "📌"
	}

	log.Infof("%s", separator)
	log.Infof("%s  Step %d | %s | [%s]", icon, seq, e.Phase, e.NodeName)
	log.Infof("   %s", e.Message)
	if e.TreeSnap != "" {
		log.Infof("\n   Current Tree:")
		for _, line := range strings.Split(e.TreeSnap, "\n") {
			if line != "" {
				log.Infof("   %s", line)
			}
		}
	}
	log.Infof("%s", separator)
}

// PrintGrowthSummary prints a timeline of all events
func (gl *GrowthLog) PrintGrowthSummary() {
	log.Infof("\n")
	log.Infof("╔══════════════════════════════════════════════════════════╗")
	log.Infof("║              🌳 GROWTH TIMELINE SUMMARY                ║")
	log.Infof("╚══════════════════════════════════════════════════════════╝")

	if len(gl.Events) == 0 {
		log.Infof("  (no events)")
		return
	}

	startTime := gl.Events[0].Timestamp
	for i, e := range gl.Events {
		elapsed := e.Timestamp.Sub(startTime)
		log.Infof("  [%3d] +%8s  %-12s  %-20s  %s",
			i+1,
			formatDuration(elapsed),
			e.Phase,
			e.NodeName,
			e.Message,
		)
	}

	totalTime := gl.Events[len(gl.Events)-1].Timestamp.Sub(startTime)
	log.Infof("\n  Total growth time: %s", formatDuration(totalTime))
	log.Infof("  Total events: %d", len(gl.Events))
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
