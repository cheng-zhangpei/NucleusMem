package configs

import (
	"os"

	"github.com/ghodss/yaml"
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

// LoadAgentConfigFromYAML 从 YAML 文件路径加载配置
//func LoadAgentConfigFromYAML(filePath string) (*AgentConfig, error) {
//	data, err := os.ReadFile(filePath)
//	if err != nil {
//		return nil, err
//	}
//	var config AgentConfig
//	err = yaml.Unmarshal(data, &config)
//	if err != nil {
//		return nil, err
//	}
//	return &config, nil
//}
