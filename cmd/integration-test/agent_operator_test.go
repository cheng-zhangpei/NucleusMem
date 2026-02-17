package test

import (
	"context"
	"testing"
	"time"

	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentManagerClient_LaunchAgent tests the LaunchAgent functionality
func TestAgentManagerClient_LaunchAgent(t *testing.T) {
	// Create client
	client := client.NewAgentManagerClient("http://localhost:7007")

	// Prepare launch request
	req := &api.LaunchAgentRequestHTTP{
		Role:           "TestAgent",
		Image:          "nucleus:test",
		BinPath:        "./bin/agent.exe", // Windows path
		IsJob:          false,
		ConfigFilePath: "./pkg/configs/file/agent_101.yaml",
	}

	// Launch agent
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.LaunchAgentWithContext(ctx, req)
	require.NoError(t, err, "LaunchAgent should succeed")
	assert.True(t, resp.Success, "Response should indicate success")

	// Optional: Verify agent is actually running
	// You can add additional verification here if needed
	t.Logf("✅ Successfully launched Agent %d", resp.AgentID)
}
