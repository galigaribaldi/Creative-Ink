package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	return CopyTree(context.Background(), src, dst, CopyOptions{
		Title:       title,
		HiddenFiles: templateHiddenFiles,
		MaxFiles:    maxTemplateFiles,
		MaxBytes:    maxTemplateBytes,
	})
}
