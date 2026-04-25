// cmd/demo/viewspace_demo.go

package main

import (
	"NucleusMem/pkg/viewspace"
	//"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pingcap-incubator/tinykv/log"
)

//func main() {
//	// CLI flags — defaults match your config files
//	task := flag.String("task", "", "Task description for the ViewSpaceTree")
//
//	// Manager addresses (must be running)
//	agentMgr := flag.String("agent-mgr", "127.0.0.1:7007", "AgentManager address")
//	memspaceMgr := flag.String("memspace-mgr", "127.0.0.1:9200", "MemSpaceManager address")
//
//	// Infrastructure services (must be running)
//	pdAddr := flag.String("pd", "127.0.0.1:2379", "TinyKV PD address")
//	embAddr := flag.String("embedding", "127.0.0.1:5050", "Embedding service address")
//	llmAddr := flag.String("llm", "127.0.0.1:5001", "LLM chat service address")
//
//	// Binary paths for dynamic process launch
//	agentBin := flag.String("agent-bin", "./bin/agent", "Agent binary path")
//	agentCfg := flag.String("agent-cfg", "./configs/agent.yaml", "Agent config template path")
//	msBin := flag.String("ms-bin", "./bin/memspace", "MemSpace binary path")
//	msCfg := flag.String("ms-cfg", "./configs/memspace.yaml", "MemSpace config template path")
//
//	// Port allocation
//	basePort := flag.Int("base-port", 20000, "Starting port for dynamic resource allocation")
//
//	// Tools
//	tools := flag.String("tools", "mock_lint,mock_security_scan,mock_complexity_analyzer",
//		"Comma-separated list of available tools")
//
//	flag.Parse()
//
//	if *task == "" {
//		printUsage()
//		os.Exit(1)
//	}
//
//	// Parse tools
//	availableTools := parseTools(*tools)
//
//	launchConfig := &viewspace.TreeLaunchConfig{
//		AgentBinPath:       *agentBin,
//		AgentConfigPath:    *agentCfg,
//		AgentBasePort:      *basePort,
//		MemSpaceBinPath:    *msBin,
//		MemSpaceConfigPath: *msCfg,
//		MemSpaceBasePort:   *basePort + 1000,
//		PdAddr:             *pdAddr,
//		EmbeddingAddr:      *embAddr,
//		LightModelAddr:     *llmAddr,
//	}
//
//	// Print startup info
//	printBanner(*task, *agentMgr, *memspaceMgr, *pdAddr, *embAddr, *llmAddr, *basePort, availableTools)
//
//	// ============================================================
//	// Phase 1: Build the tree (Seed + Grow)
//	// ============================================================
//	tree, err := viewspace.Run(
//		*task,
//		availableTools,
//		*agentMgr,
//		*memspaceMgr,
//		launchConfig,
//	)
//	if err != nil {
//		log.Fatalf("ViewSpaceTree construction failed: %v", err)
//	}
//
//	// ============================================================
//	// Phase 2: Execute the tree
//	// ============================================================
//	log.Infof("")
//	log.Infof("╔══════════════════════════════════════════════════════════╗")
//	log.Infof("║           ⚙️  Starting Execution                       ║")
//	log.Infof("╚══════════════════════════════════════════════════════════╝")
//
//	result, err := tree.Execute()
//	if err != nil {
//		log.Errorf("Execution failed: %v", err)
//	} else {
//		log.Infof("")
//		log.Infof("╔══════════════════════════════════════════════════════════╗")
//		log.Infof("║           📋 Final Result                              ║")
//		log.Infof("╚══════════════════════════════════════════════════════════╝")
//		log.Infof("Status: %s", result.Status)
//		log.Infof("Node: %s", result.NodeName)
//		if result.Output != nil {
//			for k, v := range result.Output {
//				log.Infof("  [%s]: %v", k, v)
//			}
//		}
//		if result.Error != "" {
//			log.Errorf("  Error: %s", result.Error)
//		}
//	}
//
//	// ============================================================
//	// Phase 3: Wait for shutdown signal
//	// ============================================================
//	sigCh := make(chan os.Signal, 1)
//	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
//
//	log.Infof("")
//	log.Infof("Tree is idle. Press Ctrl+C to shutdown and cleanup...")
//	<-sigCh
//
//	log.Infof("")
//	log.Infof("Shutting down all resources...")
//	tree.Shutdown()
//	log.Infof("Done. Goodbye.")
//}

func printUsage() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║           🌳 ViewSpaceTree Demo                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  viewspace_demo -task \"<task description>\"")
	fmt.Println()
	fmt.Println("Prerequisites (must be running):")
	fmt.Println("  1. TinyKV (PD)          default: 127.0.0.1:2379")
	fmt.Println("  2. Embedding Server     default: 127.0.0.1:5050")
	fmt.Println("  3. LLM Chat Server      default: 127.0.0.1:5001")
	fmt.Println("  4. AgentManager          default: 127.0.0.1:7007")
	fmt.Println("  5. MemSpaceManager       default: 127.0.0.1:9200")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  go run cmd/demo/viewspace_demo.go \\")
	fmt.Println("    -task \"Review the Go codebase and generate a quality report\"")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -agent-mgr      AgentManager address     (default: 127.0.0.1:7007)")
	fmt.Println("  -memspace-mgr   MemSpaceManager address  (default: 127.0.0.1:9200)")
	fmt.Println("  -pd             TinyKV PD address        (default: 127.0.0.1:2379)")
	fmt.Println("  -embedding      Embedding service        (default: 127.0.0.1:5050)")
	fmt.Println("  -llm            LLM service              (default: 127.0.0.1:5001)")
	fmt.Println("  -agent-bin      Agent binary path        (default: ./bin/agent)")
	fmt.Println("  -ms-bin         MemSpace binary path     (default: ./bin/memspace)")
	fmt.Println("  -base-port      Starting port            (default: 20000)")
	fmt.Println("  -tools          Available tools (comma-separated)")
}

//func printBanner(task, agentMgr, msMgr, pd, emb, llm string, basePort int, tools []string) {
//	log.Infof("╔══════════════════════════════════════════════════════════╗")
//	log.Infof("║           🌳 ViewSpaceTree Demo                        ║")
//	log.Infof("╠══════════════════════════════════════════════════════════╣")
//	log.Infof("║  Task:         %-40s║", truncateStr(task, 40))
//	log.Infof("║  AgentMgr:     %-40s║", agentMgr)
//	log.Infof("║  MemSpaceMgr:  %-40s║", msMgr)
//	log.Infof("║  PD:           %-40s║", pd)
//	log.Infof("║  Embedding:    %-40s║", emb)
//	log.Infof("║  LLM:          %-40s║", llm)
//	log.Infof("║  BasePort:     %-40d║", basePort)
//	log.Infof("║  Tools:        %-40s║", truncateStr(fmt.Sprintf("%v", tools), 40))
//	log.Infof("╚══════════════════════════════════════════════════════════╝")
//}

func splitAndTrim(s string, sep string) []string {
	parts := make([]string, 0)
	for _, p := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func main() {
	// ============================================================
	// 直接在这里改参数，不用命令行 flag，方便 IDE debug
	// ============================================================
	//
	//task := `You are the on-call SRE lead at a large e-commerce company.
	//At 2:00 AM, monitoring alerts fire: the checkout service is returning HTTP 503 errors
	//at a rate of 40%, causing significant revenue loss.
	//
	//The production environment runs on Kubernetes (3 clusters, ~500 pods).
	//The checkout service depends on: payment-gateway, inventory-service, user-auth-service,
	//and a PostgreSQL database cluster (primary + 2 replicas).
	//
	//Your team needs to perform a FULL incident response:
	//
	//PHASE 1 - TRIAGE (parallel, all teams start immediately):
	// - Infrastructure Team: Check K8s cluster health across all 3 clusters
	//   - Node status, resource utilization (CPU/Memory/Disk)
	//   - Pod restart counts and OOMKill events
	//   - Network policy and CNI plugin status
	//   - Ingress controller logs and connection pool stats
	//
	// - Application Team: Investigate the checkout service itself
	//   - Analyze checkout-service pod logs (error patterns, stack traces)
	//   - Check deployment rollout history (was there a recent deploy?)
	//   - Review HPA scaling events and current replica count
	//   - Examine readiness/liveness probe failure rates
	//
	// - Database Team: Investigate PostgreSQL cluster
	//   - Check replication lag between primary and replicas
	//   - Analyze slow query logs and active connections
	//   - Review PgBouncer connection pool saturation
	//   - Check disk I/O metrics and WAL accumulation
	//
	// - Dependency Team: Check all upstream/downstream services
	//   - payment-gateway health and latency percentiles (p50, p95, p99)
	//   - inventory-service response times and error rates
	//   - user-auth-service token validation latency
	//   - Redis cache hit rates and eviction counts
	//
	//PHASE 2 - CORRELATION & ROOT CAUSE ANALYSIS (after ALL triage completes):
	// - Timeline Reconstruction: Build a minute-by-minute timeline correlating all findings
	// - Dependency Graph Analysis: Map the failure propagation path
	// - Root Cause Identification: Determine the primary failure point
	// - Blast Radius Assessment: Identify all affected services and user journeys
	//
	//PHASE 3 - REMEDIATION PLANNING (after root cause is identified):
	// - Immediate Mitigation:
	//   - Draft kubectl commands for emergency scaling/rollback
	//   - Prepare circuit breaker configuration changes
	//   - Design traffic shifting rules (canary rollback if deployment-related)
	// - Communication:
	//   - Draft incident status page update for customers
	//   - Prepare internal stakeholder notification
	//   - Create post-incident review agenda
	//
	//PHASE 4 - EXECUTIVE INCIDENT REPORT (after remediation plan is ready):
	// - Generate a comprehensive incident report including:
	//   - Incident timeline with root cause
	//   - Impact assessment (revenue loss estimate, affected users)
	//   - Remediation actions taken and planned
	//   - Preventive measures and follow-up action items
	//   - Recommendations for monitoring improvements
	//
	//This is a multi-team, multi-phase incident response with strict sequential dependencies
	//between phases and parallel work within each phase.`
	//
	//availableTools := []string{
	//	"kubectl_cluster_info",
	//	"kubectl_get_nodes",
	//	"kubectl_get_pods",
	//	"kubectl_logs",
	//	"kubectl_describe",
	//	"kubectl_rollout_history",
	//	"prometheus_query",
	//	"grafana_dashboard",
	//	"pg_stat_activity",
	//	"pg_replication_status",
	//	"pgbouncer_stats",
	//	"redis_info",
	//	"jaeger_trace_search",
	//	"pagerduty_incident_create",
	//	"slack_notify",
	//	"statuspage_update",
	//	"report_generator",
	//}
	task := `Analyze a Python project's code quality.
	Do two things in parallel, then combine results:
	1. Run a linter to check code style
	2. Run a complexity analyzer to measure code complexity
	After both complete, generate a summary report combining the findings.`

	availableTools := []string{
		"mock_lint",
		"mock_complexity_analyzer",
		"mock_report_generator",
	}
	// Manager 地址（需要提前启动）
	agentMgrAddr := "127.0.0.1:7007"
	memspaceMgrAddr := "127.0.0.1:9200"

	// 基础设施地址（需要提前启动）
	pdAddr := "127.0.0.1:2379"
	embeddingAddr := "127.0.0.1:20002"
	llmAddr := "127.0.0.1:20001"

	// 二进制路径 — 改成你实际的路径
	agentBin := "./bin/agent"
	agentCfg := "" // 动态生成，不需要模板
	msBin := "./bin/memspace"
	msCfg := "" // 动态生成，不需要模板
	// 端口分配起点
	basePort := 22000
	//// 可用工具
	//availableTools := []string{
	//	"mock_lint",
	//	"mock_security_scan",
	//	"mock_complexity_analyzer",
	//}

	// ============================================================
	// 启动
	// ============================================================
	launchConfig := &viewspace.TreeLaunchConfig{
		AgentBinPath:       agentBin,
		AgentConfigPath:    agentCfg,
		AgentBasePort:      basePort,
		MemSpaceBinPath:    msBin,
		MemSpaceConfigPath: msCfg,
		MemSpaceBasePort:   basePort + 1000,
		PdAddr:             pdAddr,
		EmbeddingAddr:      embeddingAddr,
		LightModelAddr:     llmAddr,
	}

	printBanner(task, agentMgrAddr, memspaceMgrAddr, pdAddr, embeddingAddr, llmAddr, basePort, availableTools)

	// Phase 1: Build tree (Seed + Grow)
	tree, err := viewspace.Run(
		task,
		availableTools,
		agentMgrAddr,
		memspaceMgrAddr,
		launchConfig,
	)
	if err != nil {
		log.Fatalf("ViewSpaceTree construction failed: %v", err)
	}

	// Phase 2: Execute
	log.Infof("")
	log.Infof("╔══════════════════════════════════════════════════════════╗")
	log.Infof("║           ⚙️  Starting Execution                       ║")
	log.Infof("╚══════════════════════════════════════════════════════════╝")

	result, err := tree.Execute()
	if err != nil {
		log.Errorf("Execution failed: %v", err)
	} else {
		log.Infof("")
		log.Infof("╔══════════════════════════════════════════════════════════╗")
		log.Infof("║           📋 Final Result                              ║")
		log.Infof("╚══════════════════════════════════════════════════════════╝")
		log.Infof("Status: %s", result.Status)
		log.Infof("Node: %s", result.NodeName)
		if result.Output != nil {
			for k, v := range result.Output {
				log.Infof("  [%s]: %v", k, v)
			}
		}
		if result.Error != "" {
			log.Errorf("  Error: %s", result.Error)
		}
	}

	// Phase 3: Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	log.Infof("\nTree is idle. Press Ctrl+C to shutdown...")
	<-sigCh

	log.Infof("Shutting down...")
	tree.Shutdown()
	log.Infof("Done.")
}

func printBanner(task, agentMgr, msMgr, pd, emb, llm string, basePort int, tools []string) {
	log.Infof("╔══════════════════════════════════════════════════════════╗")
	log.Infof("║           🌳 ViewSpaceTree Demo                        ║")
	log.Infof("╠══════════════════════════════════════════════════════════╣")
	log.Infof("║  Task:         %-40s║", truncateStr(task, 40))
	log.Infof("║  AgentMgr:     %-40s║", agentMgr)
	log.Infof("║  MemSpaceMgr:  %-40s║", msMgr)
	log.Infof("║  PD:           %-40s║", pd)
	log.Infof("║  Embedding:    %-40s║", emb)
	log.Infof("║  LLM:          %-40s║", llm)
	log.Infof("║  BasePort:     %-40d║", basePort)
	log.Infof("║  Tools:        %-40s║", truncateStr(fmt.Sprintf("%v", tools), 40))
	log.Infof("╚══════════════════════════════════════════════════════════╝")
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func parseTools(s string) []string {
	var tools []string
	for _, t := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			tools = append(tools, trimmed)
		}
	}
	return tools
}
