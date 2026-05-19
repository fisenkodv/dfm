package cmd

import (
	"fmt"
	"strings"
)

// CompletionCmd outputs a shell completion script for the specified shell.
type CompletionCmd struct {
	Args struct {
		Shell string `positional-arg-name:"shell"`
	} `positional-args:"yes"`
	base
}

// Execute is the go-flags entry point for `dfm completion`.
func (c *CompletionCmd) Execute(_ []string) error {
	switch strings.ToLower(c.Args.Shell) {
	case "bash":
		fmt.Fprint(c.IO().Out, bashCompletion)
	case "zsh":
		fmt.Fprint(c.IO().Out, zshCompletion)
	case "fish":
		fmt.Fprint(c.IO().Out, fishCompletion)
	default:
		return fmt.Errorf("unknown shell %q — supported: bash, zsh, fish", c.Args.Shell)
	}

	return nil
}

const bashCompletion = `# dfm bash completion
# Add to ~/.bashrc: source <(dfm completion bash)
_dfm_completion() {
    local cur prev words cword
    _init_completion || return

    local subcommands="apply diff doctor list status completion"

    # Global flag completions
    case "$prev" in
    -C|--dir)
        _filedir -d
        return
        ;;
    -c|--config)
        _filedir
        return
        ;;
    --color)
        COMPREPLY=($(compgen -W "auto always never" -- "$cur"))
        return
        ;;
    esac

    local global_flags="-C --dir -c --config --verbose -q --quiet --color"

    # No subcommand seen yet — offer subcommands and global flags.
    # Skip values of flags that consume the next word (-C, --dir, -c, --config, --color)
    # so a directory named "apply" passed to -C is not mistaken for the subcommand.
    local sub=""
    local i skip=0
    for (( i=1; i<cword; i++ )); do
        if (( skip )); then skip=0; continue; fi
        case "${words[i]}" in
        -C|--dir|-c|--config|--color) skip=1 ;;
        --verbose|-q|--quiet) ;;
        -*)  ;;
        *)
            if [[ " $subcommands " == *" ${words[i]} "* ]]; then
                sub="${words[i]}"
            fi
            break
            ;;
        esac
    done

    if [[ -z "$sub" ]]; then
        COMPREPLY=($(compgen -W "$subcommands $global_flags" -- "$cur"))
        return
    fi

    case "$sub" in
    apply|diff)
        local profiles
        profiles=$(dfm -C "${DFM_DIR:-.}" list 2>/dev/null)
        COMPREPLY=($(compgen -W "$profiles" -- "$cur"))
        ;;
    completion)
        COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
        ;;
    esac
}

complete -F _dfm_completion dfm
`

const zshCompletion = `# dfm zsh completion
# Add to ~/.zshrc: source <(dfm completion zsh)
_dfm() {
    local state

    _arguments \
        '(-C --dir)'{-C,--dir}'[base directory]:dir:_files -/' \
        '(-c --config)'{-c,--config}'[config path]:file:_files' \
        '--verbose[enable verbose (debug) logging]' \
        '(-q --quiet)'{-q,--quiet}'[suppress progress output]' \
        '--color[colorize output]:color:(auto always never)' \
        '1: :->subcommand' \
        '*: :->args'

    case $state in
    subcommand)
        local subcommands
        subcommands=(
            'apply:apply one or more profiles'
            'diff:show planned changes without writing'
            'doctor:verify installed symlinks still resolve'
            'list:list available profiles'
            'status:show last applied profiles'
            'completion:output shell completion script'
        )
        _describe 'subcommand' subcommands
        ;;
    args)
        case ${words[2]} in
        apply|diff)
            local profiles
            profiles=(${(f)"$(dfm -C ${DFM_DIR:-.} list 2>/dev/null)"})
            _describe 'profile' profiles
            ;;
        completion)
            local shells; shells=('bash' 'zsh' 'fish')
            _describe 'shell' shells
            ;;
        esac
        ;;
    esac
}

compdef _dfm dfm
`

const fishCompletion = `# dfm fish completion
# Install: dfm completion fish > ~/.config/fish/completions/dfm.fish

# Disable file completion by default
complete -c dfm -f

# Global flags
complete -c dfm -s C -l dir     -r -d 'Base directory'      -F
complete -c dfm -s c -l config  -r -d 'Config path'         -F
complete -c dfm      -l verbose     -d 'Enable verbose (debug) logging'
complete -c dfm -s q -l quiet      -d 'Suppress progress output'
complete -c dfm      -l color   -r -d 'Colorize output (auto, always, never)' -a 'auto always never'

# Subcommands
complete -c dfm -n '__fish_use_subcommand' -a apply      -d 'Apply one or more profiles'
complete -c dfm -n '__fish_use_subcommand' -a diff       -d 'Show planned changes without writing'
complete -c dfm -n '__fish_use_subcommand' -a doctor     -d 'Verify installed symlinks still resolve'
complete -c dfm -n '__fish_use_subcommand' -a list       -d 'List available profiles'
complete -c dfm -n '__fish_use_subcommand' -a status     -d 'Show last applied profiles'
complete -c dfm -n '__fish_use_subcommand' -a completion -d 'Output shell completion script'

# Profile names for apply/diff (dynamic, calls dfm list)
complete -c dfm -n '__fish_seen_subcommand_from apply diff' \
    -a '(dfm -C $DFM_DIR list 2>/dev/null)' -d 'Profile'

# Shell names for completion subcommand
complete -c dfm -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
