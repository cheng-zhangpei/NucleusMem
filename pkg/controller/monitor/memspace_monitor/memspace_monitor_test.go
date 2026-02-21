package memspace_monitor

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

const (
	testConfigPath    = "../../../configs/file/memspace_1002.yaml"
	binPath           = "../../../../bin/memspace"
	monitorConfigPath = "../../../configs/file/memspace_monitor_1.yaml"
	memspaceID        = uint64(1001)
)

func TestMemSpaceMonitor_LaunchAndStop(t *testing.T) {
	// 检查二进制是否存在
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found: %s", binPath)
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Fatalf("Test config not found: %s", testConfigPath)
	}

	// 创建 monitor
	config, err := configs.LoadMemSpaceMonitorConfigFromYAML(monitorConfigPath)
	monitor := NewMemSpaceMonitor(config)
	// Step 1: Launch
	t.Log("→ Launching MemSpace")
	req := &api.LaunchMemSpaceRequest{
		BinPath:        binPath,
		ConfigFilePath: testConfigPath,
	}
	_, err = monitor.LaunchMemSpace(req)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}
	MemSpaceConfig, err := configs.LoadMemSpaceConfigFromYAML(testConfigPath)
	assert.NoError(t, err)
	spaceClient := client.NewMemSpaceClient(MemSpaceConfig.HttpAddr)
	response, err := spaceClient.HealthCheckWithInfo()
	log.Debugf("info %s", response.Name)
	//// Step 4: Stop
	//t.Log("→ Stopping MemSpace")
	//err = monitor.StopMemSpace(memspaceID)
	//if err != nil {
	//	t.Fatalf("Stop failed: %v", err)
	//}
	//
	//// Step 5: Verify it's gone
	//time.Sleep(500 * time.Millisecond) // 等待 shutdown 完成
	//_, err = spaceClient.HealthCheckWithInfo()
	//assert.Error(t, err)
	//
	//t.Log("✅ Launch and Stop test passed!")
}
