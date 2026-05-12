package config

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Load reads a profile YAML file from disk and parses it.
func Load(path string) (*Config, error) {
	f, err := os.Open(path) //nolint:gosec // path is user-supplied profile file
	if err != nil {
		return nil, fmt.Errorf("config: open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	cfg, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	cfg.Path = path

	return cfg, nil
}

// Parse reads a profile YAML from r into a Config. The root must be a
// mapping whose keys are directive names. Unknown directives are rejected.
func Parse(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var root yaml.Node

	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	// root is a DocumentNode wrapping the real content.
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return &Config{}, nil
	}

	body := root.Content[0]
	if body.Kind != yaml.MappingNode {
		return nil, errAt(body, "top-level must be a mapping of directives (defaults/link/shell/clean/create)")
	}

	// yaml.v3 preserves mapping key order, so execution order is deterministic.
	cfg := &Config{Directives: make([]Directive, 0, len(body.Content)/2)}
	for i := 0; i < len(body.Content); i += 2 {
		keyNode, valNode := body.Content[i], body.Content[i+1]
		d, err := parseDirective(keyNode, valNode)
		if err != nil {
			return nil, err
		}

		cfg.Directives = append(cfg.Directives, d)
	}

	return cfg, nil
}

func parseDirective(keyNode, valNode *yaml.Node) (Directive, error) {
	key := keyNode.Value
	d := Directive{Kind: DirectiveKind(key), Line: keyNode.Line}

	switch DirectiveKind(key) {
	case KindDefaults:
		v, err := parseDefaults(valNode)
		if err != nil {
			return d, err
		}
		d.Defaults = v
	case KindLink:
		v, err := parseLink(valNode)
		if err != nil {
			return d, err
		}
		d.Link = v
	case KindShell:
		v, err := parseShell(valNode)
		if err != nil {
			return d, err
		}
		d.Shell = v
	case KindClean:
		v, err := parseClean(valNode)
		if err != nil {
			return d, err
		}
		d.Clean = v
	case KindCreate:
		v, err := parseCreate(valNode)
		if err != nil {
			return d, err
		}
		d.Create = v
	default:
		return d, errAt(keyNode, "unknown directive %q (plugins are not supported)", key)
	}
	return d, nil
}

func parseDefaults(n *yaml.Node) (*Defaults, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errAt(n, "defaults: must be a mapping")
	}

	d := &Defaults{}
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "link":
			opts, err := parseLinkOptions(v)
			if err != nil {
				return nil, err
			}
			d.Link = &opts
		case "shell":
			opts, err := parseShellOptions(v)
			if err != nil {
				return nil, err
			}
			d.Shell = &opts
		case "clean":
			opts, err := parseCleanOptions(v)
			if err != nil {
				return nil, err
			}
			d.Clean = &opts
		case "create":
			// create has only "mode" which is per-entry in practice; accept
			// but ignore for now — no tests exercise it.
		default:
			return nil, errAt(k, "defaults: unknown section %q", k.Value)
		}
	}
	return d, nil
}

func parseLink(n *yaml.Node) (*Link, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errAt(n, "link: must be a mapping of target→source")
	}

	l := &Link{Entries: make([]LinkEntry, 0, len(n.Content)/2)}

	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		e := LinkEntry{Target: k.Value}
		switch v.Kind {
		case yaml.ScalarNode:
			s := v.Value
			e.Options.Path = &s
		case yaml.MappingNode:
			opts, err := parseLinkOptions(v)
			if err != nil {
				return nil, err
			}
			e.Options = opts
		default:
			return nil, errAt(v, "link %q: value must be a string or mapping", k.Value)
		}
		l.Entries = append(l.Entries, e)
	}

	return l, nil
}

func parseLinkOptions(n *yaml.Node) (LinkOptions, error) {
	var o LinkOptions
	if n.Kind != yaml.MappingNode {
		return o, errAt(n, "link options must be a mapping")
	}

	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "path":
			s := v.Value
			o.Path = &s
		case "create":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Create = &b
		case "relink":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Relink = &b
		case "force":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Force = &b
		case "relative":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Relative = &b
		case "glob":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Glob = &b
		case "ignore-missing":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.IgnoreMissing = &b
		case "backup":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Backup = &b
		case "type":
			if v.Value != "symlink" && v.Value != "hardlink" {
				return o, errAt(v, "link type must be %q or %q, got %q", "symlink", "hardlink", v.Value)
			}
			s := v.Value
			o.Type = &s
		case "canonicalize", "canonicalize-path":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Canonicalize = &b
		case "prefix":
			s := v.Value
			o.Prefix = &s
		case "exclude":
			if v.Kind != yaml.SequenceNode {
				return o, errAt(v, "link exclude must be a sequence")
			}

			o.Exclude = make([]string, 0, len(v.Content))

			for _, item := range v.Content {
				o.Exclude = append(o.Exclude, item.Value)
			}
		case "if":
			return o, errAt(k, "link: 'if' directive is not supported; use 'when:' instead")
		default:
			return o, errAt(k, "link: unknown option %q", k.Value)
		}
	}

	return o, nil
}

func parseShell(n *yaml.Node) (*Shell, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, errAt(n, "shell: must be a sequence")
	}

	s := &Shell{Entries: make([]ShellEntry, 0, len(n.Content))}

	for _, item := range n.Content {
		e, err := parseShellEntry(item)
		if err != nil {
			return nil, err
		}

		s.Entries = append(s.Entries, e)
	}

	return s, nil
}

// parseShellEntry accepts the single canonical form: a mapping with
// `name:` + `script:` plus optional flags. `script:` is the sole source
// of the command string — single-line scalars and multiline literal
// blocks both route through it. The older `command:` key, scalar-string
// entries, and `[command, description]` lists are no longer accepted;
// one shape means one way to read (and grep) a profile.
func parseShellEntry(n *yaml.Node) (ShellEntry, error) {
	var e ShellEntry
	if n.Kind != yaml.MappingNode {
		return e, errAt(n, "shell entry must be a mapping with 'name' and 'script'")
	}

	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "name":
			e.Description = v.Value
		case "script":
			// `script` is the command text. YAML literal blocks (`|`) preserve
			// newlines so `/bin/sh -c` runs it verbatim; short commands work as
			// a plain scalar on the same line.
			e.Command = v.Value
		case "stdin":
			b, err := scalarBool(v)
			if err != nil {
				return e, err
			}
			e.Options.Stdin = &b
		case "stdout":
			b, err := scalarBool(v)
			if err != nil {
				return e, err
			}
			e.Options.Stdout = &b
		case "stderr":
			b, err := scalarBool(v)
			if err != nil {
				return e, err
			}
			e.Options.Stderr = &b
		case "quiet":
			b, err := scalarBool(v)
			if err != nil {
				return e, err
			}
			e.Options.Quiet = &b
		case "command", "description":
			return e, errAt(k, "shell: %q is no longer supported; use 'script' (or 'name' for description)", k.Value)
		default:
			return e, errAt(k, "shell: unknown key %q", k.Value)
		}
	}

	if e.Command == "" {
		return e, errAt(n, "shell: 'script' is required")
	}

	return e, nil
}

func parseShellOptions(n *yaml.Node) (ShellOptions, error) {
	var o ShellOptions
	if n.Kind != yaml.MappingNode {
		return o, errAt(n, "shell options must be a mapping")
	}

	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "stdin":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Stdin = &b
		case "stdout":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Stdout = &b
		case "stderr":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Stderr = &b
		case "quiet":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Quiet = &b
		default:
			return o, errAt(k, "shell defaults: unknown key %q", k.Value)
		}
	}

	return o, nil
}

func parseClean(n *yaml.Node) (*Clean, error) {
	c := &Clean{}
	switch n.Kind {
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, errAt(item, "clean list entries must be strings")
			}
			c.Entries = append(c.Entries, CleanEntry{Target: item.Value})
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			e := CleanEntry{Target: k.Value}
			if v.Kind == yaml.MappingNode {
				opts, err := parseCleanOptions(v)
				if err != nil {
					return nil, err
				}
				e.Options = opts
			}
			c.Entries = append(c.Entries, e)
		}
	default:
		return nil, errAt(n, "clean: must be a sequence or mapping")
	}

	return c, nil
}

func parseCleanOptions(n *yaml.Node) (CleanOptions, error) {
	var o CleanOptions
	if n.Kind != yaml.MappingNode {
		return o, errAt(n, "clean options must be a mapping")
	}

	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "force":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Force = &b
		case "recursive":
			b, err := scalarBool(v)
			if err != nil {
				return o, err
			}
			o.Recursive = &b
		default:
			return o, errAt(k, "clean: unknown option %q", k.Value)
		}
	}

	return o, nil
}

func parseCreate(n *yaml.Node) (*Create, error) {
	c := &Create{}
	switch n.Kind {
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, errAt(item, "create list entries must be strings")
			}
			c.Entries = append(c.Entries, CreateEntry{Path: item.Value})
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			e := CreateEntry{Path: k.Value}
			if v.Kind == yaml.MappingNode {
				for j := 0; j < len(v.Content); j += 2 {
					ok, ov := v.Content[j], v.Content[j+1]
					if ok.Value == "mode" {
						m, err := scalarMode(ov)
						if err != nil {
							return nil, err
						}
						e.Mode = &m
					}
				}
			}
			c.Entries = append(c.Entries, e)
		}
	default:
		return nil, errAt(n, "create: must be a sequence or mapping")
	}

	return c, nil
}

func scalarBool(n *yaml.Node) (bool, error) {
	if n.Kind != yaml.ScalarNode {
		return false, errAt(n, "expected boolean")
	}
	switch n.Value {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	}

	return false, errAt(n, "invalid boolean %q", n.Value)
}

// scalarMode parses a file-mode scalar. YAML 1.2 accepts integer modes in
// decimal; octal literals are also accepted.
func scalarMode(n *yaml.Node) (uint32, error) {
	if n.Kind != yaml.ScalarNode {
		return 0, errAt(n, "expected file mode integer")
	}

	v := n.Value
	base := 10
	if len(v) > 1 && (v[0] == '0') && (v[1] == 'o' || v[1] == 'O') {
		v = v[2:]
		base = 8
	} else if len(v) > 1 && v[0] == '0' {
		base = 8
	}

	u, err := strconv.ParseUint(v, base, 32)
	if err != nil {
		return 0, errAt(n, "invalid mode %q: %v", n.Value, err)
	}

	return uint32(u), nil
}

// ParseError records a parse failure with 1-based line/column when known.
type ParseError struct {
	Line   int
	Column int
	Msg    string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d:%d: %s", e.Line, e.Column, e.Msg)
	}
	return e.Msg
}

func errAt(n *yaml.Node, format string, args ...any) error {
	return &ParseError{Line: n.Line, Column: n.Column, Msg: fmt.Sprintf(format, args...)}
}
