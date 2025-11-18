# mine

`mine` is a tiny Go-powered CLI that keeps track of your shell, Python, Node.js, or any other scripts so you can invoke them with memorable aliases. It stores metadata in a portable config file, looks up the right interpreter based on file extension, and executes the script while streaming stdout/stderr just like running it yourself.

## Features
- Catalog scripts with friendly names, descriptions, and absolute paths kept in `~/.config/mine/`.
- Automatically detect which interpreter to use (sh, python, node) and allow custom executors per extension.
- List, execute, or update command definitions without editing the config file by hand.
- Inline config helper (`-config`) for printing, reading, or writing config keys.
- Colorized logging so successes, warnings, and errors stand out.

## Installation

```bash
go install github.com/mistricky/mine@latest
```

Or clone this repository and build from source:

```bash
git clone https://github.com/mistricky/mine.git
cd mine
go build .
```

## Configuration

The config file defaults to `~/.config/mine/config.toml`, and is created automatically the first time you run any command. Use `-config-file <name|path>` to override the location. When you pass a bare name (no path or extension), it is assumed to live under `~/.config/mine/<name>.toml`.

### Structure

```toml
commands_folder = "/home/mist/.config/mine/commands"

[executors]
sh = "sh {{path}}"
py = "python {{path}}"
js = "node {{path}}"

[commands.deploy]
path = "/home/mist/.config/mine/commands/deploy.sh"
description = "Builds and deploys the service"
```

- `commands_folder`: root folder where new scripts are expected to live.
- `executors`: template strings keyed by file extension. `{{path}}` is replaced with the absolute script path; configure any runtime you need (ruby, ts-node, etc.).
- `commands.<name>`: registered commands that reference a script path and display description.

You can inspect or mutate scalar values via the `-config` helper:

- `mine -config` prints the whole config.
- `mine -config commands_folder` prints the saved value.
- `mine -config commands_folder ~/scripts` sets the value and writes the file.

## Usage

```
mine [global flags] <command> [command args]
```

### Global flags
- `-v`/`-version`: print CLI version.
- `-config-file <file>`: override the config name/path.
- `-config [key] [value]`: inline config helper described above (mutually exclusive with subcommands).

### Subcommands

| Command | Description |
| --- | --- |
| `mine add <file> <alias> <description>` | Register a script. `file` may be a relative or absolute file path; `alias` is how you will reference it. The `description` is free-form text. |
| `mine ls` | List saved commands alphabetically with their descriptions. |
| `mine exec <alias>` | Execute a saved command by alias using the executor associated with its file extension. |

#### Examples

```bash
# Add a script that already lives under commands_folder
mine add deploy.sh deploy "Build and deploy the service"

# Add a script from somewhere else; mine stores the absolute path
mine add ./scripts/cleanup.rb cleanup "Remove temp resources"

# List, then execute by alias
mine ls
mine exec cleanup
```

If you see “no executor configured for extension”, add one under `[executors]` (for example `rb = "ruby {{path}}"`).

## Development

```bash
go fmt ./...
go test ./...
go run . --help
```

The test suite uses temporary directories to cover config resolution, argument parsing, executor dispatch, and error cases. Run it before submitting changes.
