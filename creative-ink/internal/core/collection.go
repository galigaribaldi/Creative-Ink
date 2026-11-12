package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Collections of documents (docs/MAP.md §4.7): a folder such as
// Trabajos-Maestria-Urbanismo holds many documents, each in its own
// subfolder with its own main.tex. Opened in place, every document compiles
// on its own: the compile target is the root document nearest to the file
// being edited, never a lone chapter.

// Bounds of the collection check (it runs whenever a folder is opened).
const (
	maxCollectionEntries = 20_000
	maxCollectionDepth   = 8
)

var errEnough = errors.New("enough")

// IsRootDocument reports whether path is a .tex that starts a document (a
// \documentclass in its first 64 KB, standalone figures included).
func IsRootDocument(path string) bool {
	if !IsTeXFile(path) {
		return false
	}
	_, ok := documentClass(path)
	return ok
}

// RootDocumentFor returns the document that compiles tex inside folder:
// tex itself if it has a \documentclass; otherwise, from its folder up to
// folder, the first main.tex that is a root document, or the only root
// document of a folder. preferred (the last root document opened) breaks a
// tie between several in the same folder. Without any, tex itself (the
// behaviour before collections existed).
func RootDocumentFor(folder, tex, preferred string) string {
	if tex == "" || IsRootDocument(tex) {
		return tex
	}
	for dir := filepath.Dir(tex); isInside(folder, dir); dir = filepath.Dir(dir) {
		if main := filepath.Join(dir, "main.tex"); isRegularFile(main) && IsRootDocument(main) {
			return main
		}
		var roots []string
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if p := filepath.Join(dir, e.Name()); !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && IsRootDocument(p) {
				roots = append(roots, p)
			}
		}
		switch {
		case len(roots) == 1:
			return roots[0]
		case len(roots) > 1:
			for _, r := range roots {
				if r == preferred {
					return r
				}
			}
			return tex // ambiguous: the user picks one by opening it (or ★)
		}
		if dir == filepath.Clean(folder) || filepath.Dir(dir) == dir {
			break
		}
	}
	return tex
}

// CollectionInfo describes a collection of documents.
type CollectionInfo struct {
	Collection bool // no document at the top, documents in 2+ subfolders
	Documents  int  // documents found (a lower bound: the scan stops early)
}

// DetectCollection decides whether folder is a collection of documents. It
// stops as soon as it knows: a root document at the top level means "no"
// at once, and two documents in different subfolders mean "yes". Bounded
// in entries and depth, so opening a huge folder stays fast.
func DetectCollection(folder string) CollectionInfo {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return CollectionInfo{}
	}
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && isRootNonStandalone(filepath.Join(folder, e.Name())) {
			return CollectionInfo{}
		}
	}
	var info CollectionInfo
	dirs := map[string]bool{}
	seen := 0
	_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if seen++; seen > maxCollectionEntries {
			return errEnough
		}
		rel, _ := filepath.Rel(folder, path)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || skippedDirs[d.Name()] ||
				strings.Count(rel, string(filepath.Separator)) >= maxCollectionDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && isRootNonStandalone(path) {
			info.Documents++
			dirs[filepath.Dir(path)] = true
			if len(dirs) >= 2 {
				info.Collection = true
			}
		}
		return nil
	})
	return info
}

// isRootNonStandalone: a document (not a standalone TikZ figure).
func isRootNonStandalone(path string) bool {
	if !IsTeXFile(path) {
		return false
	}
	class, ok := documentClass(path)
	return ok && class != "standalone"
}
