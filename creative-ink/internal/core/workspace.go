package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Limits keep "Abrir carpeta" on a huge directory (e.g. $HOME) responsive.
const (
	maxTreeDepth   = 6
	maxTreeEntries = 2000
)

var (
	// ErrNoFolder is returned when an operation needs an open folder.
	ErrNoFolder = errors.New("no hay ninguna carpeta abierta")
	// ErrOutsideFolder is returned for paths that escape the open folder.
	ErrOutsideFolder = errors.New("la ruta está fuera de la carpeta abierta")
	// ErrNotTeX is returned when a non-.tex file is opened or saved.
	ErrNotTeX = errors.New("solo se pueden editar archivos .tex")
)

// FileNode is one entry of the folder tree sent to the frontend.
type FileNode struct {
	Name string `json:"name"`
	// Path is absolute: it identifies the file across the IPC bridge and is
	// always validated with ResolveInFolder before touching the disk.
	Path     string     `json:"path"`
	IsDir    bool       `json:"isDir"`
	Editable bool       `json:"editable"` // can be opened in the editor (.tex)
	Children []FileNode `json:"children,omitempty"`
}

// skippedDirs are never listed (besides hidden entries).
var skippedDirs = map[string]bool{"node_modules": true, "__pycache__": true}

// buildArtifacts are LaTeX/latexmk by-products, hidden from the explorer.
var buildArtifacts = map[string]bool{
	".aux": true, ".log": true, ".fls": true, ".fdb_latexmk": true, ".xdv": true,
	".out": true, ".toc": true, ".lof": true, ".lot": true, ".bbl": true, ".blg": true,
	".bcf": true, ".nav": true, ".snm": true, ".tmp": true,
}

// IsTeXFile reports whether name is a LaTeX source file.
func IsTeXFile(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".tex")
}

func isBuildArtifact(name string) bool {
	if strings.HasSuffix(name, ".synctex.gz") || strings.HasSuffix(name, ".run.xml") {
		return true
	}
	return buildArtifacts[strings.ToLower(filepath.Ext(name))]
}

// ScanTree lists root recursively: directories first, then files, both in
// case-insensitive order. Hidden entries, symlinks and build artifacts are
// skipped. truncated reports that the depth or entry limit was hit.
func ScanTree(root string) (tree FileNode, truncated bool, err error) {
	info, err := os.Stat(root)
	if err != nil {
		return FileNode{}, false, fmt.Errorf("read folder %q: %w", root, err)
	}
	if !info.IsDir() {
		return FileNode{}, false, fmt.Errorf("%q no es una carpeta", root)
	}

	s := &treeScanner{}
	tree = FileNode{Name: filepath.Base(root), Path: root, IsDir: true}
	tree.Children = s.children(root, 1)
	return tree, s.truncated, nil
}

type treeScanner struct {
	count     int
	truncated bool
}

func (s *treeScanner) children(dir string, depth int) []FileNode {
	if depth > maxTreeDepth {
		s.truncated = true
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // unreadable subfolder: show it empty rather than fail
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})

	var nodes []FileNode
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "."), e.Type()&fs.ModeSymlink != 0:
			continue
		case e.IsDir() && skippedDirs[name], !e.IsDir() && isBuildArtifact(name):
			continue
		}
		if s.count >= maxTreeEntries {
			s.truncated = true
			break
		}
		s.count++

		path := filepath.Join(dir, name)
		node := FileNode{Name: name, Path: path, IsDir: e.IsDir(), Editable: !e.IsDir() && IsTeXFile(name)}
		if e.IsDir() {
			node.Children = s.children(path, depth+1)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// PrepareRepository creates dir (and missing parents) with 0755 permissions
// and returns the document to open there. If dir has no .tex at its top
// level, it writes starter as main.tex first. It never overwrites a file.
//
// shared opens the repository folder itself to every user of the machine
// (0777 + sticky bit, like /Users/Shared: anyone can add documents, only
// their owner can delete them). On Windows, Public Documents' ACLs apply.
func PrepareRepository(dir, starter string, shared bool) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create repository %q: %w", dir, err)
	}
	if shared && runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
			return "", fmt.Errorf("share repository %q: %w", dir, err)
		}
	}
	if doc := DefaultTeXFile(dir); doc != "" {
		return doc, nil
	}

	doc := filepath.Join(dir, "main.tex")
	// O_EXCL: if another call created it in the meantime, keep that file.
	f, err := os.OpenFile(doc, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return doc, nil
	}
	if err != nil {
		return "", fmt.Errorf("create %q: %w", doc, err)
	}
	_, writeErr := f.WriteString(starter)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return "", fmt.Errorf("write %q: %w", doc, writeErr)
	}
	return doc, nil
}

// DefaultTeXFile picks the document to open in a folder: main.tex if it
// exists, otherwise the first .tex at the top level, otherwise "".
func DefaultTeXFile(folder string) string {
	if main := filepath.Join(folder, "main.tex"); isRegularFile(main) {
		return main
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return ""
	}
	for _, e := range entries { // os.ReadDir returns entries sorted by name
		if !e.IsDir() && IsTeXFile(e.Name()) && !strings.HasPrefix(e.Name(), ".") {
			return filepath.Join(folder, e.Name())
		}
	}
	return ""
}

// ResolveInFolder validates that path (absolute, possibly not existing yet)
// lies inside folder, following symlinks so a link cannot escape it.
// It returns the cleaned path, which is the identity used everywhere else.
func ResolveInFolder(folder, path string) (string, error) {
	if folder == "" {
		return "", ErrNoFolder
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("ruta inválida %q: debe ser absoluta", path)
	}
	clean := filepath.Clean(path)

	realFolder, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return "", fmt.Errorf("resolve folder %q: %w", folder, err)
	}
	// The file may not exist yet (Save As): resolve its directory instead.
	realDir, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", filepath.Dir(clean), err)
	}
	real := filepath.Join(realDir, filepath.Base(clean))
	if resolved, err := filepath.EvalSymlinks(real); err == nil {
		real = resolved // an existing symlinked file must also stay inside
	}

	rel, err := filepath.Rel(realFolder, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideFolder
	}
	return clean, nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
