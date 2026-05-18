# dfm

Standalone, single-binary dotfiles manager. Profile format is inspired by [dotbot](https://github.com/anishathalye/dotbot) but `dfm` is its own thing — **no** Python runtime, **no** git submodule.

## Install

**macOS (Homebrew)**

```sh
brew tap bitcldr/tap
brew install dfm
```

**Linux / macOS (shell script)**

```sh
curl -fsSL https://raw.githubusercontent.com/bitcldr/dfm/main/scripts/install.sh | sh
```

Drops `dfm` into `~/.local/bin` (override with `DFM_INSTALL_DIR`). Pin a version with `DFM_VERSION=vX.Y.Z`.

## Quick start

```sh
dfm apply <profile>...
```

Profiles are read from `./profiles/<name>.conf.yaml` relative to the base dir (`-C <dir>`, default cwd).

## Commands

| Command                  | Purpose                                  |
| ------------------------ | ---------------------------------------- |
| `dfm apply <profile>...` | Apply one or more profiles in order      |
| `dfm apply --dry-run`    | Report planned changes without writing   |
| `dfm diff <profile>`     | Show planned changes, no writes          |
| `dfm doctor`             | Verify installed symlinks still resolve  |
| `dfm status`             | Show last applied profiles and timestamp |
| `dfm list`               | List profiles found in `./profiles/`     |
| `dfm completion <shell>` | Output shell completion script           |

Global flags: `-C <dir>`, `-c <path>`, `--verbose` (debug logging: directive tracing, symlink resolution, shell commands), `-q`/`--quiet`.

Action lines are written to stdout (no prefix). Info and warning messages go to stderr with level prefixes (`INFO`, `WARN`). Use `--quiet` to suppress info messages or `--verbose` for debug output.

## Shell completion

```sh
# fish
dfm completion fish > ~/.config/fish/completions/dfm.fish

# bash — add to ~/.bashrc
source <(dfm completion bash)

# zsh — add to ~/.zshrc
source <(dfm completion zsh)
```

Completes subcommands, flags, and profile names (via `dfm list`). Set `$DFM_DIR` to point completions at a non-cwd base directory.

## Supported directives

Directives: `defaults`, `link`, `shell`, `clean`, `create`. Unknown directives are rejected.

- `when:` — gate any directive on `os`, `arch`, or `hostname`
- Non-symlink targets are backed up to `~/.dotfiles-backup/<timestamp>/` instead of failing
- `shell` entries use `name:` + `script:` (multiline blocks supported)

Full reference: [docs/yaml-spec.md](docs/yaml-spec.md).

## Non-goals

Templating, secret management, package installation, plugins, profile inheritance.

## License

MIT
