package test

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/memspace_manager"
	"NucleusMem/pkg/controller/monitor/memspace_monitor"
	"NucleusMem/pkg/runtime/memspace"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestStartManager_Server(t *testing.T) {
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	manager, err := memspace_manager.NewMemSpaceManager(cfgManager)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	server := memspace_manager.NewMemSpaceManagerHTTPServer(manager)
	server.Start(cfgManager.ListenAddr)
}

func TestStartMonitor_Server(t *testing.T) {
	monitorConfigFile := "../../pkg/configs/file/memspace_monitor_1.yaml"
	cfg, err := configs.LoadMemSpaceMonitorConfigFromYAML(monitorConfigFile)
	assert.NoError(t, err)
	monitor := memspace_monitor.NewMemSpaceMonitor(cfg)
	assert.NotNil(t, monitor)
	server := memspace_monitor.NewMemSpaceMonitorHTTPServer(monitor)
	err = server.Start(cfg.MonitorUrl)
	assert.NoError(t, err)
}
func TestStartMemSpace_Server(t *testing.T) {
	memspaceConfigFile := "../../pkg/configs/file/memspace_1001.yaml"
	cfg, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	ms, err := memspace.NewMemSpace(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ms)
	server := memspace.NewMemSpaceHTTPServer(ms)
	err = server.Start()
	assert.NoError(t, err)
}

func TestManagerMonitor_Notify(t *testing.T) {
	monitorConfigFile := "../../pkg/configs/file/memspace_monitor_1.yaml"
	cfgMonitor, err := configs.LoadMemSpaceMonitorConfigFromYAML(monitorConfigFile)
	assert.NoError(t, err)
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	memspaceConfigFile := "../../pkg/configs/file/memspace_1001.yaml"
	cfgMemSpace, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	monitorClient := client.NewMemSpaceMonitorClient(cfgMonitor.MonitorUrl)
	assert.NotNil(t, monitorClient)
	managerClient := client.NewMemSpaceManagerClient(cfgManager.ListenAddr)

	// connect the memspace
	err = monitorClient.ConnectMemSpace(strconv.FormatUint(cfgMemSpace.MemSpaceID, 10), cfgMemSpace.HttpAddr)
	assert.NoError(t, err)
	spaces, err := managerClient.ListMemSpaces()
	for _, space := range spaces {
		log.Infof("memspace info: %v", space)
	}
	assert.NoError(t, err)
}

func TestManager_BindingMemSpace(t *testing.T) {
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	memspaceConfigFile := "../../pkg/configs/file/memspace_1001.yaml"
	cfgMemSpace, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	managerClient := client.NewMemSpaceManagerClient(cfgManager.ListenAddr)
	err = managerClient.BindMemSpace(101, cfgMemSpace.MemSpaceID)
	assert.NoError(t, err)
}
func TestManager_unBindingMemSpace(t *testing.T) {
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	memspaceConfigFile := "../../pkg/configs/file/memspace_1001.yaml"
	cfgMemSpace, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	managerClient := client.NewMemSpaceManagerClient(cfgManager.ListenAddr)
	err = managerClient.UnbindMemSpace(101, cfgMemSpace.MemSpaceID)
	assert.NoError(t, err)
}

func TestManager_LaunchMemSpace(t *testing.T) {
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	memspaceConfigFile := "../../pkg/configs/file/memspace_1002.yaml"
	cfgMemSpace, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	managerClient := client.NewMemSpaceManagerClient(cfgManager.ListenAddr)
	req := &api.LaunchMemSpaceRequestManager{
		MemSpaceID:     cfgMemSpace.MemSpaceID,
		BinPath:        "../../bin/memspace",
		ConfigFilePath: memspaceConfigFile,
	}
	err = managerClient.LaunchMemSpace(req)
	assert.NoError(t, err)
}
func TestManager_StopMemSpace(t *testing.T) {
	managerConfigFile := "../../pkg/configs/file/memspace_manager.yaml"
	cfgManager, err := configs.LoadMemSpaceManagerConfigFromYAML(managerConfigFile)
	assert.NoError(t, err)
	memspaceConfigFile := "../../pkg/configs/file/memspace_1002.yaml"
	cfgMemSpace, err := configs.LoadMemSpaceConfigFromYAML(memspaceConfigFile)
	assert.NoError(t, err)
	managerClient := client.NewMemSpaceManagerClient(cfgManager.ListenAddr)
	err = managerClient.ShutdownMemSpace(cfgMemSpace.MemSpaceID)
	assert.NoError(t, err)
}
