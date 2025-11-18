package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	appName           = "mine"
	defaultConfigName = "config.toml"
)

type commandDefinition struct {
	Path        string
	Description string
}

type configData struct {
	Scalars   map[string]string
	Commands  map[string]commandDefinition
	Executors map[string]string
}

func resolveConfigPath(name string) (string, error) {
	appConfigDir, err := userConfigDir()
	if err != nil {
		return "", err
	}

	target := name
	if target == "" {
		target = defaultConfigName
	}

	if filepath.IsAbs(target) {
		if filepath.Ext(target) == "" {
			target += ".toml"
		}
		return target, nil
	}

	if strings.ContainsAny(target, `/\`) {
		if filepath.Ext(target) == "" {
			target += ".toml"
		}
		return filepath.Join(appConfigDir, target), nil
	}

	if filepath.Ext(target) == "" {
		target += ".toml"
	}
	return filepath.Join(appConfigDir, target), nil
}

func userConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir = filepath.Join(dir, appName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func ensureConfig(path string) (*configData, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	cfg, err := loadConfig(path)
	if err == nil {
		return &cfg, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		cfg = defaultConfig(filepath.Dir(path))
		if err := writeConfig(path, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	return nil, err
}

func defaultConfig(configDir string) configData {
	return configData{
		Scalars: map[string]string{
			"commands_folder": filepath.Join(configDir, "commands"),
		},
		Commands:  make(map[string]commandDefinition),
		Executors: defaultExecutors(),
	}
}

func loadConfig(path string) (configData, error) {
	file, err := os.Open(path)
	if err != nil {
		return configData{}, err
	}
	defer file.Close()

	cfg := configData{
		Scalars:   make(map[string]string),
		Commands:  make(map[string]commandDefinition),
		Executors: make(map[string]string),
	}

	scanner := bufio.NewScanner(file)
	currentCommand := ""
	inExecutors := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			currentCommand = ""
			inExecutors = false
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			switch {
			case section == "executors":
				currentCommand = ""
				inExecutors = true
			case strings.HasPrefix(section, "commands."):
				name := strings.TrimPrefix(section, "commands.")
				if name == "" {
					return configData{}, fmt.Errorf("invalid commands section: %q", section)
				}
				currentCommand = name
				inExecutors = false
				if _, ok := cfg.Commands[currentCommand]; !ok {
					cfg.Commands[currentCommand] = commandDefinition{}
				}
			default:
				return configData{}, fmt.Errorf("unknown section: %q", section)
			}
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return configData{}, fmt.Errorf("invalid config line: %q", line)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return configData{}, fmt.Errorf("invalid config key in line: %q", line)
		}

		valueText := strings.TrimSpace(parts[1])
		value, err := parseTomlValue(valueText)
		if err != nil {
			return configData{}, fmt.Errorf("invalid value for %q: %w", key, err)
		}

		if inExecutors {
			cfg.Executors[strings.ToLower(key)] = value
			continue
		}

		if currentCommand != "" {
			entry := cfg.Commands[currentCommand]
			switch key {
			case "path":
				entry.Path = value
			case "description":
				entry.Description = value
			default:
				return configData{}, fmt.Errorf("unknown key %q in commands.%s", key, currentCommand)
			}
			cfg.Commands[currentCommand] = entry
			continue
		}

		cfg.Scalars[key] = value
	}

	if err := scanner.Err(); err != nil {
		return configData{}, err
	}

	cfg.Executors = mergeDefaultExecutors(cfg.Executors)
	return cfg, nil
}

func writeConfig(path string, cfg *configData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(encodeConfig(cfg)), 0o644)
}

func parseTomlValue(input string) (string, error) {
	if input == "" {
		return "", errors.New("empty value")
	}

	if strings.HasPrefix(input, `"`) || strings.HasPrefix(input, `'`) {
		value, err := strconv.Unquote(input)
		if err != nil {
			return "", err
		}
		return value, nil
	}

	return input, nil
}

func encodeConfig(cfg *configData) string {
	keys := make([]string, 0, len(cfg.Scalars))
	for k := range cfg.Scalars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("%s = %s\n", key, strconv.Quote(cfg.Scalars[key])))
	}

	if len(cfg.Executors) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("[executors]\n")
		executorKeys := make([]string, 0, len(cfg.Executors))
		for key := range cfg.Executors {
			executorKeys = append(executorKeys, key)
		}
		sort.Strings(executorKeys)
		for _, key := range executorKeys {
			builder.WriteString(fmt.Sprintf("%s = %s\n", key, strconv.Quote(cfg.Executors[key])))
		}
	}

	if len(cfg.Commands) == 0 {
		return builder.String()
	}

	if builder.Len() > 0 {
		builder.WriteString("\n")
	}

	var commandNames []string
	for name := range cfg.Commands {
		commandNames = append(commandNames, name)
	}
	sort.Strings(commandNames)

	for i, name := range commandNames {
		entry := cfg.Commands[name]
		builder.WriteString(fmt.Sprintf("[commands.%s]\n", name))
		builder.WriteString(fmt.Sprintf("path = %s\n", strconv.Quote(entry.Path)))
		builder.WriteString(fmt.Sprintf("description = %s\n", strconv.Quote(entry.Description)))
		if i != len(commandNames)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func mergeDefaultExecutors(existing map[string]string) map[string]string {
	base := defaultExecutors()
	if existing == nil {
		return base
	}
	for k, v := range base {
		if _, ok := existing[k]; !ok {
			existing[k] = v
		}
	}
	return existing
}

func defaultExecutors() map[string]string {
	return map[string]string{
		"js": "node {{path}}",
		"py": "python {{path}}",
		"sh": "sh {{path}}",
	}
}
