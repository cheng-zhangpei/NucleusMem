// pkg/runtime/agent/observation_enhancer.go

package agent

import (
	"fmt"
	"regexp"
	"strings"
)

// ObservationResult 增强后的Observation
type ObservationResult struct {
	Raw        string   // 原始输出
	Tags       []string // 标签：[PIDS: xxx] [ENCRYPTION DETECTED] ...
	Highlights []string // 关键发现摘要
	Enhanced   string   // 最终内容（原始 + 标签 + 高亮）
}

// enhanceObservation 对命令输出做轻量级信息提取
func enhanceObservation(cmd string, rawOutput string) *ObservationResult {
	result := &ObservationResult{
		Raw: rawOutput,
	}

	if rawOutput == "" {
		return result
	}

	// 按命令类型做专项提取
	extractPIDs(cmd, rawOutput, result)
	extractEncryptionStatus(cmd, rawOutput, result)
	extractPremountInfo(cmd, rawOutput, result)
	extractSensitiveContent(cmd, rawOutput, result)
	extractKernelInfo(cmd, rawOutput, result)
	extractProcessInfo(cmd, rawOutput, result)
	extractNetworkInfo(cmd, rawOutput, result)
	extractFileInfo(cmd, rawOutput, result)
	extractProcAccess(cmd, rawOutput, result)
	extractExitCode(rawOutput, result)

	// 拼装最终输出
	var b strings.Builder
	b.WriteString(rawOutput)
	if len(result.Tags) > 0 {
		b.WriteString("\n\n[TAGS] ")
		b.WriteString(strings.Join(result.Tags, " | "))
	}
	if len(result.Highlights) > 0 {
		b.WriteString("\n[HIGHLIGHTS] ")
		b.WriteString(strings.Join(result.Highlights, "; "))
	}
	result.Enhanced = b.String()

	return result
}

// === 各项提取函数 ===

func extractPIDs(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "pgrep", "ps ", "pidof") {
		return
	}
	// pgrep输出格式：纯数字 或 "PID CMD"
	lines := strings.Split(output, "\n")
	var pids []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 匹配纯数字行（pgrep默认输出）
		if matched, _ := regexp.MatchString(`^\d+$`, line); matched {
			pids = append(pids, line)
			continue
		}
		// 匹配 "PID CMD" 格式（pgrep -a）
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			if matched, _ := regexp.MatchString(`^\d+$`, fields[0]); matched {
				pids = append(pids, fields[0])
			}
		}
	}
	if len(pids) > 0 {
		r.Tags = append(r.Tags, fmt.Sprintf("[PIDS: %s]", strings.Join(pids, ",")))
	}

	// 检测bwrap进程
	if strings.Contains(output, "bwrap") {
		r.Tags = append(r.Tags, "[SANDBOX: bwrap found]")
	}
	if strings.Contains(output, "Xephyr") {
		r.Tags = append(r.Tags, "[DISPLAY: Xephyr found]")
	}
	if strings.Contains(output, "init.sh") {
		r.Tags = append(r.Tags, "[SANDBOX: init.sh found]")
	}
}

func extractEncryptionStatus(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "dmsetup", "cryptsetup", "blkid", "file", "/proc/crypto") {
		return
	}
	if containsAny(output, "crypt", "LUKS", "crypto", "aes", "sm4", "twofish") {
		r.Tags = append(r.Tags, "[ENCRYPTION DETECTED]")
		r.Highlights = append(r.Highlights, "Encryption mechanism found in output")
	}
	if strings.Contains(output, "NOT_LUKS") || strings.Contains(output, "not a valid") {
		r.Tags = append(r.Tags, "[NO LUKS]")
	}
	if strings.Contains(output, "No devices found") {
		r.Tags = append(r.Tags, "[NO DM-CRYPT]")
	}
}

func extractPremountInfo(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "tc_data_premount", "tc_work") {
		return
	}
	if !strings.Contains(output, "No such file") && !strings.Contains(output, "cannot access") {
		r.Tags = append(r.Tags, "[PREMOUNT FOUND]")
		// 提取premount路径
		re := regexp.MustCompile(`/tmp/tc_data_premount\.\S+`)
		matches := re.FindAllString(output, -1)
		if len(matches) > 0 {
			r.Tags = append(r.Tags, fmt.Sprintf("[PREMOUNT_PATH: %s]", matches[0]))
		}
	}
	if strings.Contains(output, "tc_work") && !strings.Contains(output, "No such file") {
		r.Tags = append(r.Tags, "[WORK_DIR FOUND]")
	}
}

func extractSensitiveContent(cmd, output string, r *ObservationResult) {
	lower := strings.ToLower(output)
	sensitivePatterns := []struct {
		pattern string
		tag     string
	}{
		{"password", "PASSWORD"},
		{"secret", "SECRET"},
		{"key", "KEY"},
		{"token", "TOKEN"},
		{"private", "PRIVATE_KEY"},
		{"credential", "CREDENTIAL"},
		{"sm4", "SM4"},
		{"aes", "AES"},
		{"md5", "MD5"},
		{"sha256", "SHA256"},
		{"tc_device_pwd", "DEVICE_PWD"},
		{"premount_key", "PREMOUNT_KEY"},
		{"/home/sandbox/secureusb", "SECUREUSB"},
	}

	for _, sp := range sensitivePatterns {
		if strings.Contains(lower, sp.pattern) {
			r.Tags = append(r.Tags, fmt.Sprintf("[SENSITIVE: %s]", sp.tag))
			r.Highlights = append(r.Highlights, fmt.Sprintf("Sensitive content detected: %s", sp.tag))
			break // 只标记一个，避免太吵
		}
	}
}

func extractKernelInfo(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "uname", "/proc/version") {
		return
	}
	// 提取内核版本
	re := regexp.MustCompile(`\d+\.\d+\.\d+[-\w]*`)
	match := re.FindString(output)
	if match != "" {
		r.Tags = append(r.Tags, fmt.Sprintf("[KERNEL: %s]", match))
	}
}

func extractProcessInfo(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "ps ", "pgrep", "top", "pstree") {
		return
	}
	if strings.Contains(output, "bwrap") {
		r.Tags = append(r.Tags, "[SANDBOX_PROCESS: running]")
	} else {
		r.Tags = append(r.Tags, "[SANDBOX_PROCESS: not found]")
	}
}

func extractNetworkInfo(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "netstat", "ss ", "nmap", "ifconfig", "ip addr") {
		return
	}
	if strings.Contains(output, "LISTEN") {
		r.Tags = append(r.Tags, "[NETWORK: listening ports found]")
	}
	if strings.Contains(output, "ESTABLISHED") {
		r.Tags = append(r.Tags, "[NETWORK: active connections]")
	}
}

func extractFileInfo(cmd, output string, r *ObservationResult) {
	if !containsAny(cmd, "cat ", "head ", "tail ", "xxd ", "strings ") {
		return
	}
	// 检测是否成功读到了文件内容（不是报错）
	if !containsAny(output, "Permission denied", "No such file", "cannot access") {
		if len(strings.TrimSpace(output)) > 10 {
			r.Tags = append(r.Tags, "[FILE_READ: success]")
		}
	}
}

func extractProcAccess(cmd, output string, r *ObservationResult) {
	if !strings.Contains(cmd, "/proc/") {
		return
	}
	// 检测通过/proc访问sandbox文件
	if strings.Contains(cmd, "root") && !strings.Contains(output, "Permission denied") {
		r.Tags = append(r.Tags, "[PROC_ROOT: accessible]")
	}
	if strings.Contains(cmd, "environ") && !containsAny(output, "Permission denied", "No such process") {
		r.Tags = append(r.Tags, "[PROC_ENV: readable]")
	}
	if strings.Contains(cmd, "fd") && strings.Contains(output, "->") {
		r.Tags = append(r.Tags, "[PROC_FD: links found]")
	}
	if strings.Contains(cmd, "maps") && strings.Contains(output, "rw-p") {
		r.Tags = append(r.Tags, "[PROC_MEM: rw regions found]")
	}
}

func extractExitCode(output string, r *ObservationResult) {
	// 输出格式通常是 "[exit_code=N]" 或命令本身返回的
	re := regexp.MustCompile(`$$exit_code=(\d+)$$`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		code := matches[1]
		if code != "0" {
			r.Tags = append(r.Tags, fmt.Sprintf("[EXIT: %s]", code))
		}
	}
	// 检测常见错误模式
	if strings.Contains(output, "Permission denied") {
		r.Tags = append(r.Tags, "[ERROR: permission_denied]")
	}
	if strings.Contains(output, "No such file") {
		r.Tags = append(r.Tags, "[ERROR: file_not_found]")
	}
	if strings.Contains(output, "Operation not permitted") {
		r.Tags = append(r.Tags, "[ERROR: operation_not_permitted]")
	}
	if strings.Contains(output, "command not found") {
		r.Tags = append(r.Tags, "[ERROR: command_not_found]")
	}
}

// 工具函数
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
