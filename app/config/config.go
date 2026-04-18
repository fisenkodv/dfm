// Package config parses profile YAML into a typed, order-preserving Config.
// Unknown directives cause a parse error.
package config

// DirectiveKind identifies which directive a Directive holds. Kept as a
// string so error messages print the original YAML key.
type DirectiveKind string

const (
	KindDefaults DirectiveKind = "defaults"
	KindLink     DirectiveKind = "link"
	KindShell    DirectiveKind = "shell"
	KindClean    DirectiveKind = "clean"
	KindCreate   DirectiveKind = "create"
)

// Directive is one entry in a profile's ordered directive list. Exactly one
// of the typed pointer fields is non-nil, matching Kind.
type Directive struct {
	Kind     DirectiveKind
	Defaults *Defaults
	Link     *Link
	Shell    *Shell
	Clean    *Clean
	Create   *Create

	// When is an optional condition expression (see app/cond). Empty means
	// "always run".
	When string

	// Line is the 1-based YAML line where this directive starts; used to
	// produce helpful error messages. Zero if unknown.
	Line int
}

// Config is the parsed form of a profile YAML file: a flat, ordered list of
// directives. Iteration order is execution order.
type Config struct {
	Path       string
	Directives []Directive
}

// Defaults holds option defaults applied to subsequent directives. A nil
// inner pointer means "no defaults set for that directive".
type Defaults struct {
	Link  *LinkOptions
	Shell *ShellOptions
	Clean *CleanOptions
}

// LinkOptions holds per-link options. All fields are pointers so we can
// distinguish "unset" from "explicitly false/zero" when merging defaults.
type LinkOptions struct {
	Path          *string
	Create        *bool
	Relink        *bool
	Force         *bool
	Relative      *bool
	Glob          *bool
	IgnoreMissing *bool
	Backup        *bool
	Type          *string // "symlink" (default) or "hardlink"
	Canonicalize  *bool
	Prefix        *string
	Exclude       []string
}

// LinkEntry is one target→source pair inside a Link directive. Options
// embedded here override any Defaults for this specific entry.
type LinkEntry struct {
	Target  string // e.g. "~/.config/nvim"
	Options LinkOptions
	// RawValue preserves the original YAML form for diagnostics: either the
	// scalar source path or the inline map.
}

// Link is the parsed "link:" directive, preserving target order.
type Link struct {
	Entries []LinkEntry
}

// ShellOptions holds per-shell-command flags.
type ShellOptions struct {
	Stdin  *bool
	Stdout *bool
	Stderr *bool
	Quiet  *bool
}

// ShellEntry is one shell command. Command is required; the other fields are
// optional overrides of Defaults.
type ShellEntry struct {
	Command     string
	Description string
	Options     ShellOptions
}

// Shell is the parsed "shell:" directive.
type Shell struct {
	Entries []ShellEntry
}

// CleanOptions holds per-target clean flags.
type CleanOptions struct {
	Force     *bool
	Recursive *bool
}

// CleanEntry is one directory to scan for dead symlinks.
type CleanEntry struct {
	Target  string
	Options CleanOptions
}

// Clean is the parsed "clean:" directive.
type Clean struct {
	Entries []CleanEntry
}

// CreateEntry is one path to mkdir.
type CreateEntry struct {
	Path string
	Mode *uint32 // nil = default (0o777)
}

// Create is the parsed "create:" directive.
type Create struct {
	Entries []CreateEntry
}
