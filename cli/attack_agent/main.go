// cmd/attack-agent/main.go

package main

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/agent"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	// 只有一个flag：config路径
	//configPath := flag.String("config", "", "Path to attack config YAML file (default: use built-in defaults)")
	query := flag.String("query", "", "Override attack query (optional)")
	flag.Parse()
	configPath := "./pkg/configs/file/attack.yaml"
	// 加载配置：有configPath就从文件读，没有就用默认值
	var attackCfg *configs.AttackConfig
	var llmEndpoint string
	var fullConfig *configs.AgentConfig
	if configPath != "" {
		// 从YAML文件加载完整配置
		fullConfig, _ = configs.LoadAgentConfigFromYAML(configPath)
		attackCfg = fullConfig.AttackConfig
		llmEndpoint = fullConfig.ChatServerAddr
		fmt.Printf("Config loaded from: %s\n", configPath)
	} else {
		// 用默认配置
		attackCfg = configs.DefaultAttackConfig()
		llmEndpoint = "http://localhost:20001"
		fmt.Printf("Using default config\n")
	}
	// 确保attackCfg不为空
	if attackCfg == nil {
		attackCfg = configs.DefaultAttackConfig()
	}

	// 命令行query可覆盖config里的query
	if *query != "" {
		attackCfg.Query = *query
	}

	// 构建AgentConfig
	agentConfig := &configs.AgentConfig{
		AgentId:             999,
		Role:                "attack-agent",
		ChatServerAddr:      llmEndpoint,
		HttpAddr:            "localhost:19999",
		IsJob:               true,
		PrivateMemSpaceInfo: fullConfig.PrivateMemSpaceInfo,
		AttackConfig:        attackCfg,
	}

	a, err := agent.NewAgent(agentConfig)
	if err != nil {
		fmt.Printf("Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	finalQuery := attackCfg.Query

	fmt.Printf("\n=== TrustCapsule Attack Agent ===\n")
	fmt.Printf("LLM Endpoint:       %s\n", llmEndpoint)
	fmt.Printf("Max Iterations:      %d\n", attackCfg.MaxIterations)
	fmt.Printf("Temperature:         %.2f\n", attackCfg.Temperature)
	fmt.Printf("Report Dir:          %s\n", attackCfg.ReportDir)
	fmt.Printf("Attack Library:      %s\n", attackCfg.AttackLibraryPath)
	fmt.Printf("Query:               %s\n", finalQuery)
	fmt.Printf("================================\n\n")
	taskID, err := a.SubmitTask(&agent.AgentTask{
		Type:    "attack",
		Content: finalQuery,
	})
	if err != nil {
		fmt.Printf("Failed to submit task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Attack task submitted: %s\n", taskID)
	fmt.Printf("Waiting for results (timeout: 10min)...\n\n")

	result, err := a.GetTaskResult(taskID, 10*time.Minute)
	if err != nil {
		fmt.Printf("\n=== ATTACK FAILED ===\n%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== ATTACK RESULT ===\n%s\n", result)
	fmt.Printf("\nReport saved to: %s/\n", attackCfg.ReportDir)
}
