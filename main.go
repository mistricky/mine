package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mistricky/mine/logger"
)

const version = "0.1.0"

type cliOptions struct {
	ShowVersion bool
	ConfigName  string
	ConfigCmd   *configCommand
	AddCmd      *addCommand
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
		default:
			return opts, fmt.Errorf("unknown command: %s", subcommand)
		}
	}

	if opts.ConfigCmd != nil && opts.AddCmd != nil {
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
	commandsDir, ok := cfg.Scalars["commands_folder"]
	if !ok || commandsDir == "" {
		return fmt.Errorf("commands_folder is not configured")
	}

	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return fmt.Errorf("unable to prepare commands folder: %w", err)
	}

	var commandPath string
	if filepath.IsAbs(cmd.fileName) || strings.ContainsRune(cmd.fileName, os.PathSeparator) {
		abs, err := filepath.Abs(cmd.fileName)
		if err != nil {
			return fmt.Errorf("unable to resolve path %q: %w", cmd.fileName, err)
		}
		commandPath = abs
	} else {
		commandPath = filepath.Join(commandsDir, cmd.fileName)
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
		Path:        commandPath,
		Description: cmd.description,
	}

	if err := writeConfig(configPath, cfg); err != nil {
		return fmt.Errorf("unable to update config: %w", err)
	}

	logger.Success("command %q saved\n", cmd.commandName)
	return nil
}
