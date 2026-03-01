// pkg/viewspace/config_gen.go

package viewspace

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TempConfigDir is the directory for auto-generated config files
const TempConfigDir = "/tmp/nucleusmem/configs"

func init() {
	os.MkdirAll(TempConfigDir, 0755)
}

// GenerateMemSpaceConfig creates a temp YAML config file for a new MemSpace
func GenerateMemSpaceConfig(
	memSpaceID uint64,
	name string,
	msType string,
	description string,
	httpAddr string,
	pdAddr string,
	embeddingAddr string,
	lightModelAddr string,
) (string, error) {
	cfg := map[string]interface{}{
		"memspace_id":           memSpaceID,
		"name":                  name,
		"type":                  msType,
		"owner_id":              0,
		"description":           description,
		"http_addr":             httpAddr,
		"pd_addr":               pdAddr,
		"embedding_client_addr": embeddingAddr,
		"light_model_addr":      lightModelAddr,
		"summary_cnt":           5,
		"summary_threshold":     10,
	}

	filename := fmt.Sprintf("memspace_%d.yaml", memSpaceID)
	return writeYAML(cfg, filename)
}

// GenerateAgentConfig creates a temp YAML config file for a new Agent
func GenerateAgentConfig(
	agentID uint64,
	role string,
	httpAddr string,
	chatServerAddr string,
	vectorServerAddr string,
	memSpaceManagerAddr string,
	agentManagerAddr string,
	isJob bool,
) (string, error) {
	cfg := map[string]interface{}{
		"agent_id":              agentID,
		"role":                  role,
		"http_addr":             httpAddr,
		"chat_server_addr":      chatServerAddr,
		"vector_server_addr":    vectorServerAddr,
		"memspace_manager_addr": memSpaceManagerAddr,
		"agent_manager_addr":    agentManagerAddr,
		"is_job":                isJob,
		"image":                 "nucleus-agent:v1",
	}

	filename := fmt.Sprintf("agent_%d.yaml", agentID)
	return writeYAML(cfg, filename)
}

// writeYAML marshals data to YAML and writes to TempConfigDir
func writeYAML(data map[string]interface{}, filename string) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}

	filePath := filepath.Join(TempConfigDir, filename)
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	return filePath, nil
}

// CleanupTempConfigs removes all generated config files
func CleanupTempConfigs() {
	os.RemoveAll(TempConfigDir)
}
