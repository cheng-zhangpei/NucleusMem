package test

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/monitor/agent_monitor"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestConnectToExternalAgent verifies connecting to a real running agent
func TestConnectToExternalAgent(t *testing.T) {
	// Skip if not running manually (set RUN_CONNECT_TEST=1 to enable)

	// Configuration for real running agent
	const (
		AGENT_ID   = 101
		AGENT_ADDR = "localhost:9001" // Your actual running agent
		MONITOR_ID = uint64(999)      // Test monitor ID
	)
	// Create monitor
	config := &configs.MonitorConfig{NodeID: MONITOR_ID}
	monitor := agent_monitor.NewAgentMonitor(config)
	// Connect to external agent
	err := monitor.ConnectToAgent(AGENT_ID, AGENT_ADDR)
	require.NoError(t, err, "ConnectToAgent should succeed")

	t.Logf("✅ Successfully connected Monitor %d to Agent %d at %s",
		MONITOR_ID, AGENT_ID, AGENT_ADDR)
}
