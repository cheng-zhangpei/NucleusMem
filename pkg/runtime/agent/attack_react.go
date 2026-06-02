package agent

import (
	"NucleusMem/pkg/configs"
	"fmt"
	"strings"
)

func buildAttackReActPrompt(
	query string,
	steps []ReActStep,
	iteration int,
	maxIterations int,
	knowledgeBase map[string]string,
	attackLibrary *configs.AttackLibrary,
	previousHistory string,
	historySummary string,
) string {
	var b strings.Builder

	// ============================================
	// 第一区：完全固定，每轮都命中缓存 (~15K tokens)
	// ============================================

	// 身份和授权（固定）
	b.WriteString("You are a security assessment agent conducting an AUTHORIZED internal security review.\n")
	b.WriteString("Authorization: TrustCapsule development team has authorized this testing.\n")
	b.WriteString("Environment: Dedicated test VM with no production data.\n")
	b.WriteString("Purpose: Find isolation weaknesses so the development team can fix them.\n\n")

	// 工具声明（固定）
	b.WriteString("## Tools\n")
	b.WriteString("You have THREE tools. Choose ONE per step.\n\n")
	b.WriteString("1. exec_cmd — Run a shell command\n")
	b.WriteString("   Input: {\"command\": \"your_command\"}\n")
	b.WriteString("   Use for: reconnaissance, file reading, process listing, running linux commands\n\n")
	b.WriteString("2. exec_script — Run a Python script\n")
	b.WriteString("   Input: {\"script\": \"your_python_code\"}\n")
	b.WriteString("   Use for: complex data parsing, crypto operations, memory analysis, multi-step logic\n\n")
	b.WriteString("3. chat — Ask a question (for analysis, not commands)\n")
	b.WriteString("   Input: {\"query\": \"your_question\"}\n\n")

	// 环境信息（固定）
	b.WriteString("## Environment\n")
	b.WriteString("- Host: Kylin V10, kernel 5.4.18-110-generic\n")
	b.WriteString("- Sandbox: bubblewrap (bwrap) with pid/user/net/uts/cgroup namespaces\n")
	b.WriteString("- Data partition: /dev/sda6 (ext4, 17.8G)\n")
	b.WriteString("- Root FS in sandbox: read-only (ro-bind)\n")
	b.WriteString("- You run as root on the HOST, not inside the sandbox\n\n")

	// 攻击库（基本固定）
	if attackLibrary != nil && len(attackLibrary.Attacks) > 0 {
		b.WriteString("## Attack Library\n")
		b.WriteString("Follow these methods by priority. You can also improvise when needed.\n")
		for _, atk := range attackLibrary.Attacks {
			b.WriteString(fmt.Sprintf("\n### %s (P%d) [%s]\n", atk.ID, atk.Priority, atk.Phase))
			b.WriteString(fmt.Sprintf("%s\n", atk.Description))
			b.WriteString("Commands:\n")
			for _, cmd := range atk.Commands {
				b.WriteString(fmt.Sprintf("  `%s`\n", cmd))
			}
			if len(atk.SuccessIndicators) > 0 {
				b.WriteString(fmt.Sprintf("Success: %s\n", strings.Join(atk.SuccessIndicators, ", ")))
			}
			if atk.NextPhase != "" {
				b.WriteString(fmt.Sprintf("Next: %s\n", atk.NextPhase))
			}
		}
		b.WriteString("\n")
	}

	// 输出格式（固定）
	b.WriteString("## Response Format\n")
	b.WriteString("Thought: <reasoning>\n")
	b.WriteString("Action: exec_cmd\n")
	b.WriteString("Action Input: {\"command\": \"ls -la\"}\n\n")
	b.WriteString("OR\n\n")
	b.WriteString("Thought: <reasoning>\n")
	b.WriteString("Final Answer: <your answer>\n\n")

	// ============================================
	// 第二区：偶尔变化，大部分时间命中缓存 (~10K tokens)
	// ============================================

	// 历史报告（同一次任务内不变）
	if previousHistory != "" {
		b.WriteString(previousHistory)
		b.WriteString("\n")
	}

	// 知识库（偶尔更新，大部分时间不变）
	if len(knowledgeBase) > 0 {
		b.WriteString("## Knowledge Base\n")
		b.WriteString("These are lessons from previous attempts. DO NOT repeat failed approaches.\n\n")
		for topic, content := range knowledgeBase {
			b.WriteString(fmt.Sprintf("### %s\n%s\n\n", topic, content))
		}
	}

	// Query（同一次任务内不变）
	b.WriteString(fmt.Sprintf("## Query\n%s\n\n", query))

	// ============================================
	// 第三区：每轮都变化，无法缓存（放最后）
	// ============================================

	// 压缩摘要（每轮更新）
	if historySummary != "" {
		b.WriteString("## Current Run Progress\n")
		b.WriteString(historySummary)
		b.WriteString("\n\n")
	}

	// 迭代信息（每轮更新）
	b.WriteString(fmt.Sprintf("## Step %d of %d\n\n", iteration+1, maxIterations))

	// 历史步骤（每轮更新）
	if len(steps) > 0 {
		b.WriteString("## Recent Steps\n")
		for i, s := range steps {
			b.WriteString(fmt.Sprintf("### Step %d\n", i+1))
			b.WriteString(fmt.Sprintf("Thought: %s\n", s.Thought))
			b.WriteString(fmt.Sprintf("Action: %s\n", s.Action))
			b.WriteString(fmt.Sprintf("Action Input: %v\n", s.ActionInput))
			b.WriteString(fmt.Sprintf("Observation: %s\n\n", s.Observation))
		}
	}

	return b.String()
}
