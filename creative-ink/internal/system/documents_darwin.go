//go:build darwin

package system

import (
	"os"
	"path/filepath"
	"strings"
)

// userDocumentsDir is ~/Documents: its location is fixed on macOS.
func userDocumentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents"), nil
}

// sharedDocumentsDir is /Users/Shared: visible to every user in Finder and
// world-writable (with the sticky bit) by default.
func sharedDocumentsDir() (string, string) {
	const shared = "/Users/Shared"
	if info, err := os.Stat(shared); err != nil || !info.IsDir() {
		return "", "Este Mac no tiene la carpeta /Users/Shared"
	}
	return shared, ""
}

// storageWarnings flags iCloud "Desktop & Documents" sync: every .aux/.pdf
// written on each compilation would be uploaded, and files may be offloaded.
func storageWarnings(userDocs string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	iCloudDocs := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "Documents")
	real, _ := filepath.EvalSymlinks(userDocs)
	if strings.Contains(real, "Mobile Documents") || dirExists(iCloudDocs) {
		return []string{"Parece que Documentos se sincroniza con iCloud: cada compilación subirá " +
			"archivos auxiliares y macOS puede dejar documentos solo en la nube. " +
			"Si prefieres evitarlo, elige \"Para todos los usuarios\" (/Users/Shared)."}
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
