package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEnsureRepositoryLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Documents", "My-Creative-Ink-Repository")
	if err := EnsureRepositoryLayout(root, false); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".creative-ink/templates", ".creative-ink/guide", ".creative-ink/plugins", "tmp-Workspaces"} {
		if info, err := os.Stat(filepath.Join(root, rel)); err != nil || !info.IsDir() {
			t.Errorf("%s missing: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".creative-ink", "config.json")); err != nil {
		t.Errorf("config.json missing: %v", err)
	}
	// The root is a launcher: no document is written there.
	if entries, _ := os.ReadDir(root); len(entries) != 2 {
		t.Errorf("root entries = %d, want only .creative-ink and tmp-Workspaces", len(entries))
	}
	// Idempotent and never overwrites config.json.
	cfg := filepath.Join(root, ".creative-ink", "config.json")
	if err := os.WriteFile(cfg, []byte(`{"version":1,"x":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRepositoryLayout(root, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), `"x":true`) {
		t.Fatal("an existing config.json must be kept")
	}
}

func TestEnsureRepositoryLayoutShared(t *testing.T) {
	root := filepath.Join(t.TempDir(), "My-Creative-Ink-Repository")
	if err := EnsureRepositoryLayout(root, true); err != nil {
		t.Fatal(err)
	}
	// Plugins are per-user: never inside a repository every account can write.
	if _, err := os.Stat(filepath.Join(root, ".creative-ink", "plugins")); !os.IsNotExist(err) {
		t.Fatal("a shared repository must not have a plugins folder")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(root, "tmp-Workspaces"))
		if info.Mode().Perm() != 0o777 || info.Mode()&os.ModeSticky == 0 {
			t.Fatalf("tmp-Workspaces mode = %v, want drwxrwxrwt", info.Mode())
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Tesis de Maestría":            "Tesis-de-Maestria",
		"  Análisis   Socio-Espacial ": "Analisis-Socio-Espacial",
		"Proyecto/raro\\nombre?":       "Proyecto-raro-nombre",
		"ya-normalizado_v2.1":          "ya-normalizado_v2.1",
		"---":                          "",
		"Ñandú Ç":                      "Nandu-C",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Slugify(strings.Repeat("a", 200)); len(got) > maxSlugLength {
		t.Errorf("slug length = %d, want <= %d", len(got), maxSlugLength)
	}
}

func TestCreateProject(t *testing.T) {
	root := t.TempDir()
	dir, mainTex, err := CreateProject(root, "Tesis de Maestría & Co")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(root, "Tesis-de-Maestria-Co") || mainTex != filepath.Join(dir, "main.tex") {
		t.Fatalf("dir=%q main=%q", dir, mainTex)
	}
	for _, rel := range []string{"main.tex", "capitulos/introduccion.tex", "referencias.bib", "figuras"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("%s missing: %v", rel, err)
		}
	}
	data, _ := os.ReadFile(mainTex)
	if !strings.Contains(string(data), `\title{Tesis de Maestría \& Co}`) {
		t.Errorf("the title must keep the original name, escaped:\n%s", data)
	}
	if meta, _ := LoadProjectMeta(dir); meta.MainFilePath(dir) != mainTex {
		t.Errorf("main.tex must be the main file, meta = %+v", meta)
	}

	if _, _, err := CreateProject(root, "Tesis de Maestria & Co"); !errors.Is(err, ErrProjectExists) {
		t.Errorf("same slug: err = %v, want ErrProjectExists", err)
	}
	for _, bad := range []string{"", "  ", "tmp-Workspaces", "---"} {
		if _, _, err := CreateProject(root, bad); err == nil {
			t.Errorf("CreateProject(%q) must fail", bad)
		}
	}
	// A leading dot is trimmed: a project can never be a hidden folder.
	if d, _, err := CreateProject(root, ".oculto"); err != nil || filepath.Base(d) != "oculto" {
		t.Errorf(".oculto: dir=%q err=%v, want a visible \"oculto\" folder", d, err)
	}
}

func TestQuickDocsUseNumberedSubfolders(t *testing.T) {
	root := t.TempDir()
	d1, tex1, err := CreateQuickDoc(root, "starter")
	if err != nil {
		t.Fatal(err)
	}
	d2, _, _ := CreateQuickDoc(root, "starter")
	if filepath.Base(d1) != "unnamed-tmp1" || filepath.Base(d2) != "unnamed-tmp2" {
		t.Fatalf("dirs = %q, %q", d1, d2)
	}
	if data, _ := os.ReadFile(tex1); string(data) != "starter" {
		t.Fatalf("starter = %q", data)
	}
	// Gaps are not refilled: numbers keep growing after the highest one.
	if err := os.RemoveAll(d1); err != nil {
		t.Fatal(err)
	}
	if d3, _, _ := CreateQuickDoc(root, "x"); filepath.Base(d3) != "unnamed-tmp3" {
		t.Fatalf("after a deletion = %q, want unnamed-tmp3", d3)
	}
}

func TestStaleQuickDocsAreOnlyReported(t *testing.T) {
	root := t.TempDir()
	oldDir, oldTex, _ := CreateQuickDoc(root, "viejo")
	_, _, _ = CreateQuickDoc(root, "nuevo")
	nineHoursAgo := time.Now().Add(-9 * time.Hour)
	if err := os.Chtimes(oldTex, nineHoursAgo, nineHoursAgo); err != nil {
		t.Fatal(err)
	}

	stale, err := StaleQuickDocs(root, QuickDocMaxAge, time.Now())
	if err != nil || len(stale) != 1 || stale[0].Path != oldDir {
		t.Fatalf("stale = %+v, %v", stale, err)
	}
	if _, err := os.Stat(oldTex); err != nil {
		t.Fatal("reporting must never delete")
	}
}

func TestListProjects(t *testing.T) {
	root := t.TempDir()
	if err := EnsureRepositoryLayout(root, false); err != nil {
		t.Fatal(err)
	}
	dir, mainTex, _ := CreateProject(root, "Tesis")
	_, _, _ = CreateQuickDoc(root, "x")

	projects, err := ListProjects(root)
	if err != nil || len(projects) != 1 || projects[0].Path != dir || projects[0].MainFile != mainTex {
		t.Fatalf("projects = %+v, %v (tmp-Workspaces and .creative-ink must be excluded)", projects, err)
	}
	if docs, _ := ListQuickDocs(root); len(docs) != 1 {
		t.Fatalf("quick docs = %+v", docs)
	}
}

func TestEscapeTeX(t *testing.T) {
	if got := EscapeTeX(`50% & $x_1$ #1 {a} ~^\`); got != `50\% \& \$x\_1\$ \#1 \{a\} \textasciitilde{}\textasciicircum{}\textbackslash{}` {
		t.Fatalf("EscapeTeX = %q", got)
	}
}
