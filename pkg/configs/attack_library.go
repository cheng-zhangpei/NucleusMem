// pkg/configs/attack_library.go

package configs

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"sort"
	"strings"
)

// AttackLibrary 攻击库
type AttackLibrary struct {
	Name    string          `yaml:"name"`
	Version string          `yaml:"version"`
	Attacks []*AttackMethod `yaml:"attacks"`
}

// AttackMethod 单个攻击方法
type AttackMethod struct {
	ID                string   `yaml:"id"`
	Phase             string   `yaml:"phase"`
	Priority          int      `yaml:"priority"`
	Description       string   `yaml:"description"`
	Commands          []string `yaml:"commands"`
	SuccessIndicators []string `yaml:"success_indicators"`
	FailIndicators    []string `yaml:"fail_indicators"`
	NextPhase         string   `yaml:"next_phase"`
	Tags              []string `yaml:"tags"`
}

// LoadAttackLibrary 从文件加载攻击库
func LoadAttackLibrary(path string) (*AttackLibrary, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read attack library file: %w", err)
	}

	lib := &AttackLibrary{}
	if err := yaml.Unmarshal(data, lib); err != nil {
		return nil, fmt.Errorf("failed to parse attack library: %w", err)
	}

	// 按priority排序
	sort.Slice(lib.Attacks, func(i, j int) bool {
		return lib.Attacks[i].Priority < lib.Attacks[j].Priority
	})

	return lib, nil
}

// GetAttacksByPhase 按阶段获取攻击方法
func (lib *AttackLibrary) GetAttacksByPhase(phase string) []*AttackMethod {
	if lib == nil {
		return nil
	}
	var result []*AttackMethod
	for _, atk := range lib.Attacks {
		if atk.Phase == phase {
			result = append(result, atk)
		}
	}
	return result
}

// FormatForPrompt 格式化攻击库内容注入prompt
func (lib *AttackLibrary) FormatForPrompt() string {
	if lib == nil || len(lib.Attacks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Attack Library [%s v%s]\n", lib.Name, lib.Version))
	b.WriteString("Available attack methods ordered by priority. You SHOULD follow these methods but can improvise when needed.\n\n")

	currentPhase := ""
	for _, atk := range lib.Attacks {
		if atk.Phase != currentPhase {
			currentPhase = atk.Phase
			b.WriteString(fmt.Sprintf("### Phase: %s\n\n", currentPhase))
		}

		b.WriteString(fmt.Sprintf("**[%s] %s** (Priority: %d)\n", atk.ID, atk.Description, atk.Priority))
		if len(atk.Tags) > 0 {
			b.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(atk.Tags, ", ")))
		}
		b.WriteString("Commands:\n")
		for _, cmd := range atk.Commands {
			b.WriteString(fmt.Sprintf("  `%s`\n", cmd))
		}
		if len(atk.SuccessIndicators) > 0 {
			b.WriteString(fmt.Sprintf("Success: %s\n", strings.Join(atk.SuccessIndicators, ", ")))
		}
		if atk.NextPhase != "" {
			b.WriteString(fmt.Sprintf("Next: → %s\n", atk.NextPhase))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// GetDefaultAttackLibraryPath 返回默认路径
func GetDefaultAttackLibraryPath() string {
	// 优先级：环境变量 > 当前目录 > /opt/trustcapsule/attacks/
	if env := os.Getenv("ATTACK_LIBRARY_PATH"); env != "" {
		return env
	}
	if _, err := os.Stat("attacks/attack_library.yaml"); err == nil {
		return "attacks/attack_library.yaml"
	}
	if _, err := os.Stat("/opt/trustcapsule/attacks/attack_library.yaml"); err == nil {
		return "/opt/trustcapsule/attacks/attack_library.yaml"
	}
	return ""
}
