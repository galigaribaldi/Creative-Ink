package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Importing a project (docs/MAP.md §4.7, phase 2): InspectImport analyses a
// folder without changing anything; the App copies it with CopyTree into a
// staging folder and FinishImport turns that into a project of the
// repository. The original is never modified.

// LargeImportBytes: above this, the summary warns that the copy may take a
// long time (decided by the user on 2026-11-10). It is not a limit.
const LargeImportBytes = int64(2) << 30

// maxImportEntries bounds the analysis of a folder (a disk, not a project).
const maxImportEntries = 200_000

// Lists in the summary are capped; the counts are always exact.
const maxListed = 50

// ErrImportInsideRepository / ErrImportContainsRepository reject folders
// that cannot be imported.
var (
	ErrImportInsideRepository   = errors.New("esa carpeta ya está dentro de My-Creative-Ink-Repository")
	ErrImportContainsRepository = errors.New("esa carpeta contiene My-Creative-Ink-Repository: se copiaría a sí mismo")
)

// ImportPreview is what the import summary shows. Bytes and Files are what
// will be copied without the history; GitBytes/GitFiles are the .git on top.
type ImportPreview struct {
	Canceled bool   `json:"canceled"` // the user closed the folder dialog
	ID       string `json:"id"`
	Source   string `json:"source"` // the folder (for a file: its folder)
	// File is set for a single file (dropped from the system): it is copied
	// into a new quick document. InPlaceSingle: its folder is not a project
	// (Desktop, Downloads), so "open in place" opens only this file.
	File          string `json:"file"`
	InPlaceSingle bool   `json:"inPlaceSingle"`
	ProposedName  string `json:"proposedName"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
	// HasGit: Source itself holds a .git directory (its history can be
	// copied). GitRoot is the nearest repository, Source or a parent.
	HasGit   bool   `json:"hasGit"`
	GitRoot  string `json:"gitRoot"`
	GitFiles int    `json:"gitFiles"`
	GitBytes int64  `json:"gitBytes"`
	// Documents are the root documents found (\documentclass, not
	// standalone), relative with "/". Collection: none at the top level and
	// several in different subfolders, so the folder is a collection of
	// documents, opened in place and never copied as a whole.
	Documents      []string `json:"documents"`
	DocumentCount  int      `json:"documentCount"`
	Collection     bool     `json:"collection"`
	MainCandidates []string `json:"mainCandidates"`
	// ExternalRefs: \input, \includegraphics… that point outside Source;
	// they are not copied.
	ExternalRefs     []ExternalRef `json:"externalRefs"`
	ExternalRefCount int           `json:"externalRefCount"`
	NotUTF8          []string      `json:"notUtf8"`  // text files the editor will not open
	Symlinks         []string      `json:"symlinks"` // never copied
	RCFiles          []string      `json:"rcFiles"`  // .latexmkrc, latexmkrc: copied without trust
	FreeBytes        uint64        `json:"freeBytes"`
	Large            bool          `json:"large"`
	// CanCopy is false for collections and when the disk is too full;
	// CopyBlocked says why. Opening in place is always possible.
	CanCopy     bool             `json:"canCopy"`
	CopyBlocked string           `json:"copyBlocked"`
	Notices     []LocationNotice `json:"notices"`
}

// ExternalRef is a reference to a file outside the imported folder.
type ExternalRef struct {
	File   string `json:"file"` // .tex that has it, relative with "/"
	Line   int    `json:"line"`
	Ref    string `json:"ref"`
	Exists bool   `json:"exists"`
}

// InspectImport analyses src for importing (read only). repoRoot is the
// central repository; checks that need the App (broad folders, free disk
// space, the ID) are done there.
func InspectImport(ctx context.Context, repoRoot, src string) (ImportPreview, error) {
	src = filepath.Clean(src)
	switch {
	case repoRoot != "" && isInside(repoRoot, src):
		return ImportPreview{}, ErrImportInsideRepository
	case repoRoot != "" && isInside(src, repoRoot):
		return ImportPreview{}, ErrImportContainsRepository
	}
	p := ImportPreview{
		Source:         src,
		ProposedName:   Slugify(filepath.Base(src)),
		Documents:      []string{},
		MainCandidates: []string{},
		ExternalRefs:   []ExternalRef{},
		NotUTF8:        []string{},
		Symlinks:       []string{},
		RCFiles:        []string{},
		Notices:        []LocationNotice{},
	}
	home, _ := os.UserHomeDir()
	p.GitRoot = FindGitRoot(src, home)
	if info, err := os.Lstat(filepath.Join(src, ".git")); err == nil && info.IsDir() {
		p.HasGit = true
	}

	var texFiles []string
	entries := 0
	err := walkCopyable(ctx, src, CopyOptions{HiddenFiles: importHiddenFiles, IncludeGit: true}, func(e copyEntry) error {
		if entries++; entries > maxImportEntries {
			return fmt.Errorf("la carpeta tiene más de %d entradas: elige la carpeta del proyecto", maxImportEntries)
		}
		switch {
		case e.dir:
		case e.git:
			p.GitFiles++
			p.GitBytes += e.size
		default:
			p.Files++
			p.Bytes += e.size
			name := filepath.Base(e.rel)
			if IsTeXFile(name) {
				texFiles = append(texFiles, e.rel)
			}
			if IsEditableFile(name) && e.size <= MaxEditableBytes && !isUTF8File(filepath.Join(src, e.rel)) {
				appendCapped(&p.NotUTF8, filepath.ToSlash(e.rel))
			}
		}
		return nil
	})
	if err != nil {
		return ImportPreview{}, err
	}
	p.Symlinks = listSymlinks(src)

	findDocuments(src, texFiles, &p)
	compileDir := src
	if len(p.MainCandidates) > 0 {
		compileDir = filepath.Join(src, filepath.FromSlash(filepath.Dir(p.MainCandidates[0])))
	}
	if _, files, _ := RCHash(compileDir); len(files) > 0 {
		p.RCFiles = files
	}
	if !p.Collection {
		findExternalRefs(src, compileDir, texFiles, &p, func(target string) bool { return isInside(src, target) })
	}
	p.Large = p.Bytes+p.GitBytes > LargeImportBytes
	p.CanCopy = !p.Collection
	if p.Collection {
		p.CopyBlocked = "es una colección de documentos: ábrela en su ubicación o importa la carpeta de uno de ellos"
	}
	return p, nil
}

// classRe finds \documentclass outside comments; group 2 is the class.
var classRe = regexp.MustCompile(`^[^%]*\\documentclass\s*(\[[^\]]*\])?\s*\{([^}]*)\}`)

// findDocuments looks for root documents (a \documentclass other than
// standalone, in the first 64 KB) and decides the main candidates and
// whether src is a collection.
func findDocuments(src string, texFiles []string, p *ImportPreview) {
	var top []string
	dirs := map[string]bool{}
	for _, rel := range texFiles {
		class, ok := documentClass(filepath.Join(src, rel))
		if !ok || class == "standalone" {
			continue
		}
		slash := filepath.ToSlash(rel)
		p.DocumentCount++
		appendCapped(&p.Documents, slash)
		dir := filepath.Dir(slash)
		dirs[dir] = true
		if dir == "." {
			top = append(top, slash)
		}
	}
	slices.Sort(p.Documents)
	// The folder's own main file (an earlier Creative-ink project) first,
	// then main.tex, then the file named like the folder, then the rest.
	meta, _ := LoadProjectMeta(src)
	base := strings.ToLower(filepath.Base(src)) + ".tex"
	rank := func(rel string) int {
		switch {
		case meta.MainFile != "" && rel == meta.MainFile:
			return 0
		case strings.EqualFold(rel, "main.tex"):
			return 1
		case strings.ToLower(rel) == base:
			return 2
		}
		return 3
	}
	candidates := top
	switch {
	case len(top) == 0 && len(dirs) == 1:
		candidates = p.Documents // one document in a subfolder (src/tex/main.tex)
	case len(top) == 0 && len(dirs) > 1:
		p.Collection = true
		candidates = nil
	}
	slices.SortStableFunc(candidates, func(a, b string) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	p.MainCandidates = append([]string{}, candidates...)
}

// documentClass returns the class of a .tex that starts a document.
func documentClass(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, 64<<10))
	sc.Buffer(make([]byte, 0, 64<<10), 64<<10)
	for sc.Scan() {
		if m := classRe.FindStringSubmatch(sc.Text()); m != nil {
			return strings.TrimSpace(m[2]), true
		}
	}
	return "", false
}

// refRe finds file references; group 2 is the argument.
var refRe = regexp.MustCompile(`\\(input|include|includegraphics|includepdf|includesvg|includeonly|bibliography|addbibresource|graphicspath|subfile|lstinputlisting|verbatiminput)\*?\s*(?:\[[^\]]*\])?\s*\{((?:[^{}]|\{[^{}]*\})*)\}`)

// findExternalRefs lists references that resolve outside src. TeX resolves
// relative paths against the folder it runs in (the main file's), not the
// file that has them.
func findExternalRefs(src, compileDir string, texFiles []string, p *ImportPreview, copied func(target string) bool) {
	for _, rel := range texFiles {
		f, err := os.Open(filepath.Join(src, rel))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(io.LimitReader(f, MaxEditableBytes))
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for line := 1; sc.Scan(); line++ {
			text := stripTeXComment(sc.Text())
			for _, m := range refRe.FindAllStringSubmatch(text, -1) {
				for _, ref := range splitRefs(m[2]) {
					target := ref
					if !filepath.IsAbs(target) {
						target = filepath.Join(compileDir, filepath.FromSlash(ref))
					}
					if copied(target) {
						continue
					}
					p.ExternalRefCount++
					if len(p.ExternalRefs) < maxListed {
						p.ExternalRefs = append(p.ExternalRefs, ExternalRef{
							File: filepath.ToSlash(rel), Line: line, Ref: ref, Exists: refExists(target),
						})
					}
				}
			}
		}
		f.Close()
	}
}

// splitRefs turns an argument into paths: "a,b" (\bibliography),
// "{img/}{fig/}" (\graphicspath). Arguments built from macros (#1,
// \ColorTema) cannot be resolved and are skipped.
func splitRefs(arg string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(arg, func(r rune) bool { return r == ',' || r == '{' || r == '}' }) {
		part = strings.TrimSpace(part)
		if part == "" || strings.ContainsAny(part, `#\`) {
			continue
		}
		if strings.HasPrefix(part, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				part = filepath.Join(home, strings.TrimPrefix(part, "~"))
			}
		}
		out = append(out, part)
	}
	return out
}

// refExists also tries the extensions TeX adds (.tex, .bib, images).
func refExists(target string) bool {
	for _, ext := range []string{"", ".tex", ".bib", ".pdf", ".png", ".jpg", ".jpeg", ".eps", ".svg"} {
		if _, err := os.Stat(target + ext); err == nil {
			return true
		}
	}
	return false
}

func isUTF8File(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && utf8.Valid(data)
}

// listSymlinks lists the symlinks under src (not followed, not copied).
func listSymlinks(src string) []string {
	out := []string{}
	_ = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || skippedDirs[d.Name()]) {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(src, path)
			appendCapped(&out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func appendCapped(list *[]string, s string) {
	if len(*list) < maxListed {
		*list = append(*list, s)
	}
}

// InspectFileImport analyses a single text file (dropped from the system)
// for "copy into a quick document" or "open in place". Every file it
// references is outside the copy (only the file is copied).
func InspectFileImport(repoRoot, file string) (ImportPreview, error) {
	file = filepath.Clean(file)
	info, err := os.Stat(file)
	switch {
	case err != nil:
		return ImportPreview{}, err
	case !info.Mode().IsRegular() || !IsEditableFile(file):
		return ImportPreview{}, ErrNotEditable
	case info.Size() > MaxEditableBytes:
		return ImportPreview{}, fmt.Errorf("%s ocupa más de %d MB: el editor no lo abre", filepath.Base(file), MaxEditableBytes>>20)
	case !isUTF8File(file):
		return ImportPreview{}, ErrNotUTF8
	}
	parent := filepath.Dir(file)
	name := filepath.Base(file)
	p := ImportPreview{
		Source:         parent,
		File:           file,
		ProposedName:   Slugify(strings.TrimSuffix(name, filepath.Ext(name))),
		Files:          1,
		Bytes:          info.Size(),
		Documents:      []string{},
		MainCandidates: []string{},
		ExternalRefs:   []ExternalRef{},
		NotUTF8:        []string{},
		Symlinks:       []string{},
		RCFiles:        []string{},
		Notices:        []LocationNotice{},
		CanCopy:        true,
	}
	home, _ := os.UserHomeDir()
	p.GitRoot = FindGitRoot(parent, home)
	p.InPlaceSingle = CheckProjectFolder(repoRoot, parent) != nil
	if IsTeXFile(name) {
		p.MainCandidates = []string{name}
		if IsRootDocument(file) {
			p.Documents, p.DocumentCount = []string{name}, 1
		}
		findExternalRefs(parent, parent, []string{name}, &p, func(target string) bool { return target == file })
	}
	if _, files, _ := RCHash(parent); len(files) > 0 {
		p.RCFiles = files
	}
	return p, nil
}

// CopyFileToQuickDoc copies file into a new quick document folder and
// records where it came from; with gitRoot, COPIA-DE-GIT.md goes with it.
// It returns the folder and the copied file. On error nothing is left.
func CopyFileToQuickDoc(root string, p ImportPreview) (dir, copied string, err error) {
	if dir, err = newQuickDocDir(root); err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir) // created above: nothing else lives there
			dir, copied = "", ""
		}
	}()
	info, err := os.Stat(p.File)
	if err != nil {
		return
	}
	copied = filepath.Join(dir, filepath.Base(p.File))
	if err = copyOneFile(p.File, copied, copyEntry{size: info.Size(), mode: info.Mode().Perm()}, ""); err != nil {
		return
	}
	meta := ProjectMeta{Imported: &ImportInfo{From: p.File, GitRoot: p.GitRoot}}
	if IsTeXFile(copied) {
		meta.MainFile = filepath.Base(copied)
	}
	if err = SaveProjectMeta(dir, meta); err != nil {
		return
	}
	if p.GitRoot != "" {
		err = writeNew(filepath.Join(dir, GitCopyNoteName), GitCopyNoteFile(*meta.Imported))
	}
	return
}

// ImportStagingDir is where copies are made before becoming a project
// (inside the repository's hidden folder: same disk, so the final rename
// is atomic, and never listed as a project).
func ImportStagingDir(repoRoot, id string) string {
	return filepath.Join(repoRoot, MetaDirName, "importing", id)
}

// ImportedProjectDir validates the name chosen for an import and returns
// the project folder it will have (it must not exist yet).
func ImportedProjectDir(repoRoot, name string) (string, error) {
	slug := Slugify(name)
	switch {
	case slug == "":
		return "", errors.New("el nombre del proyecto no tiene caracteres válidos")
	case strings.EqualFold(slug, QuickDocsDirName) || strings.HasPrefix(slug, "."):
		return "", fmt.Errorf("%q es un nombre reservado", slug)
	}
	dir := filepath.Join(repoRoot, slug)
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("%w: %s", ErrProjectExists, slug)
	}
	return dir, nil
}

// FinishImport turns a finished copy in staging into the project dir:
// records the main file and where it came from in .creative-ink-meta,
// writes COPIA-DE-GIT.md when the original is under git (the warning
// travels with the folder), and renames staging to dir. On error, staging
// is left for the caller to remove.
func FinishImport(staging, dir, mainRel string, p ImportPreview, withHistory bool) error {
	meta, _ := LoadProjectMeta(staging)
	if mainRel != "" {
		meta.MainFile = mainRel
	}
	meta.Imported = &ImportInfo{From: p.Source, GitRoot: p.GitRoot, WithHistory: withHistory && p.HasGit}
	if err := SaveProjectMeta(staging, meta); err != nil {
		return err
	}
	if p.GitRoot != "" {
		note := filepath.Join(staging, GitCopyNoteName)
		if _, err := os.Lstat(note); errors.Is(err, os.ErrNotExist) {
			if err := writeNew(note, GitCopyNoteFile(*meta.Imported)); err != nil {
				return err
			}
		}
	}
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("%w: %s", ErrProjectExists, filepath.Base(dir))
	}
	return os.Rename(staging, dir)
}
