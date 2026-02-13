package agent

import (
	"NucleusMem/pkg/configs"
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentHTTPServer_TempChat(t *testing.T) {
	// Create a mock agent config (minimal setup)
	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001", // Mock LLM server address
		IsJob:          false,
	}

	// Create agent instance
	agentInstance, err := NewAgent(config)
	assert.NoError(t, err)

	// Create HTTP server
	server := NewAgentHTTPServer(agentInstance)

	// Test cases
	tests := []struct {
		name          string
		inputMessage  string
		expectSuccess bool
	}{
		{
			name:          "valid message",
			inputMessage:  "Hello, how are you?",
			expectSuccess: true,
		},
		{
			name:          "empty message",
			inputMessage:  "",
			expectSuccess: true, // LLM should handle empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			reqBody := map[string]string{"message": tt.inputMessage}
			jsonBody, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/v1/chat/temp", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Record response
			w := httptest.NewRecorder()
			server.handleTempChat(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			var resp struct {
				Success      bool   `json:"success"`
				Response     string `json:"response"`
				ErrorMessage string `json:"error_message"`
			}
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)

			if tt.expectSuccess {
				assert.True(t, resp.Success)
				assert.NotEmpty(t, resp.Response)
				assert.Empty(t, resp.ErrorMessage)
			} else {
				assert.False(t, resp.Success)
				assert.NotEmpty(t, resp.ErrorMessage)
			}
		})
	}
}

// Test with Job Agent type
func TestAgentHTTPServer_TempChat_JobAgent(t *testing.T) {
	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001",
		IsJob:          true, // Job agent
	}

	agentInstance, err := NewAgent(config)
	assert.NoError(t, err)

	server := NewAgentHTTPServer(agentInstance)

	// Send request
	reqBody := map[string]string{"message": "What is AI?"}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/chat/temp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.handleTempChat(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Response string `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Response)
}

// TestAgent_TempChat_ContextContinuity verifies that the agent maintains conversation context across multiple TempChat calls
func TestAgent_TempChat_ContextContinuity(t *testing.T) {
	// Skip if you don't have a real LLM backend (or use a mock that simulates memory)
	// t.Skip("Skipping due to external LLM dependency")

	config := &configs.AgentConfig{
		ChatServerAddr: "http://localhost:20001", // Your local chat server
		IsJob:          false,
	}

	agent, err := NewAgent(config)
	require.NoError(t, err)

	// Round 1: User introduces themselves
	resp1, err := agent.TempChat("你好！")
	require.NoError(t, err)
	assert.NotEmpty(t, resp1)

	// Round 2: User says their name
	resp2, err := agent.TempChat("我叫小明。")
	require.NoError(t, err)
	assert.NotEmpty(t, resp2)

	// Round 3: Ask the agent to recall the name
	resp3, err := agent.TempChat("我刚才说我的名字是什么？")
	require.NoError(t, err)
	assert.NotEmpty(t, resp3)

	// Verify context awareness (case-insensitive)
	assert.Contains(t, strings.ToLower(resp3), "小明",
		"LLM response should reference the user's name from history")
}
