package system

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseXDGDocumentsDir(t *testing.T) {
	home := "/home/ana"
	cases := []struct {
		name, content, want string
	}{
		{"localised", `XDG_DESKTOP_DIR="$HOME/Escritorio"` + "\n" + `XDG_DOCUMENTS_DIR="$HOME/Documentos"`, "/home/ana/Documentos"},
		{"absolute", `XDG_DOCUMENTS_DIR="/data/docs"`, "/data/docs"},
		{"comment and spaces", "# generated\n  XDG_DOCUMENTS_DIR=\"$HOME/Docs\"  ", "/home/ana/Docs"},
		{"disabled ($HOME)", `XDG_DOCUMENTS_DIR="$HOME/"`, ""},
		{"relative is invalid", `XDG_DOCUMENTS_DIR="Documents"`, ""},
		{"missing", `XDG_MUSIC_DIR="$HOME/Music"`, ""},
	}
	for _, c := range cases {
		if got := parseXDGDocumentsDir(c.content, home); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestXDGDocumentsDirReadsConfigOrFallsBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := xdgDocumentsDir(home); got != filepath.Join(home, "Documents") {
		t.Fatalf("without user-dirs.dirs = %q", got)
	}

	cfg := filepath.Join(home, ".config")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "user-dirs.dirs"), []byte(`XDG_DOCUMENTS_DIR="$HOME/Documentos"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := xdgDocumentsDir(home); got != filepath.Join(home, "Documentos") {
		t.Fatalf("with user-dirs.dirs = %q", got)
	}
}

func TestRepositoryOptionsPerOS(t *testing.T) {
	l := locator{
		goos:       "darwin",
		userDocs:   func() (string, error) { return "/Users/ana/Documents", nil },
		sharedDocs: func() (string, string) { return "/Users/Shared", "" },
		warnings:   func(string) []string { return []string{"icloud"} },
	}
	got := l.options()
	if got.OSName != "macOS" || !got.User.Available || !got.Shared.Available {
		t.Fatalf("options = %+v", got)
	}
	if got.User.Path != "/Users/ana/Documents/My-Creative-Ink-Repository" ||
		got.Shared.Path != "/Users/Shared/My-Creative-Ink-Repository" {
		t.Fatalf("paths = %q, %q", got.User.Path, got.Shared.Path)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %v", got.Warnings)
	}

	// Linux: no shared scope, and a user-docs failure is reported, not fatal.
	l.goos = "linux"
	l.userDocs = func() (string, error) { return "", errors.New("no home") }
	l.sharedDocs = func() (string, string) { return "", "sin carpeta compartida" }
	got = l.options()
	if got.OSName != "Linux" || got.User.Available || got.Shared.Available {
		t.Fatalf("linux options = %+v", got)
	}
	if got.User.Reason == "" || got.Shared.Reason != "sin carpeta compartida" {
		t.Fatalf("reasons = %q, %q", got.User.Reason, got.Shared.Reason)
	}
	if got.Warnings == nil {
		t.Fatal("warnings must be non-nil (JS expects [])")
	}
}
