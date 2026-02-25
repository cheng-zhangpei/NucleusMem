package test_utils

import "NucleusMem/pkg/configs"

// MockLintToolDefinition returns the mock_lint tool definition
func MockLintToolDefinition(endpoint string) *configs.ToolDefinition {
	return &configs.ToolDefinition{
		Name:        "mock_lint",
		Description: "A mock linter tool that checks code for violations",
		Tags:        []string{"static-analysis", "code-tools", "test"},
		Parameters: []configs.ToolParam{
			{
				Name:     "path",
				Type:     "string",
				Required: true,
				Default:  "",
			},
			{
				Name:     "config",
				Type:     "string",
				Required: false,
				Default:  ".eslintrc",
			},
		},
		ReturnType: "object",
		Endpoint:   endpoint,
		ExecType:   "http",
		Metadata: map[string]string{
			"version": "1.0.0",
			"author":  "test",
		},
		CreatedAt: 0,
	}
}
