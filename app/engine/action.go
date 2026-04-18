package engine

// ActionKind classifies a recorded engine action. Used by `dfm diff` to
// group and colorize output, and by tests to make assertions without
// scraping log text.
type ActionKind int

const (
	ActionLinkCreate ActionKind = iota + 1
	ActionLinkRelink
	ActionLinkExists // idempotent no-op; recorded for completeness
	ActionLinkBackup // existing non-symlink moved aside
	ActionLinkSkip   // would run but was blocked by conflict without relink/force
	ActionShellRun
	ActionCleanRemove
	ActionCreateDir
	ActionCreateExists // idempotent no-op
)

func (k ActionKind) String() string {
	switch k {
	case ActionLinkCreate:
		return "link"
	case ActionLinkRelink:
		return "relink"
	case ActionLinkExists:
		return "link-ok"
	case ActionLinkBackup:
		return "backup"
	case ActionLinkSkip:
		return "skip"
	case ActionShellRun:
		return "shell"
	case ActionCleanRemove:
		return "clean"
	case ActionCreateDir:
		return "mkdir"
	case ActionCreateExists:
		return "mkdir-ok"
	}
	return "unknown"
}

// Action is one recorded step produced by the engine. Both real and
// dry-run executions populate the same structure so diff and apply share
// their planning logic.
type Action struct {
	Kind ActionKind
	// From is the primary subject of the action: link target, directory
	// path, or command string.
	From string
	// To is the secondary target when relevant: symlink destination, backup
	// destination path, or shell command description.
	To string
	// DryRun is true when this action was recorded but not executed.
	DryRun bool
}
