package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	want := &State{
		LastApplied: []string{"base", "personal"},
		AppliedAt:   time.Now().UTC().Round(time.Second),
		Links:       []Link{{Target: "/home/u/.zshrc", Source: "/repo/zshrc"}},
	}
	if err := Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("got nil state")
		return
	}
	if len(got.LastApplied) != 2 || got.LastApplied[1] != "personal" {
		t.Errorf("last_applied: %v", got.LastApplied)
	}
	if !got.AppliedAt.Equal(want.AppliedAt) {
		t.Errorf("applied_at: got %v want %v", got.AppliedAt, want.AppliedAt)
	}
	if len(got.Links) != 1 || got.Links[0].Target != "/home/u/.zshrc" {
		t.Errorf("links: %+v", got.Links)
	}

	// verify file location honors XDG_STATE_HOME
	p, _ := Path()
	if p != filepath.Join(dir, "dfm", "state.json") {
		t.Errorf("path: %s", p)
	}
}

func TestLoad_Missing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
