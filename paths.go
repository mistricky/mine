package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveUserPath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("path is empty")
	}

	expanded := os.ExpandEnv(input)
	expanded, err := expandHomeShortcut(expanded)
	if err != nil {
		return "", err
	}

	return filepath.Abs(expanded)
}

func collapseHomePath(path string) string {
	if path == "" {
		return path
	}

	home := currentHomeDir()
	if home == "" {
		return path
	}

	cleanHome := filepath.Clean(home)
	cleanPath := filepath.Clean(path)

	if cleanPath == cleanHome {
		return "$HOME"
	}

	prefix := cleanHome + string(os.PathSeparator)
	if strings.HasPrefix(cleanPath, prefix) {
		relative := strings.TrimPrefix(cleanPath, prefix)
		if relative == "" {
			return "$HOME"
		}
		return filepath.Join("$HOME", relative)
	}

	return path
}

func expandHomeShortcut(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home := currentHomeDir()
	if home == "" {
		return "", fmt.Errorf("cannot expand ~ because HOME is not set")
	}

	if path == "~" {
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

func currentHomeDir() string {
	if value, ok := os.LookupEnv("HOME"); ok && value != "" {
		return value
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}
