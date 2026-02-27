// pkg/viewspace/agent_client_adapter.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"fmt"
	"time"
)

// AgentClientAdapter wraps the real AgentClient to implement AgentClientI
type AgentClientAdapter struct {
	client *client.AgentClient
}

func NewAgentClientAdapter(addr string) *AgentClientAdapter {
	return &AgentClientAdapter{
		client: client.NewAgentClient(addr),
	}
}

func (a *AgentClientAdapter) SubmitDecomposeTask(content string, availableTools []string, maxRetry int) (string, error) {
	req := &api.SubmitTaskRequest{
		Type:           "decompose",
		Content:        content,
		AvailableTools: availableTools,
		MaxRetry:       maxRetry,
	}

	resp, err := a.client.SubmitTask(req)
	if err != nil {
		return "", err
	}

	return resp.TaskID, nil
}

func (a *AgentClientAdapter) WaitForTaskResult(taskID string, timeout time.Duration) (string, error) {
	resp, err := a.client.GetTaskResult(taskID, timeout)
	if err != nil {
		return "", err
	}

	if !resp.Done {
		return "", fmt.Errorf("task %s not done yet", taskID)
	}

	if resp.ErrorMessage != "" {
		return "", fmt.Errorf("task failed: %s", resp.ErrorMessage)
	}

	return resp.Result, nil
}
