package standard_executor

import (
	"NucleusMem/pkg/configs"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"text/template"
)

type ShellStandardExecutor struct{}

func NewShellStandardExecutor() *ShellStandardExecutor {
	return &ShellStandardExecutor{}
}
func (e *ShellStandardExecutor) Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	cfg := tool.ShellConfig
	if cfg == nil {
		return nil, fmt.Errorf("shell config is missing")
	}

	// 1. 渲染命令模板
	tmpl, err := template.New("cmd").Parse(cfg.CommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command template: %w", err)
	}

	var cmdBuf bytes.Buffer
	if err := tmpl.Execute(&cmdBuf, params); err != nil {
		return nil, fmt.Errorf("failed to execute command template: %w", err)
	}
	cmdStr := cmdBuf.String()

	// 2. 执行命令
	// 注意：生产环境需要严格限制 WorkDir 和权限，防止注入攻击
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "."
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
			"stderr": stderr.String(),
		}, err
	}

	return map[string]interface{}{
		"status": "success",
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}, nil
}
