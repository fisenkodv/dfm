package config

import (
	"strings"
	"testing"
)

func TestParse_BaseProfile(t *testing.T) {
	src := `
defaults:
  link:
    relink: true
    force: false
  shell:
    stdout: true
    stderr: true
    quiet: true

clean:
  - "~"

shell:
  - name: installing submodules
    script: git submodule update --init

link:
  ~/.config/nvim: config/nvim
  ~/.zshrc: config/zsh/zshrc.zsh
`
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(cfg.Directives), 4; got != want {
		t.Fatalf("directives: got %d want %d", got, want)
	}

	// 1. defaults
	d := cfg.Directives[0]
	if d.Kind != KindDefaults {
		t.Errorf("directive[0].Kind = %q, want defaults", d.Kind)
	}
	if d.Defaults == nil || d.Defaults.Link == nil || d.Defaults.Shell == nil {
		t.Fatalf("defaults not populated: %+v", d.Defaults)
	}
	if d.Defaults.Link.Relink == nil || !*d.Defaults.Link.Relink {
		t.Errorf("defaults.link.relink: want true, got %v", d.Defaults.Link.Relink)
	}
	if d.Defaults.Shell.Quiet == nil || !*d.Defaults.Shell.Quiet {
		t.Errorf("defaults.shell.quiet: want true")
	}

	// 2. clean
	if cfg.Directives[1].Kind != KindClean {
		t.Errorf("directive[1].Kind = %q, want clean", cfg.Directives[1].Kind)
	}
	if got := cfg.Directives[1].Clean.Entries; len(got) != 1 || got[0].Target != "~" {
		t.Errorf("clean entries = %+v", got)
	}

	// 3. shell — order preserved (shell before link in this fixture)
	sh := cfg.Directives[2].Shell
	if sh == nil || len(sh.Entries) != 1 {
		t.Fatalf("shell not populated: %+v", sh)
	}
	if !strings.HasPrefix(sh.Entries[0].Command, "git submodule") {
		t.Errorf("shell command: %q", sh.Entries[0].Command)
	}
	if sh.Entries[0].Description != "installing submodules" {
		t.Errorf("shell desc: %q", sh.Entries[0].Description)
	}

	// 4. link
	lk := cfg.Directives[3].Link
	if lk == nil || len(lk.Entries) != 2 {
		t.Fatalf("link not populated: %+v", lk)
	}
	if lk.Entries[0].Target != "~/.config/nvim" {
		t.Errorf("link[0].Target = %q", lk.Entries[0].Target)
	}
	if lk.Entries[0].Options.Path == nil || *lk.Entries[0].Options.Path != "config/nvim" {
		t.Errorf("link[0].Path = %v", lk.Entries[0].Options.Path)
	}
}

func TestParse_LinkExtendedForm(t *testing.T) {
	src := `
link:
  ~/.vim:
    path: config/vim
    create: true
    relink: true
    glob: false
    type: symlink
    exclude: ["*.log"]
`
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	e := cfg.Directives[0].Link.Entries[0]
	if e.Target != "~/.vim" {
		t.Errorf("target: %q", e.Target)
	}
	if e.Options.Path == nil || *e.Options.Path != "config/vim" {
		t.Errorf("path: %v", e.Options.Path)
	}
	if e.Options.Create == nil || !*e.Options.Create {
		t.Errorf("create: %v", e.Options.Create)
	}
	if len(e.Options.Exclude) != 1 || e.Options.Exclude[0] != "*.log" {
		t.Errorf("exclude: %+v", e.Options.Exclude)
	}
}

func TestParse_ShellCanonicalForm(t *testing.T) {
	src := `
shell:
  - name: farewell
    script: echo bye
    quiet: true
`
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	entries := cfg.Directives[0].Shell.Entries
	if len(entries) != 1 {
		t.Fatalf("entries: %+v", entries)
	}
	if entries[0].Command != "echo bye" || entries[0].Description != "farewell" {
		t.Errorf("entry: %+v", entries[0])
	}
	if entries[0].Options.Quiet == nil || !*entries[0].Options.Quiet {
		t.Errorf("quiet: %v", entries[0].Options.Quiet)
	}
}

func TestParse_ShellRejectsLegacyForms(t *testing.T) {
	for _, src := range []string{
		"shell:\n  - echo hello\n",
		"shell:\n  - [\"echo world\", \"say world\"]\n",
		"shell:\n  - command: echo bye\n    description: farewell\n",
	} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("expected error for legacy form: %q", src)
		}
	}
}

func TestParse_ShellMultilineScript(t *testing.T) {
	src := "shell:\n" +
		"  - name: some command\n" +
		"    script: |\n" +
		"      ls -laR /tmp\n" +
		"      du -hcs /srv\n" +
		"      echo ok\n"
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	e := cfg.Directives[0].Shell.Entries[0]
	if e.Description != "some command" {
		t.Errorf("name: %q", e.Description)
	}
	for _, line := range []string{"ls -laR /tmp", "du -hcs /srv", "echo ok"} {
		if !strings.Contains(e.Command, line) {
			t.Errorf("script missing %q; got:\n%s", line, e.Command)
		}
	}
	if !strings.Contains(e.Command, "\n") {
		t.Error("script should preserve newlines")
	}
}

func TestParse_CreateWithMode(t *testing.T) {
	src := `
create:
  ~/tmp:
    mode: 0700
  ~/logs: {}
`
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	entries := cfg.Directives[0].Create.Entries
	if len(entries) != 2 {
		t.Fatalf("entries: %+v", entries)
	}
	if entries[0].Mode == nil || *entries[0].Mode != 0o700 {
		t.Errorf("mode = %v want 0700", entries[0].Mode)
	}
	if entries[1].Mode != nil {
		t.Errorf("entry[1].Mode should be nil, got %v", entries[1].Mode)
	}
}

func TestParse_UnknownDirectiveRejected(t *testing.T) {
	src := `
teleport:
  src: dst
`
	_, err := Parse(strings.NewReader(src))
	if err == nil {
		t.Fatalf("want error for unknown directive, got nil")
	}
	if !strings.Contains(err.Error(), "unknown directive") {
		t.Errorf("error = %q, want mention of unknown directive", err)
	}
}

func TestParse_RejectsIfPredicate(t *testing.T) {
	src := `
link:
  ~/.foo:
    path: foo
    if: '[ -f /etc/os-release ]'
`
	_, err := Parse(strings.NewReader(src))
	if err == nil {
		t.Fatal("want error for 'if:' directive")
	}
	if !strings.Contains(err.Error(), "'if'") {
		t.Errorf("error = %q, want mention of 'if'", err)
	}
}

func TestParse_OrderPreserved(t *testing.T) {
	src := `
link:
  a: 1
  b: 2
  c: 3
  d: 4
`
	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := make([]string, 0, 4)
	for _, e := range cfg.Directives[0].Link.Entries {
		got = append(got, e.Target)
	}
	want := []string{"a", "b", "c", "d"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
