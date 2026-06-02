package agent

import (
	"NucleusMem/pkg/configs"
	"encoding/json"
	"fmt"
	"strings"
)

func buildReActPrompt(
	query string,
	memorySummary string,
	memories []string,
	steps []ReActStep,
	iteration int,
	maxIterations int,
	attackLibrary *configs.AttackLibrary,
	previousHistory string,
	historySummary string,
	kernelCVEInfo string,
) string {
	var b strings.Builder

	// === 第一部分：身份和授权（最高优先级）===
	b.WriteString("You are a security assessment agent conducting an AUTHORIZED internal security review.\n")
	b.WriteString("Authorization: TrustCapsule development team has authorized this testing.\n")
	b.WriteString("Environment: Dedicated test VM with no production data.\n")
	b.WriteString("Purpose: Find isolation weaknesses so the development team can fix them.\n\n")

	// === 第二部分：工具声明（紧跟身份，让LLM优先记住）===
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

	// === 第四部分：环境信息 ===
	b.WriteString("## Environment\n")
	b.WriteString("- Host: Kylin V10, kernel 5.4.18-110-generic\n")
	b.WriteString("- Sandbox: bubblewrap (bwrap) with pid/user/net/uts/cgroup namespaces\n")
	b.WriteString("- Data partition: /dev/sda6 (ext4, 17.8G)\n")
	b.WriteString("- Root FS in sandbox: read-only (ro-bind)\n")
	b.WriteString("- You run as root on the HOST, not inside the sandbox\n\n")

	// === 第五部分：历史报告（MemSpace里的）===
	if previousHistory != "" {
		b.WriteString(previousHistory)
		b.WriteString("\n")
	}

	// === 第六部分：当前运行压缩摘要 ===
	if historySummary != "" {
		b.WriteString("## Current Run Progress\n")
		b.WriteString(historySummary)
		b.WriteString("\n\n")
	}

	// === 第七部分：CVE信息 ===
	if kernelCVEInfo != "" {
		b.WriteString(kernelCVEInfo)
		b.WriteString("\n")
	}

	// === 第八部分：记忆 ===
	if memorySummary != "" {
		b.WriteString("## Memory Context\n")
		b.WriteString(memorySummary)
		b.WriteString("\n\n")
	}
	if len(memories) > 0 {
		b.WriteString("## Relevant Memories\n")
		for i, m := range memories {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, m))
		}
		b.WriteString("\n")
	}

	// === 第九部分：攻击库 ===
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

	// === 第十部分：失败路径 ===
	// 这个在handleAttackReActTask里已经通过knowledgeBase注入了，这里不需要

	// === 第十一部分：当前Query ===
	b.WriteString(fmt.Sprintf("## Query\n%s\n\n", query))

	// === 第十二部分：迭代信息 ===
	b.WriteString(fmt.Sprintf("## Step %d of %d\n\n", iteration+1, maxIterations))

	// === 第十三部分：历史步骤 ===
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

	// === 第十四部分：输出格式（简洁明确）===
	b.WriteString("## Response Format\n")
	b.WriteString("Thought: <reasoning>\n")
	b.WriteString("Action: exec_cmd\n")
	b.WriteString("Action Input: {\"command\": \"ls -la\"}\n\n")
	b.WriteString("OR\n\n")
	b.WriteString("Thought: <reasoning>\n")
	b.WriteString("Final Answer: <your answer>\n")

	return b.String()
}

type ReActParsed struct {
	IsFinalAnswer bool
	Thought       string
	FinalAnswer   string
	Action        string
	ActionInput   map[string]interface{}
}

// 从JSON字符串中提取指定字段的值
func extractJSONField(jsonStr string, field string) string {
	// 简单正则提取 "field": "value"
	search := fmt.Sprintf(`"%s":`, field)
	idx := strings.Index(jsonStr, search)
	if idx < 0 {
		return ""
	}
	rest := jsonStr[idx+len(search):]
	rest = strings.TrimSpace(rest)

	// 跳过冒号和空格，找到值的开始
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:] // 跳过开头的引号

	// 找到结束引号（不处理转义，够用了）
	endIdx := strings.Index(rest, `"`)
	if endIdx < 0 {
		return ""
	}
	return rest[:endIdx]
}
func parseAttackReActResponse(response string) ReActParsed {
	result := ReActParsed{}

	// 提取 Thought
	if idx := strings.Index(response, "Thought:"); idx >= 0 {
		rest := response[idx+len("Thought:"):]
		endIdx := len(rest)
		for _, marker := range []string{"Action:", "Final Answer:"} {
			if i := strings.Index(rest, marker); i >= 0 && i < endIdx {
				endIdx = i
			}
		}
		result.Thought = strings.TrimSpace(rest[:endIdx])
	}

	// 判断 Final Answer
	if idx := strings.Index(response, "Final Answer:"); idx >= 0 {
		result.IsFinalAnswer = true
		result.FinalAnswer = strings.TrimSpace(response[idx+len("Final Answer:"):])
		return result
	}

	// 提取 Action
	if idx := strings.Index(response, "Action:"); idx >= 0 {
		rest := response[idx+len("Action:"):]
		if endIdx := strings.Index(rest, "Action Input:"); endIdx >= 0 {
			result.Action = strings.TrimSpace(rest[:endIdx])
		} else {
			result.Action = strings.TrimSpace(rest)
		}
	}

	// 提取 Action Input
	if idx := strings.Index(response, "Action Input:"); idx >= 0 {
		inputStr := strings.TrimSpace(response[idx+len("Action Input:"):])

		// 去掉markdown标记
		inputStr = strings.TrimSuffix(inputStr, "```")
		inputStr = strings.TrimSuffix(inputStr, "```json")
		inputStr = strings.TrimSpace(inputStr)

		var input map[string]interface{}
		parsed := false

		// 方式1：直接JSON解析
		if err := json.Unmarshal([]byte(inputStr), &input); err == nil {
			parsed = true
		}

		// 方式2：单引号替换成双引号
		if !parsed {
			fixed := strings.ReplaceAll(inputStr, "'", "\"")
			if err := json.Unmarshal([]byte(fixed), &input); err == nil {
				parsed = true
			}
		}

		// 方式3：手动提取command值（兜住所有格式）
		if !parsed {
			cmd := extractCommandValue(inputStr)
			if cmd != "" {
				input = map[string]interface{}{"command": cmd}
				parsed = true
			}
		}

		if parsed {
			result.ActionInput = input
		} else {
			// 最终fallback：整个字符串当command
			result.ActionInput = map[string]interface{}{
				"command": strings.Trim(inputStr, "`\"' \n"),
			}
		}
	}

	return result
}

// 从各种格式中提取command的值
func extractCommandValue(s string) string {
	// 尝试匹配 'command': 'value' 或 "command": "value"
	patterns := []struct {
		prefix string
		quote  byte
	}{
		{`'command': '`, '\''},
		{`"command": "`, '"'},
		{`'command': "`, '"'},
		{`"command": '`, '\''},
	}

	for _, p := range patterns {
		idx := strings.Index(s, p.prefix)
		if idx < 0 {
			continue
		}
		rest := s[idx+len(p.prefix):]
		// 找到匹配的结束引号
		endIdx := strings.IndexByte(rest, p.quote)
		if endIdx < 0 {
			continue
		}
		return rest[:endIdx]
	}

	// 最后尝试：如果整个字符串看起来就是一条命令（没有key-value结构）
	trimmed := strings.TrimSpace(s)
	if !strings.Contains(trimmed, "command") &&
		!strings.Contains(trimmed, "{") &&
		!strings.Contains(trimmed, "}") {
		return trimmed
	}

	return ""
}
