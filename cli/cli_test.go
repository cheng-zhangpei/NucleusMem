package cli

import (
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	defaultAgentManagerAddr = "localhost:7007"
	defaultMemManagerAddr   = "localhost:9200"
)

// ======================= Manager 生命周期 =======================
func TestLiveManagerAgentList(t *testing.T) {
	addr := getEnvOrDefault("AGENT_MANAGER_ADDR", defaultAgentManagerAddr)
	args := []string{"manager", "agent", "list", "--manager", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("list agents failed: %v", err)
	}
	t.Logf("Agent list output:\n%s", out)
}

func TestLiveManagerAgentLaunch(t *testing.T) {
	addr := getEnvOrDefault("AGENT_MANAGER_ADDR", defaultAgentManagerAddr)
	// 使用你自己的实际二进制和配置文件路径
	bin := os.Getenv("AGENT_BIN")
	cfg := os.Getenv("AGENT_CONFIG")
	if bin == "" || cfg == "" {
		bin = "bin/agent"
		cfg = "pkg/configs/file/agent_102.yaml"
		//t.Skip("Skipping test: set AGENT_BIN and AGENT_CONFIG env vars")
	}
	args := []string{
		"manager", "agent", "launch",
		"--manager", addr,
		"--bin", bin,
		"--config", cfg,
	}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("launch agent failed: %v", err)
	}

	t.Logf("Launch output:\n%s", out)
}

func TestLiveManagerAgentDestroy(t *testing.T) {
	addr := getEnvOrDefault("AGENT_MANAGER_ADDR", defaultAgentManagerAddr)
	agentID := os.Getenv("AGENT_ID_TO_DESTROY") // 注意：谨慎使用，避免误删
	if agentID == "" {
		agentID = strconv.Itoa(101)
		//t.Skip("Skipping: set AGENT_ID_TO_DESTROY to destroy an agent")
	}
	args := []string{
		"manager", "agent", "destroy",
		"--manager", addr,
		"--agent-id", agentID,
	}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("destroy agent failed: %v", err)
	}

	t.Logf("Destroy output:\n%s", out)
}

func TestLiveManagerMemSpaceList(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_MANAGER_ADDR", defaultMemManagerAddr)
	args := []string{"manager", "memspace", "list", "--manager", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("list memspaces failed: %v", err)
	}
	t.Logf("MemSpace list output:\n%s", out)
}

func TestLiveManagerMemSpaceLaunch(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_MANAGER_ADDR", defaultMemManagerAddr)
	bin := os.Getenv("MEMSPACE_BIN")
	cfg := os.Getenv("MEMSPACE_CONFIG")
	if bin == "" || cfg == "" {
		bin = "bin/memspace"
		cfg = "pkg/configs/file/memspace_1001.yaml"
	}
	args := []string{
		"manager", "memspace", "launch",
		"--manager", addr,
		"--bin", bin,
		"--config", cfg,
	}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("launch memspace failed: %v", err)
	}
	t.Logf("Launch output:\n%s", out)
}

func TestLiveManagerMemSpaceShutdown(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_MANAGER_ADDR", defaultMemManagerAddr)
	memID := os.Getenv("MEMSPACE_ID_TO_SHUTDOWN")
	if memID == "" {
		memID = strconv.Itoa(1002)
	}
	args := []string{
		"manager", "memspace", "shutdown",
		"--manager", addr,
		"--memspace-id", memID,
	}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("shutdown memspace failed: %v", err)
	}
	t.Logf("Shutdown output:\n%s", out)
}

// ======================= Agent 直接交互 =======================
func TestLiveAgentHealth(t *testing.T) {
	addr := getEnvOrDefault("AGENT_ADDR", "")
	if addr == "" {
		addr = "localhost:9002"
	}
	args := []string{"agent", "health", "--server", addr}
	_, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("agent health failed: %v", err)
	}

}

func TestLiveAgentChat(t *testing.T) {
	addr := getEnvOrDefault("AGENT_ADDR", "")
	if addr == "" {
		addr = "localhost:9002"
	}
	args := []string{"agent", "chat", "--server", addr, "-m", "what is my name?"}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	t.Logf("Chat response:\n%s", out)
}

func TestLiveAgentTempChat(t *testing.T) {
	addr := getEnvOrDefault("AGENT_ADDR", "")
	if addr == "" {
		addr = "localhost:9002"
	}
	args := []string{"agent", "tempchat", "--server", addr, "-m", "Temp hello"}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("tempchat failed: %v", err)
	}
	t.Logf("TempChat response:\n%s", out)
}

// ======================= MemSpace 直接交互 =======================
func TestLiveMemSpaceHealth(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_ADDR", "")
	if addr == "" {
		addr = "localhost:8081"
	}
	args := []string{"memspace", "health", "--server", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("memspace health failed: %v", err)
	}
	if !strings.Contains(out, "healthy") {
		t.Errorf("expected 'healthy', got: %s", out)
	}
}

func TestLiveMemSpaceToolList(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_ADDR", "")
	if addr == "" {
		addr = "localhost:8081"
	}
	args := []string{"memspace", "tool", "list", "--server", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("tool list failed: %v", err)
	}
	t.Logf("Tool list output:\n%s", out)
}

func TestLiveMemSpaceToolRegisterAndDelete(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_ADDR", "")
	if addr == "" {
		addr = "localhost:8081"
	}
	// 注册
	args := []string{
		"memspace", "tool", "register", "--server", addr,
		"--name", "cli_test_tool2",
		"--exec-type", "http",
		"--parameters", `[{"name":"msg","type":"string"}]`,
	}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("register tool failed: %v", err)
	}
	t.Logf("Register tool output:\n%s", out)

	//删除
	args = []string{
		"memspace", "tool", "delete", "--server", addr,
		"--name", "cli_test_tool",
	}
	out, err = execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("delete tool failed: %v", err)
	}
}
func TestLiveAgentBind(t *testing.T) {
	addr := getEnvOrDefault("AGENT_ADDR", "")
	memID := os.Getenv("BIND_MEMSPACE_ID")
	if addr == "" || memID == "" {
		addr = "localhost:9002"
		memID = strconv.Itoa(1001)
	}
	args := []string{"agent", "bind", "--server", addr, "--memspace-id", memID}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("agent bind failed: %v", err)
	}
	t.Logf("Bind output:\n%s", out)
}

func TestLiveAgentUnbind(t *testing.T) {
	addr := getEnvOrDefault("AGENT_ADDR", "")
	memID := os.Getenv("UNBIND_MEMSPACE_ID")
	if addr == "" || memID == "" {
		addr = "localhost:9002"
		memID = strconv.Itoa(1001)
	}
	args := []string{"agent", "unbind", "--server", addr, "--memspace-id", memID}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("agent unbind failed: %v", err)
	}
	t.Logf("Unbind output:\n%s", out)
}
func TestLiveMemSpaceMetadata(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_ADDR", "")
	if addr == "" {
		addr = "localhost:8081"
		//memID = strconv.Itoa(1001)
	}
	args := []string{"memspace", "metadata", "--server", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("metadata failed: %v", err)
	}
	t.Logf("MemSpace metadata:\n%s", out)
	if !strings.Contains(out, "success") {
		t.Errorf("expected success in output")
	}
}

func TestLiveMemSpaceMemoryDump(t *testing.T) {
	addr := getEnvOrDefault("MEMSPACE_ADDR", "")
	if addr == "" {
		addr = "localhost:8081"
	}
	args := []string{"memspace", "memory", "dump", "--server", addr}
	out, err := execute(rootCmd, args...)
	if err != nil {
		t.Fatalf("memory dump failed: %v", err)
	}
	t.Logf("Memory dump:\n%s", out)
	if !strings.Contains(out, "success") {
		t.Errorf("expected success in output")
	}
}

// ======================= 辅助函数 =======================
func execute(root *cobra.Command, args ...string) (string, error) {
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
