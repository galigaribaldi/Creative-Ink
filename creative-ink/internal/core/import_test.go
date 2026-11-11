package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTree creates files (relative path → content) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

const doc = "% comentario\n\\documentclass[12pt]{article}\n\\begin{document}\n\\input{secciones/intro}\n\\end{document}\n"

// A miniature of Trabajos-Maestria-Urbanismo: a git repository holding
// several documents, one of them using images of a sibling folder.
func masterRepo(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "Trabajos-Maestria")
	writeTree(t, root, map[string]string{
		".git/HEAD":                               "ref: refs/heads/main\n",
		".git/config":                             "[core]\n",
		"README.md":                               "# Trabajos\n",
		"Makefile":                                "all:\n",
		"Semestre/Registro1/main.tex":             doc,
		"Semestre/Registro1/secciones/intro.tex":  "Hola\n",
		"Semestre/Registro1/referencias.bib":      "@book{a,}\n",
		"Semestre/Registro1/main.aux":             "aux\n",
		"Semestre/Registro1/main.synctex.gz":      "x",
		"Semestre/Registro1/Registro1.pdf":        "%PDF",
		"Semestre/Articulo/main.tex":              doc,
		"Semestre/Articulo/img/tikz/fig1.tex":     "\\documentclass{standalone}\n",
		"Semestre/Ensayo/main.tex":                doc,
		"Semestre/Ensayo/img/foto.png":            "png",
		"Semestre/Presentacion/main.tex":          doc + "\\includegraphics[width=3cm]{../Ensayo/img/foto.png}\n% \\input{../comentado}\n\\includegraphics{#2}\n",
		"Semestre/Presentacion/.DS_Store":         "x",
		"Semestre/Presentacion/.latexmkrc":        "$pdf_mode = 1;\n",
		"Semestre/Presentacion/node_modules/x.js": "x",
	})
	return root
}

func TestInspectImportDetectsACollectionAndBlocksTheWholeCopy(t *testing.T) {
	root := masterRepo(t)
	p, err := InspectImport(context.Background(), t.TempDir(), root)
	require.NoError(t, err)
	assert.True(t, p.HasGit)
	assert.Equal(t, root, p.GitRoot)
	assert.True(t, p.Collection, "no document at the top, several in subfolders")
	assert.Equal(t, 4, p.DocumentCount, "standalone TikZ figures are not documents")
	assert.False(t, p.CanCopy)
	assert.NotEmpty(t, p.CopyBlocked)
	ids := noticeIDs(ImportNotices(p))
	assert.Equal(t, "import-collection", ids[0])
	assert.Contains(t, ids, "import-git")
}

func TestInspectImportOfADocumentInsideAGitRepository(t *testing.T) {
	root := masterRepo(t)
	src := filepath.Join(root, "Semestre", "Registro1")
	p, err := InspectImport(context.Background(), t.TempDir(), src)
	require.NoError(t, err)
	assert.False(t, p.HasGit, "the .git is in a parent: no history to copy")
	assert.Equal(t, root, p.GitRoot)
	assert.False(t, p.Collection)
	assert.True(t, p.CanCopy)
	assert.Equal(t, []string{"main.tex"}, p.MainCandidates)
	assert.Equal(t, "Registro1", p.ProposedName)
	// main.tex, intro.tex, referencias.bib, Registro1.pdf: no .aux, no .synctex.gz.
	assert.Equal(t, 4, p.Files)
	assert.Zero(t, p.ExternalRefCount)
	ids := noticeIDs(ImportNotices(p))
	assert.Equal(t, []string{"import-git-part", "import-size"}, ids)
}

func TestInspectImportListsReferencesOutsideTheFolder(t *testing.T) {
	root := masterRepo(t)
	p, err := InspectImport(context.Background(), t.TempDir(), filepath.Join(root, "Semestre", "Presentacion"))
	require.NoError(t, err)
	require.Equal(t, 1, p.ExternalRefCount, "comments and macro arguments (#2) are ignored")
	ref := p.ExternalRefs[0]
	assert.Equal(t, "../Ensayo/img/foto.png", ref.Ref)
	assert.Equal(t, "main.tex", ref.File)
	assert.True(t, ref.Exists)
	assert.Equal(t, []string{".latexmkrc"}, p.RCFiles)
	assert.Contains(t, noticeIDs(ImportNotices(p)), "import-external")
	assert.Contains(t, noticeIDs(ImportNotices(p)), "import-rc")
}

func TestInspectImportRejectsTheRepositoryItself(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "My-Creative-Ink-Repository")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "Tesis"), 0o755))
	_, err := InspectImport(context.Background(), repo, filepath.Join(repo, "Tesis"))
	assert.ErrorIs(t, err, ErrImportInsideRepository)
	_, err = InspectImport(context.Background(), repo, filepath.Dir(repo))
	assert.ErrorIs(t, err, ErrImportContainsRepository)
}

func TestLargeImportsAreFlaggedNotRefused(t *testing.T) {
	p := ImportPreview{Files: 10, Bytes: LargeImportBytes + 1, Large: true, CanCopy: true}
	n := ImportNotices(p)
	require.NotEmpty(t, n)
	assert.Equal(t, NoticeWarning, n[0].Level)
	assert.Contains(t, n[0].Text, "más de 2 GB")
	assert.Contains(t, ImportNotices(ImportPreview{Files: 3, Bytes: 2048})[0].Text, "3 archivos (2 KB)", "the size is always shown")
}

func TestCopyTreeAndFinishImportOfAWholeRepository(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Tesis")
	writeTree(t, src, map[string]string{
		".git/HEAD":             "ref: refs/heads/main\n",
		".git/objects/ab/cdef":  "obj",
		".gitignore":            "*.aux\n",
		"main.tex":              doc + "{{TITULO}}\n",
		"secciones/intro.tex":   "Hola\n",
		"main.aux":              "aux\n",
		".vscode/settings.json": "{}",
	})
	require.NoError(t, os.MkdirAll(filepath.Join(src, "img"), 0o755)) // empty folder
	script := filepath.Join(src, "build.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755))
	p, err := InspectImport(context.Background(), "", src)
	require.NoError(t, err)
	require.True(t, p.HasGit)

	repo := t.TempDir()
	staging := ImportStagingDir(repo, "abc")
	var last CopyProgress
	require.NoError(t, CopyTree(context.Background(), src, staging, CopyOptions{
		HiddenFiles: importHiddenFiles, IncludeGit: true, Progress: func(c CopyProgress) { last = c },
	}))
	assert.Equal(t, p.Files+p.GitFiles, last.Files, "the summary counts exactly what is copied")
	assert.Equal(t, p.Bytes+p.GitBytes, last.Bytes)

	dir, err := ImportedProjectDir(repo, "Tesis de Maestría")
	require.NoError(t, err)
	require.NoError(t, FinishImport(staging, dir, "main.tex", p, true))

	data, err := os.ReadFile(filepath.Join(dir, "main.tex"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "{{TITULO}}", "an import never replaces placeholders")
	assert.FileExists(t, filepath.Join(dir, ".git", "objects", "ab", "cdef"))
	assert.FileExists(t, filepath.Join(dir, ".gitignore"))
	assert.DirExists(t, filepath.Join(dir, "img"))
	assert.NoFileExists(t, filepath.Join(dir, "main.aux"))
	assert.NoDirExists(t, filepath.Join(dir, ".vscode"))
	info, err := os.Stat(filepath.Join(dir, "build.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o100, "permissions are kept")
	note, err := os.ReadFile(filepath.Join(dir, GitCopyNoteName))
	require.NoError(t, err)
	assert.Contains(t, string(note), "git pull")
	assert.NoDirExists(t, staging)

	meta, err := LoadProjectMeta(dir)
	require.NoError(t, err)
	assert.Equal(t, "main.tex", meta.MainFile)
	require.NotNil(t, meta.Imported)
	assert.Equal(t, src, meta.Imported.From)
	assert.True(t, meta.Imported.WithHistory)

	// Opening the copy reminds that it is a copy of a git project.
	loc := ClassifyLocation(repo, dir)
	assert.Equal(t, LocationRepository, loc.Kind)
	assert.Equal(t, []string{NoticeGitCopy}, noticeIDs(loc.Notices))

	// The name is taken now.
	_, err = ImportedProjectDir(repo, "Tesis de Maestria")
	assert.ErrorIs(t, err, ErrProjectExists)
}

func TestCopyTreeNeverCopiesAGitFile(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{".git": "gitdir: /otro/repo/.git/worktrees/x\n", "main.tex": doc})
	dst := filepath.Join(t.TempDir(), "copia")
	require.NoError(t, CopyTree(context.Background(), src, dst, CopyOptions{HiddenFiles: importHiddenFiles, IncludeGit: true}))
	assert.NoFileExists(t, filepath.Join(dst, ".git"), "a worktree pointer would act on the original repository")
	assert.FileExists(t, filepath.Join(dst, "main.tex"))
}

func TestCopyTreeStopsWhenCanceled(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.tex": "a", "b.tex": "b"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := CopyTree(ctx, src, filepath.Join(t.TempDir(), "x"), CopyOptions{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFormatBytes(t *testing.T) {
	assert.Equal(t, "476 KB", FormatBytes(476*1024))
	assert.Equal(t, "31,2 MB", FormatBytes(32715571))
	assert.True(t, strings.HasSuffix(FormatBytes(1932735283), "GB"))
}

func noticeIDs(notices []LocationNotice) []string {
	ids := []string{}
	for _, n := range notices {
		ids = append(ids, n.ID)
	}
	return ids
}
