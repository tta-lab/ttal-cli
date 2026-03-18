package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// codexManagedMarker identifies ttal-managed entries in Codex config.toml and agent .toml files.
const codexManagedMarker = "managed_by_ttal_sync"

// GenerateCodexVariant produces a Codex agent .toml file content from a parsed canonical agent.
// The markdown body becomes developer_instructions; optional codex: frontmatter fields are merged.
func GenerateCodexVariant(agent *ParsedAgent) string {
	var sb strings.Builder
	sb.WriteString("# " + codexManagedMarker + " = true\n")

	// Collect fields for the TOML overlay
	fields := make(map[string]interface{})
	for k, v := range agent.Frontmatter.Codex {
		fields[k] = v
	}

	// Write known fields in stable order, then remaining fields sorted
	writeField(&sb, fields, "model")
	writeField(&sb, fields, "model_reasoning_effort")

	keys := sortedKeys(fields)
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s = %s\n", k, tomlValue(fields[k]))
	}

	// Write developer_instructions from body
	if body := strings.TrimSpace(agent.Body); body != "" {
		fmt.Fprintf(&sb, "developer_instructions = \"\"\"\n%s\n\"\"\"\n", body)
	}

	return sb.String()
}

// writeField writes a single field from the map and removes it.
func writeField(sb *strings.Builder, fields map[string]interface{}, key string) {
	if val, ok := fields[key]; ok {
		fmt.Fprintf(sb, "%s = %s\n", key, tomlValue(val))
		delete(fields, key)
	}
}

// DeployCodexAgents writes per-agent .toml files and merges registration entries into config.toml.
func DeployCodexAgents(agents []*ParsedAgent, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	codexDir := filepath.Join(home, ".codex")
	agentsDir := filepath.Join(codexDir, "agents")
	configPath := filepath.Join(codexDir, "config.toml")

	if !dryRun {
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			return fmt.Errorf("creating Codex agents dir: %w", err)
		}
	}

	for _, agent := range agents {
		content := GenerateCodexVariant(agent)
		dest := filepath.Join(agentsDir, agent.Frontmatter.Name+".toml")
		if !dryRun {
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing Codex agent %s: %w", dest, err)
			}
		}
	}

	if dryRun {
		return nil
	}

	return mergeCodexConfig(configPath, agents)
}

// mergeCodexConfig reads config.toml, updates ttal-managed agent entries, enables multi_agent, and writes back.
func mergeCodexConfig(configPath string, agents []*ParsedAgent) error {
	cfg, err := loadOrInitConfig(configPath)
	if err != nil {
		return err
	}

	ensureFeatureMultiAgent(cfg)

	agentsSection := getOrCreateSection(cfg, "agents")

	removeManagedAgentEntries(agentsSection, filepath.Dir(configPath))

	for _, agent := range agents {
		entry := map[string]interface{}{
			"config_file": "./agents/" + agent.Frontmatter.Name + ".toml",
		}
		if agent.Frontmatter.Description != "" {
			entry["description"] = agent.Frontmatter.Description
		}
		agentsSection[agent.Frontmatter.Name] = entry
	}

	cfg["agents"] = agentsSection
	return writeCodexConfig(configPath, cfg)
}

func loadOrInitConfig(configPath string) (map[string]interface{}, error) {
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config.toml: %w", err)
	}

	var cfg map[string]interface{}
	if len(existing) > 0 {
		if _, err := toml.Decode(string(existing), &cfg); err != nil {
			return nil, fmt.Errorf("parsing config.toml: %w", err)
		}
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	return cfg, nil
}

func ensureFeatureMultiAgent(cfg map[string]interface{}) {
	features, ok := cfg["features"].(map[string]interface{})
	if !ok {
		features = make(map[string]interface{})
	}
	features["multi_agent"] = true
	cfg["features"] = features
}

func getOrCreateSection(cfg map[string]interface{}, key string) map[string]interface{} {
	section, ok := cfg[key].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
	}
	return section
}

func removeManagedAgentEntries(agentsSection map[string]interface{}, codexDir string) {
	for name := range agentsSection {
		sub, ok := agentsSection[name].(map[string]interface{})
		if !ok {
			continue
		}
		cfgFile, ok := sub["config_file"].(string)
		if !ok {
			continue
		}
		fullPath := cfgFile
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(codexDir, cfgFile)
		}
		content, err := os.ReadFile(fullPath)
		if err == nil && strings.Contains(string(content), codexManagedMarker) {
			delete(agentsSection, name)
		}
	}
}

// writeCodexConfig writes the config map to a TOML file with sections in a readable order.
func writeCodexConfig(configPath string, cfg map[string]interface{}) error {
	var sb strings.Builder

	writeTopLevelEntries(&sb, cfg)
	writeFeaturesSection(&sb, cfg)
	writeAgentsSection(&sb, cfg)

	return os.WriteFile(configPath, []byte(sb.String()), 0o644)
}

func writeTopLevelEntries(sb *strings.Builder, cfg map[string]interface{}) {
	topLevel := make(map[string]interface{})
	for k, v := range cfg {
		if k != "features" && k != "agents" {
			topLevel[k] = v
		}
	}
	if len(topLevel) > 0 {
		_ = toml.NewEncoder(sb).Encode(topLevel)
	}
}

func writeFeaturesSection(sb *strings.Builder, cfg map[string]interface{}) {
	featMap, ok := cfg["features"].(map[string]interface{})
	if !ok {
		return
	}
	sb.WriteString("[features]\n")
	for _, k := range sortedKeys(featMap) {
		fmt.Fprintf(sb, "%s = %s\n", k, tomlValue(featMap[k]))
	}
	sb.WriteString("\n")
}

func writeAgentsSection(sb *strings.Builder, cfg map[string]interface{}) {
	agentsMap, ok := cfg["agents"].(map[string]interface{})
	if !ok {
		return
	}

	keys := sortedKeys(agentsMap)

	// Write scalar agent keys (like max_threads, max_depth)
	hasScalars := false
	for _, k := range keys {
		if _, isMap := agentsMap[k].(map[string]interface{}); !isMap {
			if !hasScalars {
				sb.WriteString("[agents]\n")
				hasScalars = true
			}
			fmt.Fprintf(sb, "%s = %s\n", k, tomlValue(agentsMap[k]))
		}
	}
	if hasScalars {
		sb.WriteString("\n")
	}

	// Write agent sub-tables
	for _, name := range keys {
		sub, ok := agentsMap[name].(map[string]interface{})
		if !ok {
			continue
		}
		fmt.Fprintf(sb, "[agents.%s]\n", name)
		for _, k := range sortedKeys(sub) {
			fmt.Fprintf(sb, "%s = %s\n", k, tomlValue(sub[k]))
		}
		sb.WriteString("\n")
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func tomlValue(v interface{}) string {
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%q", fmt.Sprint(val))
	}
}
