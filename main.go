package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mistricky/mine/logger"
)

const version = "0.1.0"

type cliOptions struct {
	ShowVersion bool
	ConfigName  string
	ConfigCmd   *configCommand
}

type configCommand struct {
	mode  configMode
	key   string
	value string
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

	fs.BoolVar(&opts.ShowVersion, "v", false, "print version information")
	fs.BoolVar(&opts.ShowVersion, "version", false, "print version information")
	fs.StringVar(&opts.ConfigName, "config-file", "", "config file name or path")

	if err := fs.Parse(remaining); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(fs)
			return opts, err
		}
		return opts, flagParseError{err: err}
	}

	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	return opts, nil
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

func handleConfigCommand(cmd *configCommand, configPath string, values map[string]string) {
	switch cmd.mode {
	case configModePrintAll:
		logger.Default("%s", encodeConfig(values))
	case configModeGet:
		value, ok := values[cmd.key]
		if !ok {
			logger.Error("config item %q not found\n", cmd.key)
			os.Exit(1)
		}
		logger.Default("%s\n", value)
	case configModeSet:
		values[cmd.key] = cmd.value
		if err := writeConfig(configPath, values); err != nil {
			logger.Error("%v\n", err)
			os.Exit(1)
		}
		logger.Success("%s updated\n", cmd.key)
	default:
		logger.Error("unknown config command\n")
		os.Exit(1)
	}
}
