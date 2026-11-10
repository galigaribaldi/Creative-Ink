package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// Project templates (docs/MAP.md §1, `.creative-ink/templates/`): every
// subfolder of a templates folder is a template whose files are copied into
// a new project. Two places are read, the repository's first:
//
//	<repo>/.creative-ink/templates/<Nombre>/     shared with the repository
//	<UserConfigDir>/creative-ink/templates/<Nombre>/   the user's own
//
// In the copied text files, {{TITULO}} / {{TITLE}} become the project name
// (escaped for LaTeX). A template's .creative-ink-meta sets the main file;
// without one, main.tex or the first .tex is used.

// Template sources.
const (
	TemplateSourceRepository = "repository"
	TemplateSourceUser       = "user"
)

// Copy limits: a template is a few sources and figures, not a disk image.
const (
	maxTemplateFiles = 2000
	maxTemplateBytes = 200 << 20
)

// Hidden files a template may carry (other hidden entries, e.g. .git, are skipped).
var templateHiddenFiles = map[string]bool{MetaFileName: true, ".latexmkrc": true}

// titlePlaceholders are replaced by the project's title in text files.
var titlePlaceholders = []string{"{{TITULO}}", "{{TITLE}}"}

// ErrUnknownTemplate is returned for a template that is not listed.
var ErrUnknownTemplate = errors.New("esa plantilla no existe")

// TemplateSource is a folder that holds templates.
type TemplateSource struct {
	Dir    string
	Source string // TemplateSourceRepository | TemplateSourceUser
}

// TemplateInfo is one template on the start screen.
type TemplateInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"` // absolute: the identity sent back to Go
	Source string `json:"source"`
}

// ListTemplates lists the templates of the sources, sorted by name. On a
// name clash the earlier source wins (the repository's over the user's).
// Missing folders are skipped.
func ListTemplates(sources ...TemplateSource) []TemplateInfo {
	seen := map[string]bool{}
	list := []TemplateInfo{}
	for _, src := range sources {
		entries, err := os.ReadDir(src.Dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || strings.HasPrefix(name, ".") || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			list = append(list, TemplateInfo{Name: name, Path: filepath.Join(src.Dir, name), Source: src.Source})
		}
	}
	slices.SortFunc(list, func(a, b TemplateInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return list
}

// CreateProjectFromTemplate creates <root>/<Slugify(name)>/ with a copy of
// templateDir and returns the main file ("" when the template has no .tex).
// Symlinks, hidden entries (except .creative-ink-meta and .latexmkrc) and
// LaTeX build artifacts are not copied. If anything fails, the half-made
// project folder is removed.
func CreateProjectFromTemplate(root, name, templateDir string) (dir, mainTex string, err error) {
	if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%w: %s", ErrUnknownTemplate, filepath.Base(templateDir))
	}
	if dir, err = newProjectDir(root, name); err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir) // created above by us: nothing else lives there
		}
	}()
	if err = copyTemplate(templateDir, dir, EscapeTeX(strings.TrimSpace(name))); err != nil {
		return "", "", err
	}

	meta, metaErr := LoadProjectMeta(dir)
	if metaErr == nil {
		mainTex = meta.MainFilePath(dir)
	}
	if mainTex == "" {
		if mainTex = DefaultTeXFile(dir); mainTex != "" {
			rel, _ := RelativeMainFile(dir, mainTex)
			meta.MainFile = rel
			if err = SaveProjectMeta(dir, meta); err != nil {
				return "", "", err
			}
		}
	}
	return dir, mainTex, nil
}

func copyTemplate(src, dst, title string) error {
	files, total := 0, int64(0)
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		name := d.Name()
		hidden := strings.HasPrefix(name, ".")
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return nil // a link could point outside the template
		case d.IsDir():
			if hidden || skippedDirs[name] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		case !d.Type().IsRegular(), hidden && !templateHiddenFiles[name], isBuildArtifact(name):
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if files++; files > maxTemplateFiles {
			return fmt.Errorf("la plantilla tiene más de %d archivos", maxTemplateFiles)
		}
		if total += info.Size(); total > maxTemplateBytes {
			return fmt.Errorf("la plantilla ocupa más de %d MB", maxTemplateBytes>>20)
		}
		return copyTemplateFile(path, filepath.Join(dst, rel), title)
	})
}

// copyTemplateFile copies one file (never replacing one). UTF-8 text files
// get their title placeholders replaced; anything else is copied as is.
func copyTemplateFile(src, dst, title string) error {
	if IsEditableFile(src) {
		if data, err := os.ReadFile(src); err == nil && len(data) <= MaxEditableBytes && utf8.Valid(data) {
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
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}
	_, cerr := io.Copy(out, in)
	if err := out.Close(); cerr == nil {
		cerr = err
	}
	return cerr
}
