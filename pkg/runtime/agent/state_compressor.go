// pkg/runtime/agent/state_compressor.go

package agent

import (
	"NucleusMem/pkg/client"
	"fmt"
	"strings"
)

const (
	// 每隔多少步压缩一次
	CompressInterval = 5
	// 保留最近几步的完整步骤
	KeepRecentSteps = 3
)

// CompressState 压缩攻击状态
// 调用LLM生成摘要，清理旧steps，把摘要存入State.Summary
func (a *Agent) CompressState(state *ReActState) {
	if state.Iteration < CompressInterval {
		return
	}
	// 只在CompressInterval的倍数时压缩
	if state.Iteration%CompressInterval != 0 {
		return
	}

	fmt.Printf("[ATTACK] Compressing state at step %d...\n", state.Iteration)

	// 取出需要压缩的旧步骤（最近KeepRecentSteps步保留）
	compressEnd := len(state.Steps) - KeepRecentSteps
	if compressEnd <= 0 {
		return
	}
	oldSteps := state.Steps[:compressEnd]

	// 调LLM生成压缩摘要
	summary := a.callCompressLLM(state, oldSteps)

	// 把旧步骤合并进历史摘要
	if state.HistorySummary != "" {
		state.HistorySummary = state.HistorySummary + "\n\n" + summary
	} else {
		state.HistorySummary = summary
	}

	// 清理旧步骤，只保留最近几步
	state.Steps = state.Steps[compressEnd:]

	// 压缩知识图谱（去掉重复和低价值的）
	compressKnowledgeBase(state)

	fmt.Printf("[ATTACK] Compressed: %d old steps → summary. Kept %d recent steps.\n",
		compressEnd, len(state.Steps))
}

// callCompressLLM 调LLM压缩历史步骤
func (a *Agent) callCompressLLM(state *ReActState, oldSteps []ReActStep) string {
	var b strings.Builder

	b.WriteString("You are a security assessment summarizer. Compress the following attack steps into a concise summary.\n\n")
	b.WriteString(fmt.Sprintf("Target: %s\n\n", state.OriginalQuery))
	b.WriteString("Steps to compress:\n")
	for i, s := range oldSteps {
		cmd, _ := s.ActionInput["command"].(string)
		obs := s.Observation
		if len(obs) > 200 {
			obs = obs[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("Step %d: Thought=%s | Cmd=`%s` | Result=%s\n",
			i+1, truncate(s.Thought, 100), cmd, obs))
	}

	b.WriteString("\nCompressed knowledge so far:\n")
	for k, v := range state.KnowledgeBase {
		val := v
		if len(val) > 150 {
			val = val[:150] + "..."
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, val))
	}

	if len(state.FailedPaths) > 0 {
		b.WriteString("\nFailed paths:\n")
		for _, p := range state.FailedPaths {
			b.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	b.WriteString("\n\nGenerate a structured summary in this format:\n")
	b.WriteString("PHASES_COMPLETED: <list phases done>\n")
	b.WriteString("KEY_FINDINGS: <bullet list of confirmed findings>\n")
	b.WriteString("FAILED_APPROACHES: <what didn't work>\n")
	b.WriteString("OPEN_QUESTIONS: <what still needs investigation>\n")
	b.WriteString("NEXT_PRIORITY: <what to try next>\n")
	b.WriteString("Be concise. Max 300 words.\n")

	req := client.ChatCompletionRequest{
		Messages:    []client.ChatMessage{{Role: "user", Content: b.String()}},
		Temperature: 0.2,
		MaxTokens:   512,
	}
	resp, err := a.chatClient.ChatCompletion(req)
	if err != nil {
		fmt.Printf("[ATTACK] Compression LLM call failed: %v\n", err)
		return fallbackSummary(oldSteps)
	}
	if len(resp.Choices) == 0 {
		return fallbackSummary(oldSteps)
	}

	summary := resp.Choices[0].Message.Content
	fmt.Printf("[ATTACK] Compression summary:\n---\n%s\n---\n", summary)
	return summary
}

// fallbackSummary LLM失败时的降级摘要
func fallbackSummary(steps []ReActStep) string {
	var b strings.Builder
	b.WriteString("SUMMARY (auto-generated, LLM unavailable):\n")
	for i, s := range steps {
		cmd, _ := s.ActionInput["command"].(string)
		if len(cmd) > 80 {
			cmd = cmd[:80] + "..."
		}
		obs := s.Observation
		if len(obs) > 100 {
			obs = obs[:100] + "..."
		}
		b.WriteString(fmt.Sprintf("Step %d: %s → %s\n", i+1, cmd, obs))
	}
	return b.String()
}

// compressKnowledgeBase 压缩知识图谱，去除重复和低价值条目
func compressKnowledgeBase(state *ReActState) {
	// 标记哪些key保留
	keep := make(map[string]string)
	for k, v := range state.KnowledgeBase {
		// 跳过空值
		if strings.TrimSpace(v) == "" || v == "[empty]" {
			continue
		}
		// 跳过纯报错信息
		if strings.Contains(v, "Permission denied") &&
			strings.Contains(v, "No such file") {
			continue
		}
		keep[k] = v
	}
	state.KnowledgeBase = keep
}
