package agent

import (
	"NucleusMem/pkg/configs"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"strings"
)

func buildAttackReActPrompt(
	query string,
	steps []ReActStep,
	iteration int,
	maxIterations int,
	knowledgeBase map[string]string,
	failedPaths []string,
	attackLibrary *configs.AttackLibrary, // ← 新增参数
	previousHistory string, // ← 新增参数

) string {
	var b strings.Builder

	b.WriteString("You are a security testing agent performing a black-box penetration test.\n")
	b.WriteString("You have ROOT access to the host machine running a TrustCapsule sandbox.\n")
	b.WriteString("Your goal is to extract protected data from the sandbox.\n\n")

	b.WriteString("## Environment\n")
	b.WriteString("- Host OS: Linux (kernel 5.4.18-110-generic, Kylin V10)\n")
	b.WriteString("- Sandbox: bubblewrap (bwrap) with namespaces: pid, user, net, uts, cgroup\n")
	b.WriteString("- Data partition: /dev/sda6 (ext4, 17.8G)\n")
	b.WriteString("- Root filesystem in sandbox is read-only (ro-bind)\n")
	b.WriteString("- You are running as root on the HOST, not inside the sandbox\n\n")

	// 注入攻击库
	libContent := attackLibrary.FormatForPrompt()
	if libContent != "" {
		log.Info("attack library is loading")
		log.Infof("libContent len: %d", len(libContent))
		b.WriteString(libContent)

	} else {
		// 没有攻击库时用默认攻击路径
		b.WriteString("## Attack Priority (execute in order)\n")
		b.WriteString("## Of course, you can judge by yourself\n")
		b.WriteString("1. RECON: Find bwrap PID, locate premount dirs, check mountinfo\n")
		b.WriteString("2. DIRECT READ: Read /tmp/tc_data_premount.*/* directly\n")
		b.WriteString("3. PROC ACCESS: Read /proc/{container_pid}/root/\n")
		b.WriteString("4. ENCRYPTION CHECK: dmsetup ls, find crypto keys\n")
		b.WriteString("5. MEMORY DUMP: gdb dump process memory\n")
		b.WriteString("6. KERNEL EXPLOIT: Check uname for known CVEs\n\n")
	}

	b.WriteString("## Available Tools\n")
	b.WriteString("exec_cmd: Execute a shell command on the host\n")
	b.WriteString(`  Input: {"command": "your_shell_command"}` + "\n")
	b.WriteString("chat: Ask a question (for analysis, not commands)\n")
	b.WriteString(`  Input: {"query": "your_question"}` + "\n\n")
	if previousHistory != "" {
		b.WriteString(previousHistory)
		b.WriteString("\n")
	}
	// 已收集情报
	if len(knowledgeBase) > 0 {
		b.WriteString("## Collected Intelligence\n")
		for k, v := range knowledgeBase {
			val := v
			if len(val) > 200 {
				val = val[:200] + "..."
			}
			b.WriteString(fmt.Sprintf("- %s => %s\n", k, val))
		}
		b.WriteString("\n")
	}

	// 失败路径
	if len(failedPaths) > 0 {
		b.WriteString("## Failed Paths (DO NOT retry these)\n")
		for _, p := range failedPaths {
			b.WriteString(fmt.Sprintf("- %s\n", p))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("## Iteration %d/%d\n\n", iteration+1, maxIterations))

	// 历史步骤
	if len(steps) > 0 {
		b.WriteString("## Previous Steps\n")
		for i, s := range steps {
			b.WriteString(fmt.Sprintf("### Step %d\n", i+1))
			b.WriteString(fmt.Sprintf("Thought: %s\n", s.Thought))
			b.WriteString(fmt.Sprintf("Action: %s\n", s.Action))
			b.WriteString(fmt.Sprintf("Action Input: %v\n", s.ActionInput))
			b.WriteString(fmt.Sprintf("Observation: %s\n\n", s.Observation))
		}
	}

	b.WriteString("## Your Response\n")
	b.WriteString("Respond in EXACTLY one of these formats:\n\n")
	b.WriteString("Taking an action:\n")
	b.WriteString("Thought: <reasoning>\n")
	b.WriteString("Action: exec_cmd\n")
	b.WriteString(`Action Input: {"command": "your_shell_command"}` + "\n\n")
	b.WriteString("Final answer (only when done or all paths exhausted):\n")
	b.WriteString("Thought: <final reasoning>\n")
	b.WriteString("Final Answer: <complete attack report>\n")
	b.WriteString("preHistory:\n")
	if previousHistory != "" {
		b.WriteString(previousHistory)
		b.WriteString("\n")
	}
	return b.String()
}
