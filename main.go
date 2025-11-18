package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mistricky/mine/logger"
)

const version = "0.1.0"

type cliOptions struct {
	ShowVersion bool
	ConfigName  string
	Silent      bool
	ConfigCmd   *configCommand
	AddCmd      *addCommand
	ListCmd     *listCommand
	ExecCmd     *execCommand
}

type configCommand struct {
	mode  configMode
	key   string
	value string
}

type addCommand struct {
	fileName    string
	commandName string
	description string
}

type listCommand struct{}

type execCommand struct {
	name string
}

type flagParseError struct {
	err error
}

func (f flagParseError) Error() string {
	return f.err.Error()
}

type configMode int

const (
	configModePrintAll configMode = iota + 1
	configModeGet
	configModeSet
)

func main() {
	opts, err := parseArgs(os.Args[1:])
	if opts.Silent {
		logger.SetSilent(true)
	}
	if err != nil {
		switch {
		case errors.Is(err, flag.ErrHelp):
			return
		default:
			var flagErr flagParseError
			if errors.As(err, &flagErr) {
				logger.Error("%s\n", flagErr.Error())
				return
			}
		}
		logger.Error("%v\n", err)
		os.Exit(2)
	}

	if opts.ShowVersion {
		logger.Default("%s\n", version)
		return
	}

	configPath, err := resolveConfigPath(opts.ConfigName)
	if err != nil {
		logger.Error("%v\n", err)
		os.Exit(1)
	}

	configValues, err := ensureConfig(configPath)
	if err != nil {
		logger.Error("%v\n", err)
		os.Exit(1)
	}

	if opts.AddCmd != nil {
		if err := handleAddCommand(opts.AddCmd, configValues, configPath); err != nil {
			logger.Error("%v\n", err)
			os.Exit(1)
		}
		return
	}

	if opts.ExecCmd != nil {
		if err := handleExecCommand(opts.ExecCmd, configValues); err != nil {
			logger.Error("%v\n", err)
			os.Exit(1)
		}
		return
	}

	if opts.ListCmd != nil {
		handleListCommand(configValues)
		return
	}

	if opts.ConfigCmd != nil {
		handleConfigCommand(opts.ConfigCmd, configPath, configValues)
		return
	}
}

func parseArgs(args []string) (cliOptions, error) {
	var opts cliOptions

	remaining, cmd, err := extractConfigCommand(args)
	if err != nil {
		return opts, err
	}
	opts.ConfigCmd = cmd

	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		printUsage(fs)
	}

	fs.BoolVar(&opts.ShowVersion, "v", false, "print version information")
	fs.BoolVar(&opts.ShowVersion, "version", false, "print version information")
	fs.StringVar(&opts.ConfigName, "config-file", "", "config file name or path")
	fs.BoolVar(&opts.Silent, "silent", false, "suppress non-default logs")

	if err := fs.Parse(remaining); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, err
		}
		return opts, flagParseError{err: err}
	}

	if fs.NArg() > 0 {
		subcommand := fs.Arg(0)
		switch subcommand {
		case "add":
			addCmd, err := parseAddCommand(fs.Args()[1:])
			if err != nil {
				return opts, err
			}
			opts.AddCmd = addCmd
		case "ls":
			listCmd, err := parseListCommand(fs.Args()[1:])
			if err != nil {
				return opts, err
			}
			opts.ListCmd = listCmd
		case "exec":
			execCmd, err := parseExecCommand(fs.Args()[1:])
			if err != nil {
				return opts, err
			}
			opts.ExecCmd = execCmd
		default:
			return opts, fmt.Errorf("unknown command: %s", subcommand)
		}
	}

	if opts.ConfigCmd != nil && (opts.AddCmd != nil || opts.ListCmd != nil || opts.ExecCmd != nil) {
		return opts, fmt.Errorf("cannot combine -config with other commands")
	}

	return opts, nil
}

func parseAddCommand(args []string) (*addCommand, error) {
	addSet := flag.NewFlagSet("add", flag.ContinueOnError)
	addSet.SetOutput(io.Discard)
	addSet.Usage = func() {
		printUsage(addSet)
	}

	if err := addSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, err
		}
		return nil, flagParseError{err: err}
	}

	if addSet.NArg() < 3 {
		return nil, fmt.Errorf("usage: %s add filename command-name description", appName)
	}

	parsed := addSet.Args()
	return &addCommand{
		fileName:    parsed[0],
		commandName: parsed[1],
		description: strings.Join(parsed[2:], " "),
	}, nil
}

func parseListCommand(args []string) (*listCommand, error) {
	lsSet := flag.NewFlagSet("ls", flag.ContinueOnError)
	lsSet.SetOutput(io.Discard)
	lsSet.Usage = func() {
		printUsage(lsSet)
	}

	if err := lsSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, err
		}
		return nil, flagParseError{err: err}
	}

	if lsSet.NArg() > 0 {
		return nil, fmt.Errorf("usage: %s ls", appName)
	}

	return &listCommand{}, nil
}

func parseExecCommand(args []string) (*execCommand, error) {
	execSet := flag.NewFlagSet("exec", flag.ContinueOnError)
	execSet.SetOutput(io.Discard)
	execSet.Usage = func() {
		printUsage(execSet)
	}

	if err := execSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, err
		}
		return nil, flagParseError{err: err}
	}

	if execSet.NArg() != 1 {
		return nil, fmt.Errorf("usage: %s exec name", appName)
	}

	return &execCommand{name: execSet.Arg(0)}, nil
}

func printUsage(fs *flag.FlagSet) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)

	logger.Default("Usage of %s:\n", fs.Name())
	logger.Default("%s", buf.String())
}

func extractConfigCommand(args []string) ([]string, *configCommand, error) {
	clean := make([]string, 0, len(args))

	for i := range args {
		arg := args[i]
		if arg != "-config" && arg != "--config" {
			clean = append(clean, arg)
			continue
		}

		remaining := args[i+1:]
		switch len(remaining) {
		case 0:
			return clean, &configCommand{mode: configModePrintAll}, nil
		case 1:
			return clean, &configCommand{mode: configModeGet, key: remaining[0]}, nil
		case 2:
			return clean, &configCommand{mode: configModeSet, key: remaining[0], value: remaining[1]}, nil
		default:
			return nil, nil, fmt.Errorf("-config takes at most two arguments")
		}
	}

	return clean, nil, nil
}

func handleConfigCommand(cmd *configCommand, configPath string, cfg *configData) {
	switch cmd.mode {
	case configModePrintAll:
		logger.Default("%s", encodeConfig(cfg))
	case configModeGet:
		value, ok := cfg.Scalars[cmd.key]
		if !ok {
			logger.Error("config item %q not found\n", cmd.key)
			os.Exit(1)
		}
		logger.Default("%s\n", value)
	case configModeSet:
		cfg.Scalars[cmd.key] = cmd.value
		if err := writeConfig(configPath, cfg); err != nil {
			logger.Error("%v\n", err)
			os.Exit(1)
		}
		logger.Success("%s updated\n", cmd.key)
	default:
		logger.Error("unknown config command\n")
		os.Exit(1)
	}
}

func handleAddCommand(cmd *addCommand, cfg *configData, configPath string) error {
	commandsDirRaw, ok := cfg.Scalars["commands_folder"]
	if !ok || commandsDirRaw == "" {
		return fmt.Errorf("commands_folder is not configured")
	}

	commandsDir, err := resolveUserPath(commandsDirRaw)
	if err != nil {
		return fmt.Errorf("unable to resolve commands_folder: %w", err)
	}

	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return fmt.Errorf("unable to prepare commands folder: %w", err)
	}

	var commandPath string
	if isSimpleCommandName(cmd.fileName) {
		commandPath = filepath.Join(commandsDir, cmd.fileName)
	} else {
		resolved, err := resolveUserPath(cmd.fileName)
		if err != nil {
			return fmt.Errorf("unable to resolve path %q: %w", cmd.fileName, err)
		}
		commandPath = resolved
	}

	info, err := os.Stat(commandPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("command file %q does not exist", commandPath)
		}
		return fmt.Errorf("unable to inspect command file %q: %w", commandPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("command path %q is a directory, expected file", commandPath)
	}

	if _, exists := cfg.Commands[cmd.commandName]; exists {
		return fmt.Errorf("command %q already exists", cmd.commandName)
	}

	cfg.Commands[cmd.commandName] = commandDefinition{
		Path:        collapseHomePath(commandPath),
		Description: cmd.description,
	}

	if err := writeConfig(configPath, cfg); err != nil {
		return fmt.Errorf("unable to update config: %w", err)
	}

	logger.Success("command %q saved\n", cmd.commandName)
	return nil
}

func handleExecCommand(cmd *execCommand, cfg *configData) error {
	entry, ok := cfg.Commands[cmd.name]
	if !ok {
		return fmt.Errorf("command %q not found", cmd.name)
	}

	if entry.Path == "" {
		return fmt.Errorf("command %q has no path configured", cmd.name)
	}

	resolvedPath, err := resolveUserPath(entry.Path)
	if err != nil {
		return fmt.Errorf("unable to resolve command path %q: %w", entry.Path, err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("command file %q does not exist", entry.Path)
		}
		return fmt.Errorf("unable to inspect command file %q: %w", entry.Path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("command path %q is a directory, expected file", entry.Path)
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(resolvedPath)), ".")
	if ext == "" {
		return fmt.Errorf("command file %q has no extension", entry.Path)
	}

	executorTemplate, ok := cfg.Executors[ext]
	if !ok {
		return fmt.Errorf("no executor configured for extension %q", ext)
	}

	commandString, err := buildExecutorCommand(executorTemplate, resolvedPath, ext)
	if err != nil {
		return err
	}

	runCmd := exec.Command("sh", "-c", commandString)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("executor command failed: %w", err)
	}

	logger.Success("Execute %s done!\n", cmd.name)
	return nil
}

func handleListCommand(cfg *configData) {
	for _, line := range formatCommandList(cfg) {
		logger.Default("%s\n", line)
	}
}

func formatCommandList(cfg *configData) []string {
	if len(cfg.Commands) == 0 {
		return nil
	}

	names := make([]string, 0, len(cfg.Commands))
	for name := range cfg.Commands {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("%s  %s", name, cfg.Commands[name].Description))
	}
	return lines
}

func buildExecutorCommand(template, scriptPath, ext string) (string, error) {
	if !strings.Contains(template, "{{path}}") {
		return "", fmt.Errorf("executor command for extension %q must include {{path}}", ext)
	}
	quoted := shellQuote(scriptPath)
	return strings.ReplaceAll(template, "{{path}}", quoted), nil
}

func shellQuote(path string) string {
	if path == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func isSimpleCommandName(value string) bool {
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) {
		return false
	}
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, "$") {
		return false
	}
	return !strings.ContainsRune(value, os.PathSeparator)
}
