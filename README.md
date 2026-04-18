# dfm

Standalone, single-binary dotfiles manager. Profile format is inspired by [dotbot](https://github.com/anishathalye/dotbot) but `dfm` is its own thing — **no** Python runtime, **no** git submodule.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/fisenkodv/dfm/master/scripts/install.sh | sh
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
| `dfm diff <profile>`     | Show planned changes, no writes          |
| `dfm doctor`             | Verify installed symlinks still resolve  |
| `dfm status`             | Show last applied profiles and timestamp |
| `dfm list`               | List profiles found in `./profiles/`     |

Global flags: `-C <dir>`, `-c <path>`, `--dry-run`, `--dbg`.

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
