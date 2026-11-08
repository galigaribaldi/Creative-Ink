package core

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchDebounce is how long the watcher waits for a burst of filesystem
// events to settle before notifying (an unzip or a `git checkout` emits
// hundreds of events in a few milliseconds).
const WatchDebounce = 300 * time.Millisecond

// maxWatchedDirs bounds the number of watches. fsnotify is not recursive
// (one watch per directory), and on macOS its kqueue backend opens a file
// descriptor for every watched directory and for every file inside it.
const maxWatchedDirs = 256

// FolderWatcher reports structural changes (files or folders created,
// removed or renamed) inside a folder and its subfolders. It has no Wails
// dependency: the App decides what to do in onChange.
type FolderWatcher struct {
	root      string
	watcher   *fsnotify.Watcher
	debounce  time.Duration
	onChange  func()
	done      chan struct{}
	stopped   sync.WaitGroup
	closeOnce sync.Once
}

// WatchFolder starts watching root. onChange runs on the watcher goroutine,
// at most once per debounce window, after relevant events settle.
func WatchFolder(root string, debounce time.Duration, onChange func()) (*FolderWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("start watcher: %w", err)
	}
	if err := w.Add(root); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("watch %q: %w", root, err)
	}

	fw := &FolderWatcher{
		root:     root,
		watcher:  w,
		debounce: debounce,
		onChange: onChange,
		done:     make(chan struct{}),
	}
	fw.addSubdirs(root, 1)

	fw.stopped.Add(1)
	go fw.loop()
	return fw, nil
}

// Close stops the watcher and waits for its goroutine. Safe to call twice.
func (fw *FolderWatcher) Close() error {
	var err error
	fw.closeOnce.Do(func() {
		close(fw.done)
		err = fw.watcher.Close()
		fw.stopped.Wait()
	})
	return err
}

// addSubdirs watches the subfolders of dir, recursively, with the same
// filters and depth limit as ScanTree, until maxWatchedDirs is reached.
func (fw *FolderWatcher) addSubdirs(dir string, depth int) {
	if depth > maxTreeDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || isIgnoredEntry(e.Name(), true) {
			continue
		}
		if len(fw.watcher.WatchList()) >= maxWatchedDirs {
			log.Printf("[watcher] límite de %d carpetas vigiladas en %q", maxWatchedDirs, fw.root)
			return
		}
		sub := filepath.Join(dir, e.Name())
		if err := fw.watcher.Add(sub); err == nil {
			fw.addSubdirs(sub, depth+1)
		}
	}
}

// depthOf mirrors ScanTree's depth: direct children of root are depth 1.
func (fw *FolderWatcher) depthOf(path string) int {
	rel, err := filepath.Rel(fw.root, path)
	if err != nil {
		return maxTreeDepth + 1
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

func (fw *FolderWatcher) loop() {
	defer fw.stopped.Done()
	// A panic here would kill the whole app (Go does not recover goroutines).
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[watcher] panic recuperado: %v", r)
		}
	}()

	timer := time.NewTimer(fw.debounce)
	timer.Stop()
	pending := false

	for {
		select {
		case <-fw.done:
			timer.Stop()
			return

		case ev, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if !isRelevantEvent(ev) {
				continue
			}
			// fsnotify is not recursive: start watching folders created inside.
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					depth := fw.depthOf(ev.Name)
					if depth <= maxTreeDepth && len(fw.watcher.WatchList()) < maxWatchedDirs &&
						fw.watcher.Add(ev.Name) == nil {
						fw.addSubdirs(ev.Name, depth+1)
					}
				}
			}
			pending = true
			timer.Reset(fw.debounce)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] %v", err)
			// Lost events: refresh anyway rather than show a stale tree.
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				pending = true
				timer.Reset(fw.debounce)
			}

		case <-timer.C:
			if pending {
				pending = false
				fw.onChange()
			}
		}
	}
}

// isRelevantEvent keeps only structural changes to entries the explorer
// shows. Writes (autosave, latexmk) and build artifacts are noise.
func isRelevantEvent(ev fsnotify.Event) bool {
	if !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Remove) && !ev.Has(fsnotify.Rename) {
		return false
	}
	return !isIgnoredEntry(filepath.Base(ev.Name), false)
}

// isIgnoredEntry is the filter shared by ScanTree and the watcher: hidden
// entries, skipped folders and LaTeX build artifacts. For events about a
// removed path we cannot stat it, so artifacts are checked by name.
func isIgnoredEntry(name string, isDir bool) bool {
	if strings.HasPrefix(name, ".") || skippedDirs[name] {
		return true
	}
	return !isDir && isBuildArtifact(name)
}

// TreeSignature summarises a tree (paths, kinds, truncation) so callers can
// tell whether a rescan actually changed what the explorer shows.
func TreeSignature(tree FileNode, truncated bool) uint64 {
	h := fnv.New64a()
	var walk func(n FileNode)
	walk = func(n FileNode) {
		h.Write([]byte(n.Path))
		if n.IsDir {
			h.Write([]byte{'/'})
		}
		h.Write([]byte{0})
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(tree)
	if truncated {
		h.Write([]byte("truncated"))
	}
	return h.Sum64()
}
