package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Workspace trust (docs/MAP.md §4.6, step 3). latexmk executes the Perl
// code of a folder's .latexmkrc / latexmkrc. A project downloaded from the
// internet could run anything through it, so a folder with such a file is
// only compiled with its configuration once the user trusted that exact
// content (SHA-256). Otherwise latexmk runs with -norc.
//
// The trusted folder is the one where latexmk runs (the compile target's
// folder): that is the directory whose rc files latexmk reads.

// TrustState is the trust status of a folder.
type TrustState string

const (
	// TrustTrusted: no rc file (no project code to run), or its hash
	// matches the one the user trusted.
	TrustTrusted TrustState = "trusted"
	// TrustUntrusted: there is an rc file the user never trusted, or it
	// changed since. The UI asks; latexmk runs with -norc.
	TrustUntrusted TrustState = "untrusted"
	// TrustRestricted: the user explicitly refused it. latexmk runs with
	// -norc and the UI does not ask again.
	TrustRestricted TrustState = "restricted"
)

// latexmkRCNames are the project rc files latexmk reads from its working
// directory, in its own order.
var latexmkRCNames = []string{"latexmkrc", ".latexmkrc"}

// TrustInfo describes a folder's trust, for the UI and `ci trust status`.
type TrustInfo struct {
	Folder      string     `json:"folder"` // real absolute path (the trust key)
	Status      TrustState `json:"status"`
	RCFiles     []string   `json:"rcFiles"`     // rc file names found in Folder
	CurrentHash string     `json:"currentHash"` // SHA-256 of those files, "" if none
	TrustedHash string     `json:"trustedHash"` // hash the user trusted, "" if none
}

// NeedsNoRC reports whether latexmk must run with -norc in this folder.
func (t TrustInfo) NeedsNoRC() bool {
	return len(t.RCFiles) > 0 && t.Status != TrustTrusted
}

// ErrNoTrustFolder is returned when there is no folder to (un)trust.
var ErrNoTrustFolder = errors.New("no hay carpeta de proyecto para la confianza")

// RCHash returns the latexmk rc files of dir and the SHA-256 of their names
// and contents ("" and nil when there are none).
func RCHash(dir string) (hash string, files []string, err error) {
	h := sha256.New()
	for _, name := range latexmkRCNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("leer %s: %w", name, err)
		}
		files = append(files, name)
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	if len(files) == 0 {
		return "", nil, nil
	}
	return hex.EncodeToString(h.Sum(nil)), files, nil
}

// trustKey is the canonical form of a folder: absolute, cleaned and with
// symlinks resolved, so two paths to the same folder share one entry.
func trustKey(folder string) (string, error) {
	if folder == "" {
		return "", ErrNoTrustFolder
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	return filepath.Clean(abs), nil
}

// GetTrustStatus returns the trust of folder according to the settings at
// settingsPath (a missing settings file means nothing is trusted).
func GetTrustStatus(settingsPath, folder string) (TrustInfo, error) {
	key, err := trustKey(folder)
	if err != nil {
		return TrustInfo{}, err
	}
	hash, files, err := RCHash(key)
	if err != nil {
		return TrustInfo{}, err
	}
	var s Settings
	if settingsPath != "" {
		if s, err = ReadSettings(settingsPath); err != nil {
			return TrustInfo{}, err
		}
	}
	info := TrustInfo{Folder: key, RCFiles: files, CurrentHash: hash, TrustedHash: s.TrustedProjects[key]}
	switch {
	case slices.Contains(s.RestrictedProjects, key):
		info.Status = TrustRestricted
	case len(files) == 0, info.TrustedHash == hash:
		info.Status = TrustTrusted
	default:
		info.Status = TrustUntrusted
	}
	if info.RCFiles == nil {
		info.RCFiles = []string{}
	}
	return info, nil
}

// TrustProject trusts folder's current rc files (their hash is saved) and
// lifts an explicit restriction. With no rc file it only lifts it.
func TrustProject(settingsPath, folder string) (TrustInfo, error) {
	key, err := trustKey(folder)
	if err != nil {
		return TrustInfo{}, err
	}
	hash, _, err := RCHash(key)
	if err != nil {
		return TrustInfo{}, err
	}
	if _, err := UpdateSettings(settingsPath, func(s *Settings) {
		s.RestrictedProjects = slices.DeleteFunc(s.RestrictedProjects, func(p string) bool { return p == key })
		if hash == "" {
			delete(s.TrustedProjects, key)
			return
		}
		if s.TrustedProjects == nil {
			s.TrustedProjects = map[string]string{}
		}
		s.TrustedProjects[key] = hash
	}); err != nil {
		return TrustInfo{}, fmt.Errorf("guardar la confianza: %w", err)
	}
	return GetTrustStatus(settingsPath, key)
}

// UntrustProject forgets folder's trusted hash and marks it as restricted:
// latexmk runs with -norc there and the UI stops asking.
func UntrustProject(settingsPath, folder string) (TrustInfo, error) {
	key, err := trustKey(folder)
	if err != nil {
		return TrustInfo{}, err
	}
	if _, err := UpdateSettings(settingsPath, func(s *Settings) {
		delete(s.TrustedProjects, key)
		if !slices.Contains(s.RestrictedProjects, key) {
			s.RestrictedProjects = append(s.RestrictedProjects, key)
		}
	}); err != nil {
		return TrustInfo{}, fmt.Errorf("guardar la restricción: %w", err)
	}
	return GetTrustStatus(settingsPath, key)
}
