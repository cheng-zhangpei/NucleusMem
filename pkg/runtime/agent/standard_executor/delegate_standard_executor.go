package standard_executor

import (
	"NucleusMem/pkg/configs"
	"bytes"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"os/exec"
)

type DelegateStandardExecutor struct{}

func NewDelegateStandardExecutor() *DelegateStandardExecutor {
	return &DelegateStandardExecutor{}
}
func (e *DelegateStandardExecutor) Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	cfg := tool.DelegateConfig
	if cfg == nil {
		return nil, fmt.Errorf("delegate config is missing")
	}

	// 1. 提取核心指令
	taskInput, ok := params[cfg.TaskInputField].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid task input field: %s", cfg.TaskInputField)
	}

	// 2. 构建完整 Prompt
	fullPrompt := cfg.InstructionPrefix + "\n\nTask: " + taskInput

	// 3. 确定工作目录 (Sandbox Mode)
	workDir := "."
	if cfg.SandboxMode == "tmp_dir" {
		// 可以创建一个临时目录并清理
		// workDir, _ = os.MkdirTemp("", "nucleus-delegate-*")
		// defer os.RemoveAll(workDir)
	}

	// 4. 执行 Claude Code CLI (假设命令是 'claude')
	// 注意：这里需要根据你实际安装的 Claude Code CLI 调整命令参数
	cmd := exec.CommandContext(ctx, "claude", "--prompt", fullPrompt, "--cwd", workDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Infof("Executing Delegate Agent (Type: %s) with prompt length: %d", cfg.AgentType, len(fullPrompt))

	if err := cmd.Run(); err != nil {
		return map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
			"stderr": stderr.String(),
			"stdout": stdout.String(), // 即使失败也返回部分输出
		}, err
	}

	return map[string]interface{}{
		"status": "success",
		"output": stdout.String(),
		"type":   cfg.AgentType,
	}, nil
}
