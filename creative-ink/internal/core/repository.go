package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Repository layout (docs/MAP.md §1):
//
//	My-Creative-Ink-Repository/          ← launcher, never opened as a project
//	├── .creative-ink/                   ← hidden: editor data for this repository
//	│   ├── config.json                  ← repository defaults (config service, step 2)
//	│   ├── plugins/                     ← per-user plugins (personal repositories only)
//	│   ├── templates/                   ← "New project" templates
//	│   └── guide/                       ← auto-generated `ci` guide (step 4)
//	├── tmp-Workspaces/                  ← quick single-file documents
//	│   ├── unnamed-tmp1/main.tex
//	│   └── unnamed-tmp2/main.tex
//	└── <Project-Name>/                  ← one project = one folder
const (
	MetaDirName      = ".creative-ink"
	QuickDocsDirName = "tmp-Workspaces"
	quickDocPrefix   = "unnamed-tmp"
	// QuickDocMaxAge: quick documents untouched for longer are listed when a
	// session starts. They are never deleted automatically.
	QuickDocMaxAge = 8 * time.Hour
)

var (
	// ErrProjectExists is returned when a project folder already exists.
	ErrProjectExists = errors.New("ya existe un proyecto con ese nombre")
	// ErrFileExists is returned when a new file would replace an existing one.
	ErrFileExists = errors.New("ya existe un archivo con ese nombre (Creative-ink nunca lo reemplaza al crear uno nuevo)")
)

// ProjectInfo describes a folder of the repository on the start screen.
type ProjectInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"` // absolute
	MainFile string    `json:"mainFile"`
	Modified time.Time `json:"modified"` // newest top-level file
}

// MetaDir returns <root>/.creative-ink.
func MetaDir(root string) string { return filepath.Join(root, MetaDirName) }

// QuickDocsDir returns <root>/tmp-Workspaces.
func QuickDocsDir(root string) string { return filepath.Join(root, QuickDocsDirName) }

// EnsureRepositoryLayout creates the repository and its fixed folders when
// missing (0755; the root is 0777 + sticky when shared, like /Users/Shared).
// It never writes documents at the root: the root is a launcher.
//
// Plugins are per-user: a shared repository gets no plugins/ folder (any
// account could plant code there that another account would run).
func EnsureRepositoryLayout(root string, shared bool) error {
	dirs := []string{
		root,
		MetaDir(root),
		filepath.Join(MetaDir(root), "templates"),
		filepath.Join(MetaDir(root), "guide"),
		QuickDocsDir(root),
	}
	if !shared {
		dirs = append(dirs, filepath.Join(MetaDir(root), "plugins"))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %q: %w", dir, err)
		}
	}
	if shared && runtime.GOOS != "windows" {
		for _, dir := range []string{root, QuickDocsDir(root)} {
			if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
				return fmt.Errorf("share %q: %w", dir, err)
			}
		}
	}
	if err := hidePath(MetaDir(root)); err != nil {
		return fmt.Errorf("hide %q: %w", MetaDir(root), err)
	}
	config := filepath.Join(MetaDir(root), "config.json")
	if err := writeIfMissing(config, "{\n  \"version\": 1\n}\n"); err != nil {
		return err
	}
	return nil
}

// ── Names ──────────────────────────────────────────────────────────────────

var (
	accentFold = strings.NewReplacer(
		"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i", "ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u", "ñ", "n", "ç", "c",
		"Á", "A", "À", "A", "Ä", "A", "Â", "A", "Ã", "A", "É", "E", "È", "E", "Ë", "E", "Ê", "E",
		"Í", "I", "Ì", "I", "Ï", "I", "Î", "I", "Ó", "O", "Ò", "O", "Ö", "O", "Ô", "O", "Õ", "O",
		"Ú", "U", "Ù", "U", "Ü", "U", "Û", "U", "Ñ", "N", "Ç", "C",
	)
	slugInvalid = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	slugDashes  = regexp.MustCompile(`-{2,}`)
)

const maxSlugLength = 64

// Slugify normalizes a project name to the editor's convention: spaces and
// invalid characters become "-", accents are folded ("Tesis de Maestría" →
// "Tesis-de-Maestria"). It returns "" when nothing usable is left.
func Slugify(name string) string {
	s := accentFold.Replace(strings.TrimSpace(name))
	s = slugInvalid.ReplaceAllString(s, "-")
	s = slugDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-.")
	if len(s) > maxSlugLength {
		s = strings.Trim(s[:maxSlugLength], "-.")
	}
	return s
}

// ── Projects ───────────────────────────────────────────────────────────────

// CreateProject creates <root>/<Slugify(name)>/ from the built-in template
// (main.tex, capitulos/, figuras/, referencias.bib) with main.tex set as the
// main file. It never overwrites an existing folder.
func CreateProject(root, name string) (dir, mainTex string, err error) {
	if dir, err = newProjectDir(root, name); err != nil {
		return "", "", err
	}

	title := strings.TrimSpace(name)
	files := map[string]string{
		"main.tex":                        projectMainTemplate(EscapeTeX(title)),
		"capitulos/introduccion.tex":      projectChapterTemplate,
		"referencias.bib":                 projectBibTemplate,
		filepath.Join("figuras", ".keep"): "",
	}
	for rel, content := range files {
		if err := writeNew(filepath.Join(dir, rel), content); err != nil {
			return "", "", err
		}
	}
	if err := SaveProjectMeta(dir, ProjectMeta{MainFile: "main.tex"}); err != nil {
		return "", "", err
	}
	return dir, filepath.Join(dir, "main.tex"), nil
}

// newProjectDir validates a project name and creates its empty folder
// <root>/<Slugify(name)>/. It never reuses an existing folder.
func newProjectDir(root, name string) (string, error) {
	slug := Slugify(name)
	switch {
	case slug == "":
		return "", errors.New("el nombre del proyecto no tiene caracteres válidos")
	case strings.EqualFold(slug, QuickDocsDirName) || strings.HasPrefix(slug, "."):
		return "", fmt.Errorf("%q es un nombre reservado", slug)
	}
	dir := filepath.Join(root, slug)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("%w: %s", ErrProjectExists, slug)
		}
		return "", fmt.Errorf("create project %q: %w", dir, err)
	}
	return dir, nil
}

// ListProjects returns the project folders of the repository (every visible
// subfolder except tmp-Workspaces), newest first.
func ListProjects(root string) ([]ProjectInfo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read repository %q: %w", root, err)
	}
	projects := []ProjectInfo{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == QuickDocsDirName {
			continue
		}
		dir := filepath.Join(root, e.Name())
		projects = append(projects, describe(dir))
	}
	sortNewestFirst(projects)
	return projects, nil
}

// ── Quick documents ────────────────────────────────────────────────────────

// CreateQuickDoc creates tmp-Workspaces/unnamed-tmp<N>/main.tex with the
// next free N, so many quick documents never mix their auxiliary files.
func CreateQuickDoc(root, starter string) (dir, tex string, err error) {
	if dir, err = newQuickDocDir(root); err != nil {
		return "", "", err
	}
	tex = filepath.Join(dir, "main.tex")
	if err := writeNew(tex, starter); err != nil {
		return "", "", err
	}
	return dir, tex, nil
}

// newQuickDocDir creates the next free tmp-Workspaces/unnamed-tmp<N>/.
// Mkdir fails if another window created it meanwhile: then the next number.
func newQuickDocDir(root string) (string, error) {
	parent := QuickDocsDir(root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create %q: %w", parent, err)
	}
	for n := nextQuickDocNumber(parent); ; n++ {
		dir := filepath.Join(parent, quickDocPrefix+strconv.Itoa(n))
		if err := os.Mkdir(dir, 0o755); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return "", fmt.Errorf("create %q: %w", dir, err)
		}
		return dir, nil
	}
}

// ReusableQuickDoc returns the newest quick document that is still
// untouched (only its main.tex, with exactly the starter text: never
// edited nor compiled), so closing the start screen again and again does
// not pile up empty documents in tmp-Workspaces (the editor never deletes
// them). "" if there is none.
func ReusableQuickDoc(root, starter string) (dir, tex string) {
	docs, err := ListQuickDocs(root)
	if err != nil {
		return "", ""
	}
	for _, d := range docs {
		entries, err := os.ReadDir(d.Path)
		if err != nil || len(entries) != 1 || entries[0].Name() != "main.tex" {
			continue
		}
		tex := filepath.Join(d.Path, "main.tex")
		if data, err := os.ReadFile(tex); err == nil && string(data) == starter {
			return d.Path, tex
		}
	}
	return "", ""
}

// ListQuickDocs returns the quick documents, newest first.
func ListQuickDocs(root string) ([]ProjectInfo, error) {
	entries, err := os.ReadDir(QuickDocsDir(root))
	if errors.Is(err, fs.ErrNotExist) {
		return []ProjectInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	docs := []ProjectInfo{}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			docs = append(docs, describe(filepath.Join(QuickDocsDir(root), e.Name())))
		}
	}
	sortNewestFirst(docs)
	return docs, nil
}

// StaleQuickDocs returns the quick documents not modified for longer than
// maxAge. The editor only reports them; deleting is the user's decision.
func StaleQuickDocs(root string, maxAge time.Duration, now time.Time) ([]ProjectInfo, error) {
	docs, err := ListQuickDocs(root)
	if err != nil {
		return nil, err
	}
	stale := []ProjectInfo{}
	for _, d := range docs {
		if !d.Modified.IsZero() && now.Sub(d.Modified) > maxAge {
			stale = append(stale, d)
		}
	}
	return stale, nil
}

func nextQuickDocNumber(parent string) int {
	entries, _ := os.ReadDir(parent)
	max := 0
	for _, e := range entries {
		if n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), quickDocPrefix)); err == nil &&
			strings.HasPrefix(e.Name(), quickDocPrefix) && n > max {
			max = n
		}
	}
	return max + 1
}

// ── Helpers ────────────────────────────────────────────────────────────────

// describe summarizes a folder: its main file (meta, else main.tex / first
// .tex) and the newest modification time of its top-level files.
func describe(dir string) ProjectInfo {
	info := ProjectInfo{Name: filepath.Base(dir), Path: dir}
	if meta, err := LoadProjectMeta(dir); err == nil {
		info.MainFile = meta.MainFilePath(dir)
	}
	if info.MainFile == "" {
		info.MainFile = DefaultTeXFile(dir)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() && fi.ModTime().After(info.Modified) {
			info.Modified = fi.ModTime()
		}
	}
	return info
}

func sortNewestFirst(list []ProjectInfo) {
	slices.SortStableFunc(list, func(a, b ProjectInfo) int { return b.Modified.Compare(a.Modified) })
}

// EscapeTeX escapes the characters that are special in LaTeX text, so a
// project name can go into \title{…} safely.
func EscapeTeX(s string) string {
	return strings.NewReplacer(
		`\`, `\textbackslash{}`, "{", `\{`, "}", `\}`, "$", `\$`, "&", `\&`,
		"#", `\#`, "%", `\%`, "_", `\_`, "~", `\textasciitilde{}`, "^", `\textasciicircum{}`,
	).Replace(s)
}

// CreateEmptyFile creates an empty file (and its folders). It fails with
// ErrFileExists instead of replacing an existing file.
func CreateEmptyFile(path string) error {
	if err := writeNew(path, ""); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrFileExists, filepath.Base(path))
		}
		return err
	}
	return nil
}

// writeNew creates path (and its folders) only if it does not exist.
func writeNew(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	_, werr := f.WriteString(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

func writeIfMissing(path, content string) error {
	if err := writeNew(path, content); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

func projectMainTemplate(title string) string {
	return `\documentclass[12pt]{article}
\usepackage{fontspec}            % XeLaTeX: fuentes del sistema y UTF-8
\usepackage[spanish]{babel}
\usepackage{graphicx}
\graphicspath{{figuras/}}

\title{` + title + `}
\author{}
\date{\today}

\begin{document}
\maketitle

\input{capitulos/introduccion}

% Bibliografía: descomenta estas líneas cuando cites algo con \cite{…}
% \bibliographystyle{apalike}
% \bibliography{referencias}

\end{document}
`
}

const projectChapterTemplate = `\section{Introducción}

Escribe aquí tu primer capítulo.
`

const projectBibTemplate = `% Referencias del proyecto (BibTeX).
% Ejemplo:
% @book{clave,
%   author    = {Apellido, Nombre},
%   title     = {Título},
%   publisher = {Editorial},
%   year      = {2026}
% }
`
