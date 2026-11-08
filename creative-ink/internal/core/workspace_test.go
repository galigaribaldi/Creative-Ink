package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// mkfiles creates files (and their parent dirs) under root.
func mkfiles(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(root, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func names(nodes []FileNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

func TestScanTreeOrderAndFilters(t *testing.T) {
	root := t.TempDir()
	mkfiles(t, root,
		"main.tex", "Apendice.tex", "refs.bib", "main.pdf",
		"main.aux", "main.log", "main.fdb_latexmk", "main.synctex.gz", ".hidden.tex",
		"chapters/intro.tex", "node_modules/x.js", ".git/config", "figs/a.png")

	tree, truncated, err := ScanTree(root)
	if err != nil || truncated {
		t.Fatalf("ScanTree: err=%v truncated=%v", err, truncated)
	}

	got := names(tree.Children)
	want := []string{"chapters", "figs", "Apendice.tex", "main.pdf", "main.tex", "refs.bib"}
	if len(got) != len(want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children = %v, want %v", got, want)
		}
	}

	byName := map[string]FileNode{}
	for _, n := range tree.Children {
		byName[n.Name] = n
	}
	if !byName["main.tex"].Editable || byName["refs.bib"].Editable || byName["chapters"].Editable {
		t.Errorf("only .tex files must be editable: %+v", byName)
	}
	if ch := byName["chapters"].Children; len(ch) != 1 || ch[0].Path != filepath.Join(root, "chapters", "intro.tex") {
		t.Errorf("nested children = %+v", ch)
	}
}

func TestScanTreeTruncatesDeepTrees(t *testing.T) {
	root := t.TempDir()
	deep := root
	for i := 0; i < maxTreeDepth+2; i++ {
		deep = filepath.Join(deep, "d")
	}
	mkfiles(t, deep, "x.tex")

	if _, truncated, err := ScanTree(root); err != nil || !truncated {
		t.Fatalf("deep tree: err=%v truncated=%v, want truncated", err, truncated)
	}
}

func TestDefaultTeXFile(t *testing.T) {
	root := t.TempDir()
	if got := DefaultTeXFile(root); got != "" {
		t.Fatalf("empty folder = %q, want \"\"", got)
	}
	mkfiles(t, root, "b.tex", "a.tex")
	if got := DefaultTeXFile(root); got != filepath.Join(root, "a.tex") {
		t.Fatalf("without main.tex = %q, want a.tex", got)
	}
	mkfiles(t, root, "main.tex")
	if got := DefaultTeXFile(root); got != filepath.Join(root, "main.tex") {
		t.Fatalf("with main.tex = %q", got)
	}
}

func TestResolveInFolder(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mkfiles(t, root, "main.tex", "sub/c.tex")
	mkfiles(t, outside, "secret.tex")
	if err := os.Symlink(filepath.Join(outside, "secret.tex"), filepath.Join(root, "link.tex")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}

	ok := []string{
		filepath.Join(root, "main.tex"),
		filepath.Join(root, "sub", "c.tex"),
		filepath.Join(root, "new.tex"),            // does not exist yet (Save As)
		filepath.Join(root, "sub", "..", "x.tex"), // cleaned back inside
	}
	for _, p := range ok {
		if _, err := ResolveInFolder(root, p); err != nil {
			t.Errorf("ResolveInFolder(%q) = %v, want ok", p, err)
		}
	}

	bad := map[string]error{
		filepath.Join(root, "..", "evil.tex"):        ErrOutsideFolder,
		filepath.Join(outside, "secret.tex"):         ErrOutsideFolder,
		filepath.Join(root, "link.tex"):              ErrOutsideFolder, // symlinked file escapes
		filepath.Join(root, "linkdir", "secret.tex"): ErrOutsideFolder, // symlinked dir escapes
	}
	for p, want := range bad {
		if _, err := ResolveInFolder(root, p); !errors.Is(err, want) {
			t.Errorf("ResolveInFolder(%q) = %v, want %v", p, err, want)
		}
	}
	if _, err := ResolveInFolder(root, "main.tex"); err == nil {
		t.Error("relative paths must be rejected")
	}
	if _, err := ResolveInFolder("", filepath.Join(root, "main.tex")); !errors.Is(err, ErrNoFolder) {
		t.Errorf("no folder: err = %v", err)
	}
}

func TestPrepareRepositoryCreatesFolderAndStarter(t *testing.T) {
	// Missing parents too: ~/Documents may not exist on a fresh Linux install.
	dir := filepath.Join(t.TempDir(), "Documents", "My Creative Ink Repository")

	doc, err := PrepareRepository(dir, "starter", false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("repository not created: %v", err)
	}
	if extra := info.Mode().Perm() &^ 0o755; extra != 0 {
		t.Errorf("permissions = %o, must not exceed 0755", info.Mode().Perm())
	}
	if doc != filepath.Join(dir, "main.tex") {
		t.Fatalf("doc = %q", doc)
	}
	if data, _ := os.ReadFile(doc); string(data) != "starter" {
		t.Fatalf("starter content = %q", data)
	}
}

func TestPrepareRepositoryNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, "main.tex")
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte("mío"), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := PrepareRepository(dir, "starter", false)
	if err != nil || doc != filepath.Join(dir, "main.tex") {
		t.Fatalf("PrepareRepository = %q, %v", doc, err)
	}
	if data, _ := os.ReadFile(doc); string(data) != "mío" {
		t.Fatal("an existing document must never be overwritten")
	}
}

func TestPrepareRepositoryKeepsExistingTeX(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, "tesis.tex", "figs/a.png")

	doc, err := PrepareRepository(dir, "starter", false)
	if err != nil || doc != filepath.Join(dir, "tesis.tex") {
		t.Fatalf("PrepareRepository = %q, %v", doc, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.tex")); !os.IsNotExist(err) {
		t.Fatal("no main.tex must be created when the folder already has a .tex")
	}
}

func TestPrepareRepositorySharedIsWritableByAll(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows relies on Public Documents ACLs")
	}
	dir := filepath.Join(t.TempDir(), "shared")
	if _, err := PrepareRepository(dir, "starter", true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o777 || info.Mode()&os.ModeSticky == 0 {
		t.Fatalf("mode = %v, want drwxrwxrwt", info.Mode())
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creative-ink", "settings.json")

	if _, ok, err := LoadSettings(path); ok || err != nil {
		t.Fatalf("first run: ok=%v err=%v, want not configured", ok, err)
	}
	want := Settings{RepositoryScope: "user", RepositoryPath: "/Users/ana/Documents/My Creative Ink Repository"}
	if err := SaveSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadSettings(path)
	want.Version = settingsVersion
	if err != nil || !ok || got != want {
		t.Fatalf("LoadSettings = %+v, %v, %v; want %+v", got, ok, err, want)
	}

	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := LoadSettings(path); ok || err == nil {
		t.Fatal("a corrupt file must be reported and treated as not configured")
	}
	if err := os.WriteFile(path, []byte(`{"repositoryPath":"relativo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := LoadSettings(path); ok {
		t.Fatal("a relative repository path must be ignored")
	}
}

func TestUpdateSettingsKeepsOtherFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creative-ink", "settings.json")
	if err := SaveSettings(path, Settings{RepositoryScope: "user", RepositoryPath: "/r"}); err != nil {
		t.Fatal(err)
	}
	if (Settings{}).AutoCompileEnabled() != true {
		t.Fatal("a missing autoCompile must mean on")
	}

	off := false
	if _, err := UpdateSettings(path, func(s *Settings) { s.AutoCompile = &off }); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadSettings(path)
	if err != nil || !ok || got.RepositoryPath != "/r" || got.AutoCompileEnabled() {
		t.Fatalf("settings = %+v, ok=%v, err=%v", got, ok, err)
	}

	// Works before the first-run setup too (no file yet).
	fresh := filepath.Join(t.TempDir(), "settings.json")
	on := true
	if s, err := UpdateSettings(fresh, func(s *Settings) { s.AutoCompile = &on }); err != nil || !s.AutoCompileEnabled() {
		t.Fatalf("fresh = %+v, %v", s, err)
	}
}
