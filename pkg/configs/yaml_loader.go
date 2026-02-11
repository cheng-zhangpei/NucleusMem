package configs

import (
	"bufio"
	"fmt"
	"github.com/ghodss/yaml"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// LoadAgentConfigFromYAML load the config from yaml file
func LoadAgentConfigFromYAML(filePath string) (*AgentConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var config AgentConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadMonitorConfigFromYAML load the MonitorConfig from yaml file
func LoadMonitorConfigFromYAML(filePath string) (*MonitorConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config MonitorConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type agentManagerRawConfig struct {
	MonitorURLs map[string]string `yaml:"monitor_urls"`
}

func LoadAgentManagerConfigFromYAML(filePath string) (*AgentManagerConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	monitorURLs := make(map[uint64]string)
	inMonitorSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "monitor_urls:" {
			inMonitorSection = true
			continue
		}

		if inMonitorSection {
			// 匹配: "1": "localhost:8081"
			// 或:   1: localhost:8081 （无引号）
			re := regexp.MustCompile(`^["']?(\d+)["']?\s*:\s*["']?([a-zA-Z0-9.:\-_]+)["']?$`)
			matches := re.FindStringSubmatch(line)
			if matches != nil {
				id, err := strconv.ParseUint(matches[1], 10, 64)
				if err != nil {
					continue // 跳过无效 ID
				}
				hostPort := matches[2]
				monitorURLs[id] = hostPort
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(monitorURLs) == 0 {
		return nil, fmt.Errorf("no monitor URLs found in %s", filePath)
	}

	return &AgentManagerConfig{MonitorURLs: monitorURLs}, nil
}
