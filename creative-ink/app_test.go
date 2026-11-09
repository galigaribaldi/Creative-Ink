package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v2/pkg/menu"

	"creative-ink/internal/config"
	"creative-ink/internal/core"
	"creative-ink/internal/system"
)

// emittedEvent is one call to App.emit.
type emittedEvent struct {
	name string
	data []any
}

// compileFunc matches compiler.CompileLaTeXContext.
type compileFunc func(ctx context.Context, workdir, filename string) (string, error)

func okCompile(ctx context.Context, workdir, filename string) (string, error) { return "ok", nil }

// newTestApp opens a temp folder with main.tex, a recording emitter and a
// fake compiler. Dialogs fail unless a test replaces them.
func newTestApp(t *testing.T, compile compileFunc) (*App, chan emittedEvent) {
	t.Helper()
	folder := t.TempDir()
	doc := filepath.Join(folder, "main.tex")
	if err := os.WriteFile(doc, []byte("\\documentclass{article}"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := newApp(folder, doc, compile)
	// These tests exercise compile-on-open: turn compile.auto on (the
	// product default is off, see TestAutoCompileIsOffByDefault).
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	if _, err := core.UpdateSettings(a.settingsPath, func(s *core.Settings) {
		s.SetUserConfig(config.CompileAuto, json.RawMessage("true"))
	}); err != nil {
		t.Fatal(err)
	}
	a.healthDelay = time.Hour // health checks are tested on their own
	events := make(chan emittedEvent, 32)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }
	a.pickFolder = func(string) (string, error) { return "", errors.New("unexpected folder dialog") }
	a.pickSavePath = func(string, string) (string, error) { return "", errors.New("unexpected save dialog") }
	a.pickNewPath = func(string, string) (string, error) { return "", errors.New("unexpected new-file dialog") }
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	return a, events
}

func nextEvent(t *testing.T, events chan emittedEvent) emittedEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an event")
		return emittedEvent{}
	}
}

func expectNoEvent(t *testing.T, events chan emittedEvent) {
	t.Helper()
	select {
	case ev := <-events:
		t.Fatalf("no event expected, got %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

// expectCompileOf asserts a started/finished pair for file.
func expectCompileOf(t *testing.T, events chan emittedEvent, file string) {
	t.Helper()
	ev := nextEvent(t, events)
	if ev.name != EventCompileStarted || ev.data[0].(CompileStartedEvent).File != file {
		t.Fatalf("started = %+v, want file %q", ev, file)
	}
	ev = nextEvent(t, events)
	if ev.name != EventCompileFinished {
		t.Fatalf("finished = %+v", ev)
	}
	if fin := ev.data[0].(CompileFinishedEvent); fin.File != file || !fin.Success {
		t.Fatalf("finished payload = %+v, want successful %q", fin, file)
	}
}

func TestSaveContentSavesWithoutCompiling(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	_, doc := a.workspace()

	res, err := a.SaveContent(doc, "nuevo")
	if err != nil || !res.Saved {
		t.Fatalf("SaveContent = %+v, %v", res, err)
	}
	if data, _ := os.ReadFile(doc); string(data) != "nuevo" {
		t.Fatalf("file = %q", data)
	}
	// Compiling is the UI's call (3 s idle or Cmd/Ctrl+S → RequestCompile).
	expectNoEvent(t, events)
}

func TestRequestCompileDoesNotBlock(t *testing.T) {
	release := make(chan struct{})
	a, events := newTestApp(t, func(ctx context.Context, workdir, filename string) (string, error) {
		<-release // a slow latexmk
		return "ok", nil
	})
	_, doc := a.workspace()

	done := make(chan bool, 1)
	go func() { done <- a.RequestCompile("manual") }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("RequestCompile rejected")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("RequestCompile blocked on the compilation")
	}
	close(release)
	expectCompileOf(t, events, doc)
}

func TestSaveContentValidatesPath(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	outside := filepath.Join(t.TempDir(), "x.tex")

	cases := map[string]error{
		outside:                              core.ErrOutsideFolder,
		filepath.Join(folder, "..", "x.tex"): core.ErrOutsideFolder,
		filepath.Join(folder, "main.pdf"):    core.ErrNotEditable,
	}
	for path, want := range cases {
		if _, err := a.SaveContent(path, "x"); !errors.Is(err, want) {
			t.Errorf("SaveContent(%q) err = %v, want %v", path, err, want)
		}
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("nothing may be written outside the open folder")
	}
	expectNoEvent(t, events)
}

func TestSaveContentOfAnotherFile(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	other := filepath.Join(folder, "chapter.tex")

	// A late autosave of a file that is no longer active: saved, not compiled.
	if _, err := a.SaveContent(other, "capítulo"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(other); string(data) != "capítulo" {
		t.Fatalf("other file = %q", data)
	}
	expectNoEvent(t, events)
}

func TestOpenFileSwitchesActiveDocument(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	chapter := filepath.Join(folder, "chapter.tex")
	if err := os.WriteFile(chapter, []byte("cap 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := a.OpenFile(chapter)
	if err != nil || got.Path != chapter || got.Content != "cap 1" {
		t.Fatalf("OpenFile = %+v, %v", got, err)
	}
	if _, doc := a.workspace(); doc != chapter {
		t.Fatalf("active doc = %q", doc)
	}
	if a.pdfPath() != filepath.Join(folder, "chapter.pdf") {
		t.Fatalf("preview must follow the active doc, got %q", a.pdfPath())
	}
	expectCompileOf(t, events, chapter)

	if _, err := a.OpenFile(filepath.Join(folder, "missing.tex")); err == nil {
		t.Error("opening a missing file must fail")
	}
	if _, err := a.OpenFile(filepath.Join(t.TempDir(), "x.tex")); !errors.Is(err, core.ErrOutsideFolder) {
		t.Errorf("outside folder: err = %v", err)
	}
	if _, doc := a.workspace(); doc != chapter {
		t.Fatal("a failed OpenFile must not change the active document")
	}
}

func TestOpenFolderDialog(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "tesis.tex"), []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.pickFolder = func(string) (string, error) { return "", nil }
	if st, err := a.OpenFolderDialog(); err != nil || !st.Canceled {
		t.Fatalf("cancel: %+v, %v", st, err)
	}

	a.pickFolder = func(string) (string, error) { return other, nil }
	st, err := a.OpenFolderDialog()
	if err != nil {
		t.Fatal(err)
	}
	if st.FolderPath != other || st.FilePath != filepath.Join(other, "tesis.tex") {
		t.Fatalf("state = %+v", st)
	}
	if len(st.Tree.Children) != 1 || st.Tree.Children[0].Name != "tesis.tex" {
		t.Fatalf("tree = %+v", st.Tree)
	}
}

func TestSaveAsDialog(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()

	// Inside the open folder, without extension → .tex is appended.
	a.pickSavePath = func(dir, name string) (string, error) {
		if dir != folder || name != "main.tex" {
			t.Errorf("dialog defaults = %q, %q", dir, name)
		}
		return filepath.Join(folder, "copia"), nil
	}
	st, err := a.SaveAsDialog("contenido")
	copyPath := filepath.Join(folder, "copia.tex")
	if err != nil || st.FilePath != copyPath || st.FolderPath != folder {
		t.Fatalf("SaveAs inside = %+v, %v", st, err)
	}
	if data, _ := os.ReadFile(copyPath); string(data) != "contenido" {
		t.Fatalf("copy = %q", data)
	}
	expectCompileOf(t, events, copyPath)

	// Outside the open folder → that folder becomes the open one.
	elsewhere := t.TempDir()
	a.pickSavePath = func(string, string) (string, error) { return filepath.Join(elsewhere, "otro.tex"), nil }
	st, err = a.SaveAsDialog("x")
	if err != nil || st.FolderPath != elsewhere || st.FilePath != filepath.Join(elsewhere, "otro.tex") {
		t.Fatalf("SaveAs outside = %+v, %v", st, err)
	}
	expectCompileOf(t, events, filepath.Join(elsewhere, "otro.tex"))

	// Wrong extension and cancel.
	a.pickSavePath = func(string, string) (string, error) { return filepath.Join(elsewhere, "a.pdf"), nil }
	if _, err := a.SaveAsDialog("x"); !errors.Is(err, core.ErrNotEditable) {
		t.Errorf(".pdf: err = %v", err)
	}
	a.pickSavePath = func(string, string) (string, error) { return "", nil }
	if st, err := a.SaveAsDialog("x"); err != nil || !st.Canceled {
		t.Errorf("cancel: %+v, %v", st, err)
	}
}

func TestRequestCompileQueuesWithoutSaving(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	_, doc := a.workspace()
	before, _ := os.Stat(doc)

	if !a.RequestCompile("manual") {
		t.Fatal("RequestCompile must be accepted while the app is running")
	}
	expectCompileOf(t, events, doc)
	if after, _ := os.Stat(doc); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("RequestCompile must not write the document")
	}

	a.compiles.Close(time.Second)
	if a.RequestCompile("manual") {
		t.Fatal("RequestCompile after shutdown must report false")
	}
}

// fakeRepoOptions proposes temp folders: a user one and, if shared, a shared one.
func fakeRepoOptions(t *testing.T, shared bool) (func() system.RepositoryOptions, string, string) {
	t.Helper()
	userPath := filepath.Join(t.TempDir(), "Documents", system.RepositoryName)
	sharedPath := filepath.Join(t.TempDir(), "Shared", system.RepositoryName)
	return func() system.RepositoryOptions {
		opts := system.RepositoryOptions{
			OS: "darwin", OSName: "macOS", Warnings: []string{},
			User:   system.RepositoryOption{Scope: system.ScopeUser, Path: userPath, Available: true},
			Shared: system.RepositoryOption{Scope: system.ScopeShared, Reason: "no hay carpeta compartida"},
		}
		if shared {
			opts.Shared = system.RepositoryOption{Scope: system.ScopeShared, Path: sharedPath, Available: true}
		}
		return opts
	}, userPath, sharedPath
}

// newFirstRunApp has no folder open and an empty settings location.
func newFirstRunApp(t *testing.T, shared bool) (*App, string, string) {
	t.Helper()
	a := newApp("", "", okCompile)
	a.healthDelay = time.Hour
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	var userPath, sharedPath string
	a.repoOptions, userPath, sharedPath = fakeRepoOptions(t, shared)
	a.settingsPath = filepath.Join(t.TempDir(), "creative-ink", "settings.json")
	return a, userPath, sharedPath
}

func TestFirstRunNeedsSetupAndCreatesNothing(t *testing.T) {
	// Simulate a Finder launch: the working directory must not matter.
	t.Chdir(t.TempDir())
	a, userPath, _ := newFirstRunApp(t, true)

	st, err := a.GetWorkspace()
	if err != nil || !st.NeedsSetup {
		t.Fatalf("first run = %+v, %v; want NeedsSetup", st, err)
	}
	if _, err := os.Stat(userPath); !os.IsNotExist(err) {
		t.Fatal("nothing may be created before the user chooses")
	}
	if _, err := os.Stat("workspace"); !os.IsNotExist(err) {
		t.Fatal("nothing may be created relative to the working directory")
	}
}

func TestSetupRepositoryCreatesLayoutAndShowsStart(t *testing.T) {
	a, userPath, _ := newFirstRunApp(t, true)

	st, err := a.SetupRepository(system.ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	// The root is a launcher: nothing is opened, the start screen follows.
	if st.NeedsSetup || !st.NeedsStart || st.FolderPath != "" || st.RepositoryPath != userPath {
		t.Fatalf("state = %+v", st)
	}
	for _, rel := range []string{".creative-ink/config.json", ".creative-ink/plugins", "tmp-Workspaces"} {
		if _, err := os.Stat(filepath.Join(userPath, rel)); err != nil {
			t.Errorf("%s missing: %v", rel, err)
		}
	}
	if scope, path := a.currentRepository(); scope != system.ScopeUser || path != userPath {
		t.Fatalf("saved = %q %q", scope, path)
	}

	// Next launch: no setup screen, straight to the start screen.
	next := newApp("", "", okCompile)
	t.Cleanup(func() { next.compiles.Close(time.Second) })
	next.settingsPath = a.settingsPath
	st, err = next.GetWorkspace()
	if err != nil || st.NeedsSetup || !st.NeedsStart || st.RepositoryPath != userPath {
		t.Fatalf("next launch = %+v, %v", st, err)
	}
}

func TestSetupRepositorySharedScope(t *testing.T) {
	a, _, sharedPath := newFirstRunApp(t, true)

	st, err := a.SetupRepository(system.ScopeShared)
	if err != nil || st.RepositoryPath != sharedPath {
		t.Fatalf("shared = %+v, %v", st, err)
	}
	if opts := a.GetRepositoryOptions(); opts.CurrentScope != system.ScopeShared || opts.CurrentPath != sharedPath {
		t.Fatalf("current = %q %q", opts.CurrentScope, opts.CurrentPath)
	}
	// Plugins are per-user: outside the shared repository.
	if _, err := os.Stat(filepath.Join(sharedPath, ".creative-ink", "plugins")); !os.IsNotExist(err) {
		t.Fatal("a shared repository must not hold plugins")
	}
	if got := a.pluginsDir(); got != filepath.Join(filepath.Dir(a.settingsPath), "plugins") || !dirExists(got) {
		t.Fatalf("plugins dir = %q", got)
	}
}

func TestSetupRepositoryRejectsUnavailableOrInvalidScope(t *testing.T) {
	a, _, _ := newFirstRunApp(t, false) // e.g. Linux: no shared folder

	if _, err := a.SetupRepository(system.ScopeShared); err == nil {
		t.Error("an unavailable scope must be rejected")
	}
	if _, err := a.SetupRepository("/etc"); err == nil {
		t.Error("an unknown scope must be rejected (paths never come from JS)")
	}
	if _, ok, _ := core.LoadSettings(a.settingsPath); ok {
		t.Error("nothing may be saved after a rejected setup")
	}
}

func TestLoadRepositoryKeepsUserFolder(t *testing.T) {
	a, _ := newTestApp(t, okCompile) // a folder is already open
	a.repoOptions, _, _ = fakeRepoOptions(t, false)
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := core.SaveSettings(a.settingsPath, core.Settings{RepositoryScope: system.ScopeUser, RepositoryPath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	before, _ := a.workspace()

	if _, err := a.loadRepository(); err != nil {
		t.Fatal(err)
	}
	if after, _ := a.workspace(); after != before {
		t.Fatalf("open folder changed from %q to %q", before, after)
	}
}

func TestEmitWailsIsNoOpBeforeStartup(t *testing.T) {
	// runtime.EventsEmit calls log.Fatalf with a nil context; this must not.
	(&App{}).emitWails(EventCompileStarted)
}

// newProjectApp is a test app whose folder has main.tex and chapters/intro.tex,
// with intro.tex open in the editor.
func newProjectApp(t *testing.T) (a *App, events chan emittedEvent, mainTex, chapter string) {
	t.Helper()
	a, events = newTestApp(t, okCompile)
	folder, mainTex := a.workspace()
	chapter = filepath.Join(folder, "chapters", "intro.tex")
	if err := os.MkdirAll(filepath.Dir(chapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chapter, []byte("capítulo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenFile(chapter); err != nil {
		t.Fatal(err)
	}
	expectCompileOf(t, events, chapter) // no main yet: the active doc compiles
	return a, events, mainTex, chapter
}

func TestSetMainFileCompilesMainWhileEditingAChapter(t *testing.T) {
	a, events, mainTex, chapter := newProjectApp(t)
	folder, _ := a.workspace()

	st, err := a.SetMainFile(mainTex)
	if err != nil || st.MainFilePath != mainTex || st.FilePath != chapter {
		t.Fatalf("SetMainFile = %+v, %v", st, err)
	}
	expectCompileOf(t, events, mainTex)

	// Saving the chapter does not compile; the UI's RequestCompile then
	// compiles the main document, not the chapter.
	if _, err := a.SaveContent(chapter, "capítulo 2"); err != nil {
		t.Fatal(err)
	}
	expectNoEvent(t, events)
	a.RequestCompile("manual")
	expectCompileOf(t, events, mainTex)
	if a.pdfPath() != filepath.Join(folder, "main.pdf") {
		t.Fatalf("preview must show the main PDF, got %q", a.pdfPath())
	}

	// Persisted, relative, in the hidden meta file.
	meta, err := core.LoadProjectMeta(folder)
	if err != nil || meta.MainFile != "main.tex" {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
}

func TestMainFileIsReloadedWithTheFolder(t *testing.T) {
	a, _, mainTex, _ := newProjectApp(t)
	folder, _ := a.workspace()
	if _, err := a.SetMainFile(mainTex); err != nil {
		t.Fatal(err)
	}

	// Another folder without meta: no main file there.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "x.tex"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.pickFolder = func(string) (string, error) { return other, nil }
	if st, err := a.OpenFolderDialog(); err != nil || st.MainFilePath != "" {
		t.Fatalf("other folder = %+v, %v", st, err)
	}

	// Back to the project: the main file comes back from .creative-ink-meta.
	a.pickFolder = func(string) (string, error) { return folder, nil }
	if st, err := a.OpenFolderDialog(); err != nil || st.MainFilePath != mainTex {
		t.Fatalf("project folder = %+v, %v", st, err)
	}
}

func TestSetMainFileValidationAndClear(t *testing.T) {
	a, events, mainTex, chapter := newProjectApp(t)
	folder, _ := a.workspace()

	bad := map[string]error{
		filepath.Join(t.TempDir(), "x.tex"):  core.ErrOutsideFolder,
		filepath.Join(folder, "notes.txt"):   core.ErrNotTeX,
		filepath.Join(folder, "missing.tex"): nil, // any error
	}
	for path, want := range bad {
		_, err := a.SetMainFile(path)
		if err == nil || (want != nil && !errors.Is(err, want)) {
			t.Errorf("SetMainFile(%q) err = %v, want %v", path, err, want)
		}
	}
	expectNoEvent(t, events)

	if _, err := a.SetMainFile(mainTex); err != nil {
		t.Fatal(err)
	}
	expectCompileOf(t, events, mainTex)
	st, err := a.SetMainFile("")
	if err != nil || st.MainFilePath != "" {
		t.Fatalf("clear = %+v, %v", st, err)
	}
	expectCompileOf(t, events, chapter) // back to the active document
}

func TestDeletedMainFileFallsBackToActiveDocument(t *testing.T) {
	a, events, mainTex, chapter := newProjectApp(t)
	if _, err := a.SetMainFile(mainTex); err != nil {
		t.Fatal(err)
	}
	expectCompileOf(t, events, mainTex)

	if err := os.Remove(mainTex); err != nil {
		t.Fatal(err)
	}
	if got := a.compileTarget(); got != chapter {
		t.Fatalf("compile target = %q, want the active chapter", got)
	}
	if st, _ := a.workspaceState(); st.MainFilePath != "" {
		t.Fatalf("a deleted main file must not be reported, got %q", st.MainFilePath)
	}
}

// fakeWatcher records Close and lets the test trigger onChange.
type fakeWatcher struct{ closed bool }

func (f *fakeWatcher) Close() error { f.closed = true; return nil }

func TestWorkspaceChangedOnlyWhenTheTreeChanges(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	var onChange func()
	watchers := []*fakeWatcher{}
	a.watchFolder = func(root string, cb func()) (io.Closer, error) {
		onChange = cb
		w := &fakeWatcher{}
		watchers = append(watchers, w)
		return w, nil
	}
	a.restartWatcher()
	folder, _ := a.workspace()

	onChange() // e.g. kqueue Remove+Create after an atomic save: same tree
	expectNoEvent(t, events)

	if err := os.WriteFile(filepath.Join(folder, "figura.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	onChange()
	ev := nextEvent(t, events)
	if ev.name != EventWorkspaceChanged || ev.data[0].(WorkspaceChangedEvent).Folder != folder {
		t.Fatalf("event = %+v", ev)
	}
	onChange() // nothing new since the last event
	expectNoEvent(t, events)

	// Opening another folder replaces the watcher; the old one is closed and
	// its late callbacks are ignored.
	staleChange := onChange
	other := t.TempDir()
	a.pickFolder = func(string) (string, error) { return other, nil }
	if _, err := a.OpenFolderDialog(); err != nil {
		t.Fatal(err)
	}
	if len(watchers) != 2 || !watchers[0].closed {
		t.Fatalf("watchers = %+v, want the first one closed", watchers)
	}
	if err := os.WriteFile(filepath.Join(folder, "otra.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleChange()
	expectNoEvent(t, events)

	a.stopWatching()
	if !watchers[1].closed {
		t.Fatal("shutdown must close the watcher")
	}
}

// newClosingApp records quits and before-close events.
func newClosingApp(t *testing.T) (*App, chan emittedEvent, *atomic.Int32) {
	t.Helper()
	a, events := newTestApp(t, okCompile)
	var quits atomic.Int32
	a.quit = func() {
		quits.Add(1)
		// runtime.Quit re-enters OnBeforeClose, which must now allow it.
		if a.beforeClose(nil) {
			t.Error("beforeClose must allow the close after it was confirmed")
		}
	}
	a.closeTimeout = time.Hour // tests opt in to the timeout
	return a, events, &quits
}

func TestBeforeCloseWaitsForTheUIFlush(t *testing.T) {
	a, events, quits := newClosingApp(t)

	if !a.beforeClose(nil) {
		t.Fatal("the first close request must be prevented")
	}
	if ev := nextEvent(t, events); ev.name != EventAppBeforeClose {
		t.Fatalf("event = %+v, want %s", ev, EventAppBeforeClose)
	}
	if !a.beforeClose(nil) {
		t.Fatal("a second close while flushing must still be prevented")
	}
	expectNoEvent(t, events) // no duplicate request to the UI
	if quits.Load() != 0 {
		t.Fatal("must not quit before the UI confirms")
	}

	a.ConfirmClose(true)
	if quits.Load() != 1 {
		t.Fatalf("quits = %d, want 1", quits.Load())
	}
	a.ConfirmClose(true) // late duplicate: ignored
	if quits.Load() != 1 {
		t.Fatal("must quit only once")
	}
}

func TestBeforeCloseTimeoutQuitsAnyway(t *testing.T) {
	a, _, quits := newClosingApp(t)
	a.closeTimeout = 30 * time.Millisecond

	a.beforeClose(nil) // the UI never answers
	deadline := time.Now().Add(2 * time.Second)
	for quits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if quits.Load() != 1 {
		t.Fatal("the timeout must quit when the UI does not confirm")
	}
	a.ConfirmClose(true) // too late: ignored
	if quits.Load() != 1 {
		t.Fatal("must quit only once")
	}
}

func TestConfirmCloseWithUnsavedEditsAsksFirst(t *testing.T) {
	a, events, quits := newClosingApp(t)
	var asked int
	a.confirmQuit = func(string) bool { asked++; return false } // "Cancelar"

	a.beforeClose(nil)
	nextEvent(t, events)
	a.ConfirmClose(false)
	if asked != 1 || quits.Load() != 0 {
		t.Fatalf("asked=%d quits=%d: declining must keep the window open", asked, quits.Load())
	}

	// The next close request starts over and asks the UI again.
	if !a.beforeClose(nil) {
		t.Fatal("must prevent again after a cancelled close")
	}
	nextEvent(t, events)
	a.confirmQuit = func(string) bool { return true } // "Cerrar sin guardar"
	a.ConfirmClose(false)
	if quits.Load() != 1 {
		t.Fatal("accepting must quit")
	}
}

func TestConfirmCloseWithoutCloseInProgressIsIgnored(t *testing.T) {
	a, _, quits := newClosingApp(t)
	a.ConfirmClose(true)
	if quits.Load() != 0 {
		t.Fatal("ConfirmClose must not quit unless a close was requested")
	}
}

func TestRunTerminalCommand(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("uses sh syntax")
	}
	a, _ := newTestApp(t, okCompile)
	folder, _ := a.workspace()

	res, err := a.RunTerminalCommand("ls; exit 2")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 2 || res.Folder != folder || !strings.Contains(res.Output, "main.tex") {
		t.Fatalf("result = %+v: must run in the open folder and report the exit code", res)
	}

	if _, err := a.RunTerminalCommand("  "); err == nil {
		t.Error("an empty command must be rejected")
	}
}

func TestRunTerminalCommandOneAtATimeAndCancel(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("uses sh syntax")
	}
	a, _ := newTestApp(t, okCompile)

	done := make(chan TerminalResult, 1)
	go func() {
		res, _ := a.RunTerminalCommand("sleep 20")
		done <- res
	}()
	// Wait until it is running.
	deadline := time.Now().Add(2 * time.Second)
	for {
		a.termMu.Lock()
		running := a.termCancel != nil
		a.termMu.Unlock()
		if running || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := a.RunTerminalCommand("echo otro"); !errors.Is(err, errTerminalBusy) {
		t.Fatalf("second command: err = %v, want errTerminalBusy", err)
	}

	a.CancelTerminalCommand()
	select {
	case res := <-done:
		if !res.Canceled || res.ExitCode != -1 {
			t.Fatalf("result = %+v, want canceled", res)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop the command")
	}

	// Free again.
	if res, err := a.RunTerminalCommand("echo libre"); err != nil || !strings.Contains(res.Output, "libre") {
		t.Fatalf("after cancel: %+v, %v", res, err)
	}
}

func TestRunTerminalCommandNeedsAFolder(t *testing.T) {
	a := newApp("", "", okCompile)
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	if _, err := a.RunTerminalCommand("ls"); !errors.Is(err, core.ErrNoFolder) {
		t.Fatalf("err = %v, want ErrNoFolder", err)
	}
}

func TestSyncTexBindingsWithoutDataAreSilent(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, doc := a.workspace()

	fwd, err := a.SyncTexForward(3, doc)
	if err != nil || fwd.Found || fwd.Reason == "" {
		t.Fatalf("forward = %+v, %v; want not found with a reason", fwd, err)
	}
	back, err := a.SyncTexBackward(1, 100, 100)
	if err != nil || back.Found || back.Reason == "" {
		t.Fatalf("backward = %+v, %v; want not found with a reason", back, err)
	}

	// Paths are validated like every other file binding.
	if _, err := a.SyncTexForward(1, filepath.Join(t.TempDir(), "x.tex")); !errors.Is(err, core.ErrOutsideFolder) {
		t.Errorf("outside folder: err = %v", err)
	}
	if _, err := a.SyncTexForward(1, filepath.Join(folder, "notes.txt")); !errors.Is(err, core.ErrNotTeX) {
		t.Errorf("not .tex: err = %v", err)
	}
}

func TestHybridTerminalRoutesCiInternally(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")

	// `ci` never reaches the shell: `ci help` works and lists the commands.
	res, err := a.RunTerminalCommand("  ci help")
	if err != nil || res.ExitCode != 0 || !strings.Contains(res.Output, "ci compile completo") {
		t.Fatalf("ci help = %+v, %v", res, err)
	}

	// Settings: persisted in settings.json and reflected in WorkspaceState.
	if res, _ := a.RunTerminalCommand("ci config autocompile off"); res.ExitCode != 0 {
		t.Fatalf("autocompile off = %+v", res)
	}
	if st, _ := a.workspaceState(); st.AutoCompile {
		t.Fatal("WorkspaceState.AutoCompile must reflect the saved setting")
	}
	if s, _ := core.ReadSettings(a.settingsPath); string(s.Config[config.CompileAuto]) != "false" {
		t.Fatal("the setting must be saved")
	}

	// Usage errors are terminal results with exit code 1, not rejections.
	res, err = a.RunTerminalCommand("ci compile volar")
	if err != nil || res.ExitCode != 1 || !strings.Contains(res.Output, "✗") {
		t.Fatalf("bad ci = %+v, %v", res, err)
	}
}

func TestHybridTerminalCiWorksWithoutAFolder(t *testing.T) {
	a := newApp("", "", okCompile)
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	if res, err := a.RunTerminalCommand("ci help"); err != nil || res.ExitCode != 0 {
		t.Fatalf("ci help without folder = %+v, %v", res, err)
	}
	if _, err := a.RunTerminalCommand("ls"); !errors.Is(err, core.ErrNoFolder) {
		t.Fatalf("shell commands still need a folder, err = %v", err)
	}
}

func TestCiBuildFailureEndsCompilingState(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	_, doc := a.workspace()
	env := a.ciEnv(0, nil)

	env.BuildStarted()
	env.BuildFinished(time.Second, errors.New("xelatex falló"), "▶ xelatex …")
	if ev := nextEvent(t, events); ev.name != EventCompileStarted || ev.data[0].(CompileStartedEvent).File != doc {
		t.Fatalf("started = %+v", ev)
	}
	fin := nextEvent(t, events).data[0].(CompileFinishedEvent)
	if fin.Success || fin.File != doc || !strings.Contains(fin.Logs, "xelatex falló") || fin.DurationMs != 1000 {
		t.Fatalf("finished = %+v", fin)
	}
}

// newRepoApp has a configured repository and no folder open (start screen).
func newRepoApp(t *testing.T) (*App, string) {
	t.Helper()
	a, userPath, _ := newFirstRunApp(t, false)
	if _, err := a.SetupRepository(system.ScopeUser); err != nil {
		t.Fatal(err)
	}
	return a, userPath
}

func TestLegacyRepositoryIsRenamed(t *testing.T) {
	docs := t.TempDir()
	legacy := filepath.Join(docs, system.LegacyRepositoryName)
	project := filepath.Join(legacy, "Tesis")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	a := newApp("", "", okCompile)
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := core.SaveSettings(a.settingsPath, core.Settings{
		RepositoryScope: system.ScopeUser, RepositoryPath: legacy, RecentFolders: []string{project},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := a.loadRepository(); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(docs, system.RepositoryName)
	if a.repository() != renamed || !dirExists(filepath.Join(renamed, "Tesis")) || dirExists(legacy) {
		t.Fatalf("repo = %q: the legacy folder must be renamed with its content", a.repository())
	}
	s, _ := core.ReadSettings(a.settingsPath)
	if s.RepositoryPath != renamed || s.RecentFolders[0] != filepath.Join(renamed, "Tesis") {
		t.Fatalf("settings not updated: %+v", s)
	}
}

func TestLegacyRepositoryNotRenamedWhenBothExist(t *testing.T) {
	docs := t.TempDir()
	legacy := filepath.Join(docs, system.LegacyRepositoryName)
	for _, d := range []string{legacy, filepath.Join(docs, system.RepositoryName)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a := newApp("", "", okCompile)
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	_ = core.SaveSettings(a.settingsPath, core.Settings{RepositoryScope: system.ScopeUser, RepositoryPath: legacy})

	if _, err := a.loadRepository(); err != nil {
		t.Fatal(err)
	}
	if a.repository() != legacy {
		t.Fatal("with both folders present nothing may be moved")
	}
	if screen, _ := a.GetStartScreen(); len(screen.Warnings) == 0 {
		t.Fatal("the user must be warned")
	}
}

func TestCreateProjectAndQuickDocument(t *testing.T) {
	a, root := newRepoApp(t)
	events := make(chan emittedEvent, 32)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }

	st, err := a.CreateProject("Tesis de Maestría")
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "Tesis-de-Maestria")
	if st.FolderPath != project || st.MainFilePath != filepath.Join(project, "main.tex") || st.NeedsStart {
		t.Fatalf("project state = %+v", st)
	}
	// compile.auto is off by default: a new project is not compiled (there
	// is no PDF to show yet either). ⌘S compiles it.
	expectNoEvent(t, events)

	st, err = a.CreateQuickDocument()
	quick := filepath.Join(root, "tmp-Workspaces", "unnamed-tmp1")
	if err != nil || st.FolderPath != quick || st.FilePath != filepath.Join(quick, "main.tex") {
		t.Fatalf("quick state = %+v, %v", st, err)
	}
	expectNoEvent(t, events)
	if !a.RequestCompile("manual") {
		t.Fatal("manual compile")
	}
	expectCompileOf(t, events, filepath.Join(quick, "main.tex"))

	screen, err := a.GetStartScreen()
	if err != nil || len(screen.Projects) != 1 || len(screen.QuickDocs) != 1 {
		t.Fatalf("start screen = %+v, %v", screen, err)
	}
	if len(screen.Recent) != 2 || screen.Recent[0] != quick {
		t.Fatalf("recent = %v, want the quick document first", screen.Recent)
	}
}

func TestOpenProjectOnlyFromTheRepositoryOrRecents(t *testing.T) {
	a, root := newRepoApp(t)
	if _, err := a.CreateProject("Tesis"); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "Tesis")

	if st, err := a.OpenProject(project); err != nil || st.MainFilePath != filepath.Join(project, "main.tex") {
		t.Fatalf("open project = %+v, %v", st, err)
	}
	for _, bad := range []string{root, filepath.Join(root, ".creative-ink"), filepath.Join(root, "tmp-Workspaces"), t.TempDir()} {
		if _, err := a.OpenProject(bad); err == nil {
			t.Errorf("OpenProject(%q) must be rejected", bad)
		}
	}

	// A folder opened with ⌘O becomes a recent and can be reopened.
	outside := t.TempDir()
	a.pickFolder = func(string) (string, error) { return outside, nil }
	if _, err := a.OpenFolderDialog(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenProject(outside); err != nil {
		t.Fatalf("a recent folder must be openable: %v", err)
	}
}

func TestStartScreenReportsStaleQuickDocs(t *testing.T) {
	a, root := newRepoApp(t)
	st, _ := a.CreateQuickDocument()
	old := time.Now().Add(-9 * time.Hour)
	if err := os.Chtimes(st.FilePath, old, old); err != nil {
		t.Fatal(err)
	}
	screen, err := a.GetStartScreen()
	if err != nil || len(screen.StaleQuickDocs) != 1 || screen.StaleAfterHours != 8 {
		t.Fatalf("screen = %+v, %v", screen, err)
	}
	if _, err := os.Stat(st.FilePath); err != nil {
		t.Fatal("stale quick documents are never deleted by the editor")
	}

	// Loose .tex files at the root are flagged (the root is only a launcher).
	if err := os.WriteFile(filepath.Join(root, "suelto.tex"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if screen, _ := a.GetStartScreen(); len(screen.Warnings) == 0 {
		t.Fatal("loose documents at the root must be reported")
	}
}

// ── Text files other than .tex, second pane, new files and menu ───────────

func TestOpenBibKeepsCompilingTheLastTeX(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, doc := a.workspace()
	bib := filepath.Join(folder, "referencias.bib")
	if err := os.WriteFile(bib, []byte("@book{clave, title={Título}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	opened, err := a.OpenFile(bib)
	if err != nil || opened.Content != "@book{clave, title={Título}}" {
		t.Fatalf("OpenFile(.bib) = %+v, %v", opened, err)
	}
	// The .bib is the active document, but main.tex is still compiled.
	expectCompileOf(t, events, doc)
	st, _ := a.workspaceState()
	if st.FilePath != bib || st.TexFilePath != doc {
		t.Fatalf("state = file %q, tex %q", st.FilePath, st.TexFilePath)
	}
	if _, err := a.SaveContent(bib, "@misc{otra}"); err != nil {
		t.Fatalf("SaveContent(.bib): %v", err)
	}
	if data, _ := os.ReadFile(bib); string(data) != "@misc{otra}" {
		t.Fatalf("bib = %q", data)
	}
	if _, err := a.SetMainFile(bib); !errors.Is(err, core.ErrNotTeX) {
		t.Errorf("a .bib can never be the main file: err = %v", err)
	}
}

func TestOpenFileRejectsBinaryAndNonUTF8(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	cases := map[string][]byte{
		"latin1.bib": {'@', 'b', 'o', 'o', 'k', '{', 'a', 0xF1, 'o', '}'}, // "año" in Latin-1
		"nul.txt":    {'a', 0, 'b'},
	}
	for name, data := range cases {
		path := filepath.Join(folder, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := a.OpenFile(path); !errors.Is(err, core.ErrNotUTF8) {
			t.Errorf("%s: err = %v, want ErrNotUTF8", name, err)
		}
	}
	if _, err := a.OpenFile(filepath.Join(folder, "main.pdf")); !errors.Is(err, core.ErrNotEditable) {
		t.Errorf("pdf: err = %v", err)
	}
	expectNoEvent(t, events)
}

func TestLoadFileDoesNotChangeTheActiveDocument(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, doc := a.workspace()
	other := filepath.Join(folder, "otro.sty")
	if err := os.WriteFile(other, []byte("\\ProvidesPackage{otro}"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := a.LoadFile(other)
	if err != nil || file.Path != other {
		t.Fatalf("LoadFile = %+v, %v", file, err)
	}
	if _, active := a.workspace(); active != doc {
		t.Fatalf("active = %q, want %q (the second pane must not change it)", active, doc)
	}
	if _, err := a.LoadFile(filepath.Join(t.TempDir(), "x.tex")); !errors.Is(err, core.ErrOutsideFolder) {
		t.Errorf("outside: err = %v", err)
	}
	expectNoEvent(t, events)
}

func TestNewFileDialog(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, doc := a.workspace()

	// The native dialog creates folders itself (CanCreateDirectories).
	if err := os.Mkdir(filepath.Join(folder, "capitulos"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.pickNewPath = func(dir, name string) (string, error) {
		if dir != folder || name != "nuevo-archivo.tex" {
			t.Errorf("dialog dir=%q name=%q", dir, name)
		}
		return filepath.Join(folder, "capitulos", "Capítulo Dos"), nil
	}
	st, err := a.NewFileDialog()
	want := filepath.Join(folder, "capitulos", "Capitulo-Dos.tex")
	if err != nil || st.FilePath != want || st.TexFilePath != want {
		t.Fatalf("NewFileDialog = %+v, %v; want %q (normalized, .tex by default)", st.FilePath, err, want)
	}
	if data, err := os.ReadFile(want); err != nil || len(data) != 0 {
		t.Fatalf("new file = %q, %v", data, err)
	}
	expectNoEvent(t, events) // an empty file is not compiled

	// Never replaces an existing file, even if the dialog confirmed it.
	a.pickNewPath = func(string, string) (string, error) { return doc, nil }
	if _, err := a.NewFileDialog(); !errors.Is(err, core.ErrFileExists) {
		t.Errorf("existing: err = %v", err)
	}
	for _, bad := range []string{filepath.Join(t.TempDir(), "fuera.tex"), filepath.Join(folder, "foto.png")} {
		a.pickNewPath = func(string, string) (string, error) { return bad, nil }
		if _, err := a.NewFileDialog(); err == nil {
			t.Errorf("%s must be rejected", bad)
		}
	}
	a.pickNewPath = func(string, string) (string, error) { return "", nil }
	if st, err := a.NewFileDialog(); err != nil || !st.Canceled {
		t.Errorf("cancel: %+v, %v", st, err)
	}
	a.setWorkspace("", "")
	if _, err := a.NewFileDialog(); !errors.Is(err, core.ErrNoFolder) {
		t.Errorf("no folder: err = %v", err)
	}
}

func TestMenuOnlyOnMacAndEmitsCommands(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	for _, goos := range []string{"linux", "windows"} {
		if m := a.buildMenu(goos); m != nil {
			t.Errorf("%s: a menu bar would change the window layout", goos)
		}
	}
	m := a.buildMenu("darwin")
	if m == nil {
		t.Fatal("macOS needs a menu (the Edit role enables ⌘C/⌘V)")
	}
	var newFile *menu.MenuItem
	hasEdit := false
	for _, top := range m.Items {
		hasEdit = hasEdit || top.Role == menu.EditMenuRole
		if top.SubMenu == nil {
			continue
		}
		for _, item := range top.SubMenu.Items {
			if item.Label == "Nuevo archivo…" {
				newFile = item
			}
		}
	}
	if !hasEdit || newFile == nil || newFile.Accelerator == nil || newFile.Accelerator.Key != "n" {
		t.Fatalf("edit=%v newFile=%+v", hasEdit, newFile)
	}
	newFile.Click(&menu.CallbackData{MenuItem: newFile})
	ev := nextEvent(t, events)
	if ev.name != EventMenuCommand || ev.data[0].(MenuCommandEvent).ID != "new-file" {
		t.Fatalf("event = %+v", ev)
	}
}

// ── Config service, auto-compile gate, terminal spool, project health ─────

func TestAutoCompileIsOffByDefault(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	a.settingsPath = filepath.Join(t.TempDir(), "vacio.json") // no user value: default
	folder, doc := a.workspace()
	if a.autoCompileEnabled() {
		t.Fatal("compile.auto must default to off")
	}

	// Opening a file compiles nothing; an existing PDF is shown as cached.
	if err := os.WriteFile(filepath.Join(folder, "main.pdf"), []byte("%PDF-1.7"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenFile(doc); err != nil {
		t.Fatal(err)
	}
	ev := nextEvent(t, events)
	fin, ok := ev.data[0].(CompileFinishedEvent)
	if ev.name != EventCompileFinished || !ok || !fin.Cached || !fin.Success || fin.File != doc {
		t.Fatalf("event = %+v, want a cached preview of main.pdf", ev)
	}
	expectNoEvent(t, events) // no compile:started: latexmk never ran

	// Go rejects automatic requests (second lock); manual ones always run.
	if a.RequestCompile("auto") || a.RequestCompile("otro") {
		t.Fatal("auto (with compile.auto off) and unknown triggers must be rejected")
	}
	if !a.RequestCompile("manual") {
		t.Fatal("⌘S must always compile")
	}
	expectCompileOf(t, events, doc)
}

func TestSetConfigLayersAndEvent(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, _ := a.workspace()

	e, err := a.SetConfig("autocompile", "off", "project")
	if err != nil || e.Value != false || e.Scope != config.ScopeProject {
		t.Fatalf("project = %+v, %v", e, err)
	}
	ev := nextEvent(t, events)
	if changed := ev.data[0].(ConfigChangedEvent); ev.name != EventConfigChanged || changed.Written != config.ScopeProject || changed.Folder != folder {
		t.Fatalf("event = %+v", ev)
	}
	if a.autoCompileEnabled() {
		t.Fatal("the project layer must win over the user's (on)")
	}
	if meta, _ := core.LoadProjectMeta(folder); string(meta.Config[config.CompileAuto]) != "false" {
		t.Fatalf("meta = %+v", meta)
	}
	if e, _ := a.SetConfig(config.CompileAuto, "", "project"); e.Value != true || e.Scope != config.ScopeUser {
		t.Fatalf("after unset = %+v (back to the user value)", e)
	}
	for _, bad := range [][3]string{{"nada", "on", "user"}, {"compile.auto", "quizá", "user"}, {"compile.auto", "on", "default"}} {
		if _, err := a.SetConfig(bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("SetConfig%v must fail", bad)
		}
	}
	if st, _ := a.workspaceState(); !st.AutoCompile {
		t.Fatal("WorkspaceState.AutoCompile is the effective value")
	}
}

func TestTerminalOutputIsSpooled(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	spool, err := core.NewOutputSpool()
	if err != nil {
		t.Fatal(err)
	}
	a.spool = spool
	t.Cleanup(func() { _ = spool.Close() })

	res, err := a.RunTerminalCommand("echo hola-spool")
	if err != nil || res.RunID == 0 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	ev := nextEvent(t, events)
	if started := ev.data[0].(TerminalStartedEvent); ev.name != EventTerminalStarted || started.RunID != res.RunID {
		t.Fatalf("event = %+v", ev)
	}
	chunk, err := a.ReadTerminalOutput(res.RunID, 0)
	if err != nil || !strings.Contains(chunk.Data, "hola-spool") || !chunk.Done {
		t.Fatalf("chunk = %+v, %v", chunk, err)
	}
	if _, err := a.ReadTerminalOutput(999, 0); !errors.Is(err, core.ErrUnknownRun) {
		t.Fatalf("unknown run: %v", err)
	}
}

func TestProjectHealthWarnsWithoutChangingAnything(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, doc := a.workspace()
	body := "\\documentclass{article}\n" + strings.Repeat("\\includegraphics{fig}\n", 61)
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	a.checkProjectHealth()
	ev := nextEvent(t, events)
	h, ok := ev.data[0].(core.ProjectHealth)
	if ev.name != EventProjectHealth || !ok || h.Folder != folder || len(h.Warnings) != 1 || h.Warnings[0].Kind != core.HealthManyImages {
		t.Fatalf("event = %+v", ev)
	}
	if got := a.GetProjectHealth(); len(got.Warnings) != 1 {
		t.Fatalf("GetProjectHealth = %+v", got)
	}
	a.checkProjectHealth() // same result: no new event
	expectNoEvent(t, events)
}

// ── Archetype tests (testify) ──────────────────────────────────────────────

// TestCompileTargetResolution: what is compiled and previewed. Without
// .creative-ink-meta it is the active document; with a meta pointing to
// main.tex, main.tex wins even while a chapter is being edited.
func TestCompileTargetResolution(t *testing.T) {
	newProject := func(t *testing.T, withMeta bool) (folder, mainTex, chapter string) {
		t.Helper()
		folder = t.TempDir()
		mainTex = filepath.Join(folder, "main.tex")
		chapter = filepath.Join(folder, "capitulos", "cap1.tex")
		require.NoError(t, os.MkdirAll(filepath.Dir(chapter), 0o755))
		require.NoError(t, os.WriteFile(mainTex, []byte("\\input{capitulos/cap1}"), 0o644))
		require.NoError(t, os.WriteFile(chapter, []byte("capítulo"), 0o644))
		if withMeta {
			require.NoError(t, os.WriteFile(filepath.Join(folder, core.MetaFileName), []byte(`{"version":1,"mainFile":"main.tex"}`), 0o644))
		}
		return folder, mainTex, chapter
	}
	open := func(t *testing.T, folder, doc string) *App {
		a := newApp(folder, doc, okCompile)
		t.Cleanup(func() { a.compiles.Close(time.Second) })
		return a
	}

	t.Run("without meta: the active document", func(t *testing.T) {
		folder, _, chapter := newProject(t, false)
		a := open(t, folder, chapter)
		assert.Equal(t, chapter, a.compileTarget())
		assert.Equal(t, filepath.Join(folder, "capitulos", "cap1.pdf"), a.pdfPath())
	})

	t.Run("with meta: main.tex even while editing a chapter", func(t *testing.T) {
		folder, mainTex, chapter := newProject(t, true)
		a := open(t, folder, chapter)
		_, active := a.workspace()
		assert.Equal(t, chapter, active, "the chapter stays the active document")
		assert.Equal(t, mainTex, a.compileTarget())
		assert.Equal(t, filepath.Join(folder, "main.pdf"), a.pdfPath())
	})

	t.Run("meta pointing to a missing file falls back to the active document", func(t *testing.T) {
		folder, mainTex, chapter := newProject(t, true)
		require.NoError(t, os.Remove(mainTex))
		a := open(t, folder, chapter)
		assert.Equal(t, chapter, a.compileTarget())
	})

	t.Run("a .bib keeps the last .tex as target", func(t *testing.T) {
		folder, _, chapter := newProject(t, false)
		bib := filepath.Join(folder, "referencias.bib")
		require.NoError(t, os.WriteFile(bib, []byte("@misc{a}"), 0o644))
		a := open(t, folder, chapter)
		a.setWorkspace(folder, bib)
		assert.Equal(t, chapter, a.compileTarget())
	})
}

// TestRapidAutoRequestsCancelTheObsoleteRun: three automatic requests in
// well under 500 ms. The run in progress is cancelled (its context is done),
// the others coalesce, and only the last run finishes — with the newest
// content on disk and the right `file` in compile:started/finished.
func TestRapidAutoRequestsCancelTheObsoleteRun(t *testing.T) {
	firstStarted := make(chan struct{})
	firstDying := make(chan struct{}) // latexmk takes a moment to die
	var runs atomic.Int32
	var firstCanceled atomic.Bool
	compile := func(ctx context.Context, workdir, filename string) (string, error) {
		data, err := os.ReadFile(filepath.Join(workdir, filename)) // what latexmk would read
		if err != nil {
			return "", err
		}
		if runs.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
			firstCanceled.Store(true)
			<-firstDying
			return "", fmt.Errorf("compilación cancelada: %w", ctx.Err())
		}
		return string(data), nil
	}
	a, events := newTestApp(t, compile)
	_, doc := a.workspace()
	save := func(v string) { require.NoError(t, os.WriteFile(doc, []byte(v), 0o644)) }

	start := time.Now()
	save("v1")
	require.True(t, a.RequestCompile("auto"))
	<-firstStarted
	save("v2")
	require.True(t, a.RequestCompile("auto")) // cancels run 1 (young, automatic)
	save("v3")
	require.True(t, a.RequestCompile("auto")) // coalesces into the follow-up
	assert.Less(t, time.Since(start), 500*time.Millisecond)
	close(firstDying)

	ev := nextEvent(t, events)
	require.Equal(t, EventCompileStarted, ev.name)
	assert.Equal(t, doc, ev.data[0].(CompileStartedEvent).File)

	ev = nextEvent(t, events)
	require.Equal(t, EventCompileFinished, ev.name)
	canceled := ev.data[0].(CompileFinishedEvent)
	assert.True(t, canceled.Canceled, "the obsolete run must be reported as cancelled")
	assert.False(t, canceled.Success)
	assert.Zero(t, canceled.PDFVersion, "a cancelled run publishes no PDF")
	assert.True(t, firstCanceled.Load(), "its context must be cancelled (latexmk killed)")

	ev = nextEvent(t, events)
	require.Equal(t, EventCompileStarted, ev.name)
	assert.Equal(t, doc, ev.data[0].(CompileStartedEvent).File)

	ev = nextEvent(t, events)
	require.Equal(t, EventCompileFinished, ev.name)
	last := ev.data[0].(CompileFinishedEvent)
	assert.True(t, last.Success)
	assert.False(t, last.Canceled)
	assert.Equal(t, doc, last.File)
	assert.Equal(t, "v3", last.Logs, "the last run must compile the newest content")

	expectNoEvent(t, events) // 3 requests → 2 runs: no third compilation
	assert.EqualValues(t, 2, runs.Load())
}
