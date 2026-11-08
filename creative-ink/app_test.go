package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	events := make(chan emittedEvent, 32)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }
	a.pickFolder = func(string) (string, error) { return "", errors.New("unexpected folder dialog") }
	a.pickSavePath = func(string, string) (string, error) { return "", errors.New("unexpected save dialog") }
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
	go func() { done <- a.RequestCompile() }()
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
		filepath.Join(folder, "notes.txt"):   core.ErrNotTeX,
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
	a.pickSavePath = func(string, string) (string, error) { return filepath.Join(elsewhere, "a.txt"), nil }
	if _, err := a.SaveAsDialog("x"); !errors.Is(err, core.ErrNotTeX) {
		t.Errorf(".txt: err = %v", err)
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

	if !a.RequestCompile() {
		t.Fatal("RequestCompile must be accepted while the app is running")
	}
	expectCompileOf(t, events, doc)
	if after, _ := os.Stat(doc); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("RequestCompile must not write the document")
	}

	a.compiles.Close(time.Second)
	if a.RequestCompile() {
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

func TestSetupRepositoryUserScopePersists(t *testing.T) {
	a, userPath, _ := newFirstRunApp(t, true)

	st, err := a.SetupRepository(system.ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if st.NeedsSetup || st.FolderPath != userPath || st.FilePath != filepath.Join(userPath, "main.tex") {
		t.Fatalf("state = %+v", st)
	}
	if data, _ := os.ReadFile(st.FilePath); string(data) != starterDocument {
		t.Fatalf("starter = %q", data)
	}
	if scope, path := a.currentRepository(); scope != system.ScopeUser || path != userPath {
		t.Fatalf("saved = %q %q", scope, path)
	}

	// Next launch: the saved choice is reused, no setup screen.
	next := newApp("", "", okCompile)
	t.Cleanup(func() { next.compiles.Close(time.Second) })
	next.settingsPath = a.settingsPath
	st, err = next.GetWorkspace()
	if err != nil || st.NeedsSetup || st.FolderPath != userPath {
		t.Fatalf("next launch = %+v, %v", st, err)
	}
}

func TestSetupRepositorySharedScope(t *testing.T) {
	a, _, sharedPath := newFirstRunApp(t, true)

	st, err := a.SetupRepository(system.ScopeShared)
	if err != nil || st.FolderPath != sharedPath {
		t.Fatalf("shared = %+v, %v", st, err)
	}
	if opts := a.GetRepositoryOptions(); opts.CurrentScope != system.ScopeShared || opts.CurrentPath != sharedPath {
		t.Fatalf("current = %q %q", opts.CurrentScope, opts.CurrentPath)
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

func TestConfiguredRepositoryKeepsUserFolder(t *testing.T) {
	a, _ := newTestApp(t, okCompile) // a folder is already open
	a.repoOptions, _, _ = fakeRepoOptions(t, false)
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := core.SaveSettings(a.settingsPath, core.Settings{RepositoryScope: system.ScopeUser, RepositoryPath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	before, _ := a.workspace()

	if _, err := a.openConfiguredRepository(); err != nil {
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
	a.RequestCompile()
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
	if s, _ := core.ReadSettings(a.settingsPath); s.AutoCompileEnabled() {
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
	env := a.ciEnv()

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
