//go:build !darwin && !linux && !windows

package system

import (
	"os"
	"path/filepath"
)

// Other platforms (e.g. BSD) are not supported targets: use ~/Documents.
func userDocumentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents"), nil
}

func sharedDocumentsDir() (string, string) {
	return "", "Este sistema operativo no tiene una carpeta compartida conocida"
}

func storageWarnings(string) []string { return nil }
