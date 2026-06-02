// pkg/runtime/agent/script_executor.go

package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	_ "strings"
	"time"
)

// executeScript 执行Python脚本
func (a *Agent) executeScript(script string, timeout int) string {
	// 写入临时文件
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("attack_%d.py", time.Now().UnixNano()))

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Sprintf("[exit_code=1]\nFailed to write script: %v", err)
	}
	defer os.Remove(scriptPath)

	// 确定python路径
	python := "python3"

	// 执行
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout))
	defer cancel()

	cmd := exec.CommandContext(ctx, python, scriptPath)
	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Sprintf("[exit_code=timeout]\n%s\n[TIMEOUT after %ds]", result, timeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("[exit_code=%d]\n%s", exitErr.ExitCode(), result)
		}
		return fmt.Sprintf("[exit_code=1]\n%s\nError: %v", result, err)
	}
	return fmt.Sprintf("[exit_code=0]\n%s", result)
}

// handleAttackAction 统一处理所有attack action类型
func (a *Agent) handleAttackAction(action string, actionInput map[string]interface{}) string {
	switch action {
	case "exec_cmd":
		return a.executeShellCommandWithTimeout(actionInput, a.attackConfig.TimeoutPerStep)

	case "exec_script":
		script, _ := actionInput["script"].(string)
		if script == "" {
			return "[exit_code=1]\nError: script field is required"
		}
		return a.executeScript(script, a.attackConfig.TimeoutPerStep)

	case "chat":
		query, _ := actionInput["query"].(string)
		resp, err := a.chatClient.QuickChat(query)
		if err != nil {
			return fmt.Sprintf("Chat error: %v", err)
		}
		return resp.Response

	default:
		return fmt.Sprintf("Unknown action: %s. Use exec_cmd, exec_script, or chat.", action)
	}
}
