package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AttackReport 单次攻击报告（存入MemSpace的单元）
type AttackReport struct {
	RunID        string    `json:"run_id"`
	Timestamp    time.Time `json:"timestamp"`
	Query        string    `json:"query"`
	KernelInfo   string    `json:"kernel_info"`
	SandboxInfo  string    `json:"sandbox_info"`
	TotalSteps   int       `json:"total_steps"`
	SuccessLevel string    `json:"success_level"` // "full" / "partial" / "failed"

	// 关键发现（最重要的部分，下次运行直接用）
	Findings []AttackFinding `json:"findings"`

	// 失败路径（下次不要重复尝试）
	DeniedPaths []DeniedPath `json:"denied_paths"`

	// 未完成的任务（下次可以接着做）
	PendingTasks []string `json:"pending_tasks"`

	// 本次攻击的结论
	Conclusion string `json:"conclusion"`

	// 下次建议的攻击方向
	NextSuggestion string `json:"next_suggestion"`
}

// AttackFinding 单条发现
type AttackFinding struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // "recon", "encryption", "process", "filesystem", "network"
	Title       string `json:"title"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"` // 命令输出证据
	Severity    string `json:"severity"` // "critical", "high", "medium", "low", "info"
	StepNumber  int    `json:"step_number"`
	Exploitable bool   `json:"exploitable"`
	ExploitHint string `json:"exploit_hint"` // 如果可利用，怎么利用
}

// DeniedPath 被拒绝的路径
type DeniedPath struct {
	Command string `json:"command"`
	Reason  string `json:"reason"` // "permission_denied", "file_not_found", "not_applicable"
	Times   int    `json:"times"`  // 被拒次数
}

// FormatForNextRun 格式化为下次运行的prompt注入内容
func (r *AttackReport) FormatForNextRun() string {
	var b string
	b += fmt.Sprintf("### Previous Run [%s] (RunID: %s)\n", r.Timestamp.Format("2006-01-02 15:04"), r.RunID)
	b += fmt.Sprintf("- Query: %s\n", r.Query)
	b += fmt.Sprintf("- Steps: %d, Result: %s\n", r.TotalSteps, r.SuccessLevel)
	b += fmt.Sprintf("- Kernel: %s\n", r.KernelInfo)

	if len(r.Findings) > 0 {
		b += "\n**Confirmed Findings:**\n"
		for _, f := range r.Findings {
			b += fmt.Sprintf("- [%s] %s: %s\n", f.Severity, f.Title, f.Description)
			if f.Exploitable {
				b += fmt.Sprintf("  → Exploitable: %s\n", f.ExploitHint)
			}
		}
	}

	if len(r.DeniedPaths) > 0 {
		b += "\n**Do NOT retry these:**\n"
		for _, d := range r.DeniedPaths {
			b += fmt.Sprintf("- `%s` (failed %d times: %s)\n", d.Command, d.Times, d.Reason)
		}
	}

	if len(r.PendingTasks) > 0 {
		b += "\n**Unfinished tasks:**\n"
		for _, t := range r.PendingTasks {
			b += fmt.Sprintf("- %s\n", t)
		}
	}

	if r.NextSuggestion != "" {
		b += fmt.Sprintf("\n**Suggested next direction:** %s\n", r.NextSuggestion)
	}

	b += "\n"
	return b
}

// Serialize 序列化为JSON
func (r *AttackReport) Serialize() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeserializeReport 反序列化
func DeserializeReport(data string) (*AttackReport, error) {
	report := &AttackReport{}
	if err := json.Unmarshal([]byte(data), report); err != nil {
		return nil, err
	}
	return report, nil
}

// 辅助函数

func categorizeCommand(cmd string) string {
	switch {
	case strings.Contains(cmd, "pgrep") || strings.Contains(cmd, "ps "):
		return "process"
	case strings.Contains(cmd, "ls ") || strings.Contains(cmd, "find ") || strings.Contains(cmd, "mount"):
		return "filesystem"
	case strings.Contains(cmd, "cat /proc") || strings.Contains(cmd, "readlink"):
		return "proc"
	case strings.Contains(cmd, "dmsetup") || strings.Contains(cmd, "crypt"):
		return "encryption"
	case strings.Contains(cmd, "gdb") || strings.Contains(cmd, "strace"):
		return "memory"
	case strings.Contains(cmd, "uname") || strings.Contains(cmd, "kernel"):
		return "kernel"
	default:
		return "other"
	}
}

func estimateSeverity(cmd, output string) string {
	// 包含敏感数据
	if strings.Contains(output, "root:") || strings.Contains(output, "password") {
		return "critical"
	}
	// 直接可读文件
	if strings.Contains(cmd, "cat") && !strings.Contains(output, "Permission denied") {
		return "high"
	}
	// 信息泄露
	if strings.Contains(output, "bwrap") || strings.Contains(output, "tc_") {
		return "medium"
	}
	return "info"
}

func estimateExploitable(cmd, output string) bool {
	// premount目录可读
	if strings.Contains(cmd, "tc_data_premount") && !strings.Contains(output, "No such file") {
		return true
	}
	// 环境变量含密钥
	if strings.Contains(cmd, "environ") && strings.Contains(output, "PWD") {
		return true
	}
	// fd泄漏
	if strings.Contains(cmd, "fd/") && strings.Contains(output, "/proc") {
		return true
	}
	return false
}

func evaluateSuccessLevel(state *ReActState, finalAnswer string) string {
	text := strings.ToLower(finalAnswer)
	if strings.Contains(text, "success") || strings.Contains(text, "extracted") {
		return "full"
	}
	if len(state.KnowledgeBase) > 3 {
		return "partial"
	}
	return "failed"
}

func generateNextSuggestion(state *ReActState, finalAnswer string) string {
	phase := state.AttackPhase
	switch phase {
	case "recon":
		return "Try direct read of premount directories or /proc/PID/root access"
	case "direct_read":
		return "Check encryption status, try memory dump if encrypted"
	case "encryption_check":
		return "Attempt memory dump via gdb to extract encryption keys"
	case "memory_dump":
		return "Try strace to monitor file access, or nsenter namespace"
	default:
		return "Review findings and try alternative approaches"
	}
}
func extractTitle(cmd string) string {
	switch {
	case strings.Contains(cmd, "pgrep"):
		return "Process Discovery"
	case strings.Contains(cmd, "ps aux"):
		return "Process Enumeration"
	case strings.Contains(cmd, "uname"):
		return "Kernel Version Check"
	case strings.Contains(cmd, "/proc/version"):
		return "Kernel Info via /proc"
	case strings.Contains(cmd, "lsblk"):
		return "Block Device Enumeration"
	case strings.Contains(cmd, "mount"):
		return "Mount Point Discovery"
	case strings.Contains(cmd, "tc_data_premount"):
		return "Premount Directory Access"
	case strings.Contains(cmd, "tc_work"):
		return "Work Directory Discovery"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "root"):
		return "Proc Root Filesystem Access"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "environ"):
		return "Process Environment Variables"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "cmdline"):
		return "Process Command Line"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "fd"):
		return "File Descriptor Leak"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "maps"):
		return "Process Memory Map"
	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "mem"):
		return "Process Memory Dump"
	case strings.Contains(cmd, "dmsetup"):
		return "Device Mapper Status"
	case strings.Contains(cmd, "cryptsetup"):
		return "Disk Encryption Check"
	case strings.Contains(cmd, "blkid"):
		return "Block Device UUID/Type"
	case strings.Contains(cmd, "gdb"):
		return "Debugger Memory Inspection"
	case strings.Contains(cmd, "strace"):
		return "Syscall Tracing"
	case strings.Contains(cmd, "nsenter"):
		return "Namespace Entry"
	case strings.Contains(cmd, "nmap"):
		return "Network Scan"
	case strings.Contains(cmd, "netstat"):
		return "Network Connections"
	case strings.Contains(cmd, "lsof"):
		return "Open Files Inspection"
	case strings.Contains(cmd, "find"):
		return "File Search"
	case strings.Contains(cmd, "cat"):
		return "File Content Read"
	case strings.Contains(cmd, "strings"):
		return "Binary String Extraction"
	case strings.Contains(cmd, "xxd") || strings.Contains(cmd, "hexdump"):
		return "Hex Dump"
	case strings.Contains(cmd, "ls "):
		return "Directory Listing"
	case strings.Contains(cmd, "Xephyr"):
		return "Xephyr Process Check"
	case strings.Contains(cmd, ".X11"):
		return "X11 Socket Inspection"
	default:
		if len(cmd) > 60 {
			return cmd[:60] + "..."
		}
		if cmd == "" {
			return "Unknown Command"
		}
		return cmd
	}
}

func estimateExploitHint(cmd, output string) string {
	switch {
	case strings.Contains(cmd, "tc_data_premount") && !strings.Contains(output, "No such file"):
		return "Data partition files are directly accessible via premount path, no encryption or access control"

	case strings.Contains(cmd, "environ"):
		if strings.Contains(output, "TC_DEVICE_PWD") || strings.Contains(output, "PWD") {
			return "Device password found in environment variable, can decrypt encrypted volumes"
		}
		if strings.Contains(output, "KEY") || strings.Contains(output, "SECRET") {
			return "Encryption key or secret leaked in environment variables"
		}
		return "Process environment variables may contain sensitive configuration data"

	case strings.Contains(cmd, "fd/") && !strings.Contains(output, "No such process"):
		if strings.Contains(output, "/proc") || strings.Contains(output, "/home") || strings.Contains(output, "/etc") {
			return "File descriptor points to host path, potential fd leak for accessing host filesystem"
		}
		return "File descriptors may leak host filesystem paths"

	case strings.Contains(cmd, "/proc/") && strings.Contains(cmd, "root") && !strings.Contains(output, "Permission denied"):
		return "Can read sandbox files via /proc/PID/root from host, bypasses bwrap isolation"

	case strings.Contains(cmd, ".X11"):
		return "X11 socket is shared between host and sandbox, can capture screen or inject keystrokes"

	case strings.Contains(cmd, "Xephyr") && !strings.Contains(output, "No such process"):
		return "Xephyr display server is running, screen capture or input injection possible via X11 protocol"

	case strings.Contains(cmd, "dmsetup") && strings.Contains(output, "crypt"):
		return "Encrypted volume detected, need to extract key from process memory or environment"

	case strings.Contains(cmd, "gdb") && strings.Contains(output, "rw-p"):
		return "Readable/writable memory region found, can dump memory to search for encryption keys"

	case strings.Contains(cmd, "strace") && strings.Contains(output, "openat"):
		return "Syscall trace reveals file access patterns, can identify paths to sensitive data"

	case strings.Contains(cmd, "nmap") || strings.Contains(cmd, "netstat"):
		return "Network services may be accessible for lateral movement or data exfiltration"

	case strings.Contains(cmd, "nsenter") && !strings.Contains(output, "failed"):
		return "Successfully entered sandbox namespace, has direct access to sandbox resources"

	default:
		return ""
	}
}

func guessDenialReason(cmd string, state *ReActState) string {
	for _, step := range state.Steps {
		stepCmd, _ := step.ActionInput["command"].(string)
		if stepCmd != cmd {
			continue
		}
		obs := step.Observation
		switch {
		case strings.Contains(obs, "Permission denied"):
			return "permission_denied"
		case strings.Contains(obs, "No such file") || strings.Contains(obs, "cannot access"):
			return "file_not_found"
		case strings.Contains(obs, "No such process"):
			return "process_not_found"
		case strings.Contains(obs, "Operation not permitted"):
			return "operation_not_permitted"
		case strings.Contains(obs, "command not found"):
			return "tool_not_installed"
		case strings.Contains(obs, "Connection refused") || strings.Contains(obs, "Network is unreachable"):
			return "network_error"
		case strings.Contains(obs, "timeout") || strings.Contains(obs, "timed out"):
			return "timeout"
		case strings.Contains(obs, "not available") || strings.Contains(obs, "not support"):
			return "not_applicable"
		default:
			return "unknown_failure"
		}
	}
	return "unknown_failure"
}
