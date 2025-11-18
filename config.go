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

func resolveConfigPath(name string) (string, error) {
	appConfigDir, err := userConfigDir()
	if err != nil {
		return "", err
	}

	target := name
	if target == "" {
		target = defaultConfigName
	}

	// If a custom absolute path is provided, honor it directly.
	if filepath.IsAbs(target) {
		if filepath.Ext(target) == "" {
			target += ".toml"
		}
		return target, nil
	}

	// Treat values containing path separators as relative paths.
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

func ensureConfig(path string) (map[string]string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	cfg, err := loadConfig(path)
	if err == nil {
		return cfg, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		cfg = defaultConfig(filepath.Dir(path))
		if err := writeConfig(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	return nil, err
}

func defaultConfig(configDir string) map[string]string {
	return map[string]string{
		"commands_folder": filepath.Join(configDir, "commands"),
	}
}

func loadConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config line: %q", line)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid config key in line: %q", line)
		}

		valueText := strings.TrimSpace(parts[1])
		value, err := parseTomlValue(valueText)
		if err != nil {
			return nil, fmt.Errorf("invalid value for %q: %w", key, err)
		}
		cfg[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func writeConfig(path string, cfg map[string]string) error {
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

func encodeConfig(cfg map[string]string) string {
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("%s = %s\n", key, strconv.Quote(cfg[key])))
	}

	return builder.String()
}
