package tool_executors

import (
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
	"os/exec"
	_ "strings"
	"time"
)

type ShellExecutor struct{}

func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

func (s *ShellExecutor) Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	// 从params中提取command
	command, ok := params["command"].(string)
	if !ok || command == "" {
		// fallback: 尝试 "query" 字段
		command, _ = params["query"].(string)
	}
	if command == "" {
		return nil, fmt.Errorf("shell executor: 'command' parameter is required")
	}

	// 安全检查：禁止危险命令（可选，攻击测试时可以注释掉）
	// dangerous := []string{"rm -rf", "mkfs", "dd if=/dev/zero"}
	// for _, d := range dangerous {
	//     if strings.Contains(command, d) {
	//         return nil, fmt.Errorf("blocked dangerous command: %s", d)
	//     }
	// }

	timeout := 30 * time.Second
	if tool.TimeoutSeconds > 0 {
		timeout = time.Duration(tool.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	result := map[string]interface{}{
		"command":   command,
		"output":    string(output),
		"truncated": false,
	}

	// 截断过长输出，防止token爆掉
	outputStr := string(output)
	if len(outputStr) > 4096 {
		result["output"] = outputStr[:4096] + "\n... [truncated, total bytes: " + fmt.Sprintf("%d", len(outputStr)) + "]"
		result["truncated"] = true
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else {
			// timeout 或其他错误
			result["exit_code"] = -1
			result["error"] = err.Error()
		}
	} else {
		result["exit_code"] = 0
	}

	return result, nil
}
