package main

import (
	"fmt"
	"io"
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

func TestParseArgs_ListCommand(t *testing.T) {
	args := []string{"ls"}

	opts, err := parseArgs(args)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.ListCmd == nil {
		t.Fatal("expected ListCmd to be populated")
	}
}

func TestParseArgs_ExecCommand(t *testing.T) {
	args := []string{"exec", "deploy"}

	opts, err := parseArgs(args)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if opts.ExecCmd == nil {
		t.Fatal("expected ExecCmd to be populated")
	}

	if opts.ExecCmd.name != "deploy" {
		t.Fatalf("ExecCmd.name = %q, want %q", opts.ExecCmd.name, "deploy")
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

func TestHandleListCommand_PrintsSortedCommands(t *testing.T) {
	cfg := &configData{
		Commands: map[string]commandDefinition{
			"deploy":  {Description: "Run deployment"},
			"cleanup": {Description: "Cleanup artifacts"},
		},
	}

	output := captureStdout(t, func() {
		handleListCommand(cfg)
	})

	expected := "cleanup  Cleanup artifacts\ndeploy  Run deployment\n"
	if output != expected {
		t.Fatalf("output = %q, want %q", output, expected)
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

func TestHandleExecCommand_RunsScript(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "hello.sh")
	outputPath := filepath.Join(dir, "exec-output.txt")
	content := fmt.Sprintf("#!/bin/sh\necho executed > %q\n", outputPath)
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cfg := &configData{
		Commands: map[string]commandDefinition{
			"hello": {
				Path:        scriptPath,
				Description: "demo",
			},
		},
		Executors: map[string]string{
			"sh": "sh {{path}}",
		},
	}

	if err := handleExecCommand(&execCommand{name: "hello"}, cfg); err != nil {
		t.Fatalf("handleExecCommand returned error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if strings.TrimSpace(string(data)) != "executed" {
		t.Fatalf("output = %q, want %q", strings.TrimSpace(string(data)), "executed")
	}
}

func TestHandleExecCommand_NoExecutorConfigured(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "task.rb")
	if err := os.WriteFile(scriptPath, []byte("puts 'hi'\n"), 0o644); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cfg := &configData{
		Commands: map[string]commandDefinition{
			"ruby-task": {Path: scriptPath},
		},
		Executors: map[string]string{},
	}

	err := handleExecCommand(&execCommand{name: "ruby-task"}, cfg)
	if err == nil {
		t.Fatal("expected error when executor is missing")
	}
	if !strings.Contains(err.Error(), "no executor configured") {
		t.Fatalf("error = %v, want no executor configured", err)
	}
}

func TestHandleExecCommand_MissingPlaceholder(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "noop.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cfg := &configData{
		Commands: map[string]commandDefinition{
			"noop": {Path: scriptPath},
		},
		Executors: map[string]string{
			"sh": "sh",
		},
	}

	err := handleExecCommand(&execCommand{name: "noop"}, cfg)
	if err == nil {
		t.Fatal("expected error when executor template is invalid")
	}
	if !strings.Contains(err.Error(), "must include {{path}}") {
		t.Fatalf("error = %v, want placeholder message", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()

	originalStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading pipe: %v", err)
	}

	return string(data)
}
