package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs_AddCommand(t *testing.T) {
	args := []string{"add", "deploy", "my-command", "Run the full deployment pipeline"}

	opts, err := parseArgs(args)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.AddCmd == nil {
		t.Fatal("expected AddCmd to be populated")
	}

	if opts.AddCmd.fileName != "deploy" {
		t.Fatalf("fileName = %q, want %q", opts.AddCmd.fileName, "deploy")
	}

	if opts.AddCmd.commandName != "my-command" {
		t.Fatalf("commandName = %q, want %q", opts.AddCmd.commandName, "my-command")
	}

	if opts.AddCmd.description != "Run the full deployment pipeline" {
		t.Fatalf("description = %q, want %q", opts.AddCmd.description, "Run the full deployment pipeline")
	}
}

func TestHandleAddCommand_SavesConfigEntry(t *testing.T) {
	dir := t.TempDir()
	cfg := &configData{
		Scalars:  map[string]string{"commands_folder": filepath.Join(dir, "commands")},
		Commands: make(map[string]commandDefinition),
	}
	configPath := filepath.Join(dir, "config.toml")
	cmd := &addCommand{
		fileName:    "deploy.sh",
		commandName: "deploy",
		description: "Run deployment",
	}

	commandsDir := cfg.Scalars["commands_folder"]
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("preparing commands dir: %v", err)
	}
	scriptPath := filepath.Join(commandsDir, cmd.fileName)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho deploy\n"), 0o755); err != nil {
		t.Fatalf("creating command file: %v", err)
	}

	if err := handleAddCommand(cmd, cfg, configPath); err != nil {
		t.Fatalf("handleAddCommand returned error: %v", err)
	}

	entry, ok := cfg.Commands["deploy"]
	if !ok {
		t.Fatal("expected deploy entry to exist")
	}

	expectedPath := filepath.Join(cfg.Scalars["commands_folder"], "deploy.sh")
	if entry.Path != expectedPath {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, expectedPath)
	}

	if entry.Description != "Run deployment" {
		t.Fatalf("entry.Description = %q, want %q", entry.Description, "Run deployment")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if !strings.Contains(string(data), "[commands.deploy]") {
		t.Fatalf("config does not contain commands section:\n%s", data)
	}

	if err := handleAddCommand(cmd, cfg, configPath); err == nil {
		t.Fatal("expected error when adding the same command name twice")
	}
}

func TestHandleAddCommand_HandlesPathInput(t *testing.T) {
	dir := t.TempDir()
	cfg := &configData{
		Scalars:  map[string]string{"commands_folder": filepath.Join(dir, "commands")},
		Commands: make(map[string]commandDefinition),
	}
	configPath := filepath.Join(dir, "config.toml")

	relativePath := filepath.Join("scripts", "cleanup.sh")
	workdir := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("creating workspace: %v", err)
	}
	target := filepath.Join(workdir, relativePath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("preparing script dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\necho cleanup\n"), 0o755); err != nil {
		t.Fatalf("creating script file: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Errorf("restoring cwd: %v", err)
		}
	})

	cmd := &addCommand{
		fileName:    relativePath,
		commandName: "cleanup",
		description: "Cleanup system",
	}

	if err := handleAddCommand(cmd, cfg, configPath); err != nil {
		t.Fatalf("handleAddCommand returned error: %v", err)
	}

	entry := cfg.Commands["cleanup"]
	if entry.Path != target {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, target)
	}
}

func TestHandleAddCommand_MissingConfig(t *testing.T) {
	cfg := &configData{
		Scalars:  map[string]string{},
		Commands: make(map[string]commandDefinition),
	}
	cmd := &addCommand{
		fileName:    "noop",
		commandName: "echo-noop",
		description: "No operation",
	}

	if err := handleAddCommand(cmd, cfg, "config.toml"); err == nil {
		t.Fatal("expected error when commands_folder is not configured")
	}
}

func TestHandleAddCommand_ErrorsWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := &configData{
		Scalars:  map[string]string{"commands_folder": filepath.Join(dir, "commands")},
		Commands: make(map[string]commandDefinition),
	}
	cmd := &addCommand{
		fileName:    "missing.sh",
		commandName: "missing",
		description: "Missing script",
	}

	if err := handleAddCommand(cmd, cfg, filepath.Join(dir, "config.toml")); err == nil {
		t.Fatal("expected error when script file does not exist")
	}
}
