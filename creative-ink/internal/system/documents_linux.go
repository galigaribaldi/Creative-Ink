//go:build linux

package system

import (
	"os"
	"path/filepath"
)

// userDocumentsDir follows XDG user dirs, so the repository lands in the
// Documents folder the file manager shows (e.g. ~/Documentos), not in a
// ~/Documents nobody sees.
func userDocumentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return xdgDocumentsDir(home), nil
}

// Linux has no standard folder shared by all users that a regular user can
// write to without sudo and group setup, so only the per-user scope exists.
func sharedDocumentsDir() (string, string) {
	return "", "Linux no tiene una carpeta de documentos compartida estándar; " +
		"el repositorio se crea en tus Documentos"
}

func storageWarnings(userDocs string) []string {
	home, err := os.UserHomeDir()
	if err != nil || userDocs == filepath.Join(home, "Documents") {
		return nil
	}
	return []string{"Tu carpeta de Documentos es " + userDocs + " (configuración XDG del escritorio)."}
}
