package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyLocationInsideTheRepository(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Join(repo, "Tesis")
	quick := filepath.Join(repo, QuickDocsDirName, "unnamed-tmp1")
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".git"), 0o755)) // git inside the repository: no notice
	require.NoError(t, os.MkdirAll(quick, 0o755))

	loc := ClassifyLocation(repo, project)
	assert.Equal(t, LocationRepository, loc.Kind)
	assert.Empty(t, loc.Notices)
	assert.Empty(t, loc.GitRoot)
	assert.Equal(t, LocationQuickDoc, ClassifyLocation(repo, quick).Kind)
}

func TestClassifyLocationExternalWithoutGitIsALightSuggestion(t *testing.T) {
	folder := t.TempDir()
	loc := ClassifyLocation(t.TempDir(), folder)
	assert.Equal(t, LocationExternal, loc.Kind)
	assert.Empty(t, loc.GitRoot)
	require.Len(t, loc.Notices, 1)
	assert.Equal(t, NoticeExternal, loc.Notices[0].ID)
	assert.Equal(t, NoticeInfo, loc.Notices[0].Level)
	assert.Contains(t, loc.Notices[0].Text, "Sugerencia")
	assert.Contains(t, loc.Notices[0].Text, folder)
}

func TestClassifyLocationExternalGitIsAStrongWarning(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "docs", "tesis")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	loc := ClassifyLocation("", project)
	assert.Equal(t, root, loc.GitRoot, "a .git in a parent counts")
	require.NotEmpty(t, loc.Notices)
	n := loc.Notices[0]
	assert.Equal(t, NoticeExternalGit, n.ID)
	assert.Equal(t, NoticeWarning, n.Level)
	assert.Contains(t, n.Text, "NO se recomienda copiarla")
	assert.Contains(t, n.Text, root)
	assert.Contains(t, strings.Join(n.Steps, "\n"), "git commit")
}

func TestFindGitRootAcceptsAGitFileAndStopsAtHome(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(home, "wt")
	require.NoError(t, os.MkdirAll(worktree, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: ../repo/.git/worktrees/wt\n"), 0o644))
	assert.Equal(t, worktree, FindGitRoot(worktree, home))

	// A dotfiles repository in ~ does not make every project versioned.
	require.NoError(t, os.Mkdir(filepath.Join(home, ".git"), 0o755))
	plain := filepath.Join(home, "Proyectos", "Tesis")
	require.NoError(t, os.MkdirAll(plain, 0o755))
	assert.Empty(t, FindGitRoot(plain, home))
}

func TestSyncServiceRecognisesCloudFolders(t *testing.T) {
	cases := map[string]string{
		"/Users/u/Library/Mobile Documents/com~apple~CloudDocs/Tesis": "iCloud Drive",
		"/Users/u/Library/CloudStorage/Dropbox/Tesis":                 "Dropbox",
		"/Users/u/Library/CloudStorage/OneDrive-Personal/Tesis":       "OneDrive",
		"/Users/u/Library/CloudStorage/GoogleDrive-u@mail.com/Tesis":  "Google Drive",
		"/home/u/Dropbox/Tesis":                                       "Dropbox",
		"/home/u/Proyectos/Tesis":                                     "",
		`C:/Users/u/OneDrive - Universidad/Tesis`:                     "OneDrive",
	}
	for path, want := range cases {
		assert.Equal(t, want, SyncService(filepath.FromSlash(path), ""), path)
	}
}

func TestExternalSyncedFolderAddsTheSyncWarning(t *testing.T) {
	dropbox := filepath.Join(t.TempDir(), "Dropbox", "Tesis")
	require.NoError(t, os.MkdirAll(dropbox, 0o755))
	loc := ClassifyLocation("", dropbox)
	assert.Equal(t, "Dropbox", loc.SyncService)
	require.Len(t, loc.Notices, 2)
	assert.Equal(t, NoticeSynced, loc.Notices[1].ID)
	assert.Equal(t, NoticeWarning, loc.Notices[1].Level)
}

func TestCheckProjectFolderRejectsBroadFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	repo := filepath.Join(home, "Documents", "My-Creative-Ink-Repository")

	for _, dir := range []string{
		home,
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Descargas"),
		filepath.Join(home, "Documents"),
		repo,
		filepath.Join(repo, MetaDirName),
		filepath.Join(repo, MetaDirName, "templates"),
		filepath.Join(repo, QuickDocsDirName),
		string(filepath.Separator),
	} {
		assert.ErrorIs(t, CheckProjectFolder(repo, dir), ErrBroadFolder, dir)
	}
	for _, dir := range []string{
		filepath.Join(home, "Desktop", "Tesis"),
		filepath.Join(repo, "Tesis"),
		filepath.Join(repo, QuickDocsDirName, "unnamed-tmp1"),
	} {
		assert.NoError(t, CheckProjectFolder(repo, dir), dir)
	}
}

func TestGitCopyNoteExplainsHowToKeepTheOriginalInSync(t *testing.T) {
	whole := GitCopyNoteFile(ImportInfo{From: "/home/u/tesis", GitRoot: "/home/u/tesis", WithHistory: true})
	assert.Contains(t, whole, "COPIA")
	assert.Contains(t, whole, "/home/u/tesis")
	assert.Contains(t, whole, "git push")
	assert.Contains(t, whole, "git pull")

	part := GitCopyNoteFile(ImportInfo{From: "/home/u/repo/Registro1", GitRoot: "/home/u/repo"})
	assert.Contains(t, part, "NO tiene historial")
	assert.Contains(t, part, "/home/u/repo/Registro1")
	assert.Contains(t, part, "Abrir en su ubicación")
	assert.Len(t, gitSaveSteps, 5, "GitCopyNotice must not alter the shared steps")
}
