package core

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// CopyOptions controls CopyTree. The same filter decides what InspectImport
// counts and what is copied, so the size shown before copying is the size
// that gets copied.
type CopyOptions struct {
	// Title, when not empty, replaces {{TITULO}}/{{TITLE}} in UTF-8 text
	// files (project templates). Imports leave every file as it is.
	Title string
	// HiddenFiles are hidden files that are copied (.latexmkrc…); every
	// other hidden entry is skipped, except .git with IncludeGit.
	HiddenFiles map[string]bool
	// IncludeGit copies .git directories (the history). A .git *file*
	// (worktree or submodule) is never copied: it points to the original
	// repository, and git commands in the copy would act on it.
	IncludeGit bool
	// MaxFiles / MaxBytes bound the copy (0 = no limit).
	MaxFiles int
	MaxBytes int64
	// Progress, if set, is called after every copied file.
	Progress func(CopyProgress)
}

// CopyProgress is how far a copy got.
type CopyProgress struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// importHiddenFiles are the hidden files an imported project keeps: build
// and editor configuration that belongs to the project (.DS_Store, .vscode,
// .idea… are skipped).
var importHiddenFiles = map[string]bool{
	MetaFileName: true, ".latexmkrc": true, ".gitignore": true, ".gitattributes": true,
	".gitmodules": true, ".editorconfig": true, ".latexindent.yaml": true, ".chktexrc": true,
}

// copyEntry is one file CopyTree will copy.
type copyEntry struct {
	rel  string // relative to the source, OS separators
	size int64
	mode fs.FileMode
	git  bool // inside a .git directory: copied raw
	dir  bool // a folder (created even if it stays empty)
}

// walkCopyable lists what CopyTree copies from src, calling fn for every
// folder and file. Symlinks are never followed (they could point outside), and LaTeX
// build artifacts and skipped folders (node_modules…) are left out, except
// inside .git, which is copied raw.
func walkCopyable(ctx context.Context, src string, opts CopyOptions, fn func(copyEntry) error) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		inGit := isUnderGitDir(rel)
		name := d.Name()
		hidden := strings.HasPrefix(name, ".")
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			switch {
			case inGit, name == ".git" && opts.IncludeGit:
			case hidden || skippedDirs[name]:
				return filepath.SkipDir
			}
			return fn(copyEntry{rel: rel, git: inGit || name == ".git", dir: true})
		case !d.Type().IsRegular():
			return nil
		case !inGit && (hidden && !opts.HiddenFiles[name] || isBuildArtifact(name)):
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fn(copyEntry{rel: rel, size: info.Size(), mode: info.Mode().Perm(), git: inGit})
	})
}

// isUnderGitDir reports whether rel lies inside a .git directory.
func isUnderGitDir(rel string) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	for _, p := range parts[:len(parts)-1] {
		if p == ".git" {
			return true
		}
	}
	return false
}

// CopyTree copies src into dst (created if needed) with opts. It never
// replaces a file: dst is expected to be new. Cancelling ctx stops it; the
// caller removes the half-made copy.
func CopyTree(ctx context.Context, src, dst string, opts CopyOptions) error {
	var progress CopyProgress
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return walkCopyable(ctx, src, opts, func(e copyEntry) error {
		if e.dir {
			return os.MkdirAll(filepath.Join(dst, e.rel), 0o755)
		}
		if progress.Files++; opts.MaxFiles > 0 && progress.Files > opts.MaxFiles {
			return fmt.Errorf("la carpeta tiene más de %d archivos", opts.MaxFiles)
		}
		if progress.Bytes += e.size; opts.MaxBytes > 0 && progress.Bytes > opts.MaxBytes {
			return fmt.Errorf("la carpeta ocupa más de %d MB", opts.MaxBytes>>20)
		}
		if err := copyOneFile(filepath.Join(src, e.rel), filepath.Join(dst, e.rel), e, opts.Title); err != nil {
			return err
		}
		if opts.Progress != nil {
			opts.Progress(progress)
		}
		return nil
	})
}

// copyOneFile copies a file without replacing one, keeping its permissions
// (scripts, git hooks). With a title, UTF-8 text files get their
// placeholders replaced.
func copyOneFile(src, dst string, e copyEntry, title string) error {
	if title != "" && !e.git && IsEditableFile(src) && e.size <= MaxEditableBytes {
		if data, err := os.ReadFile(src); err == nil && utf8.Valid(data) {
			text := string(data)
			for _, p := range titlePlaceholders {
				text = strings.ReplaceAll(text, p, title)
			}
			return writeNew(dst, text)
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	mode := e.mode | 0o600 // always readable and writable by the user
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}
	_, cerr := io.Copy(out, in)
	if err := out.Close(); cerr == nil {
		cerr = err
	}
	return cerr
}

// ImportHiddenFiles returns the hidden files an import keeps (a copy, so
// callers cannot change the package's set).
func ImportHiddenFiles() map[string]bool {
	out := make(map[string]bool, len(importHiddenFiles))
	for k, v := range importHiddenFiles {
		out[k] = v
	}
	return out
}
