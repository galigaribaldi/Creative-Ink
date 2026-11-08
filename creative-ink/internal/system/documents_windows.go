//go:build windows

package system

import (
	"strings"

	"golang.org/x/sys/windows"
)

// userDocumentsDir asks Windows for the real Documents folder (Known
// Folders): it may be redirected to OneDrive or by group policy, so
// %USERPROFILE%\Documents can be the wrong place.
func userDocumentsDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
}

// sharedDocumentsDir is Public Documents (usually C:\Users\Public\Documents),
// visible to and writable by every user.
func sharedDocumentsDir() (string, string) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_PublicDocuments, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", "No se pudo localizar Documentos públicos: " + err.Error()
	}
	return path, ""
}

func storageWarnings(userDocs string) []string {
	if strings.Contains(strings.ToLower(userDocs), "onedrive") {
		return []string{"Documentos está en OneDrive: cada compilación sincronizará archivos " +
			"auxiliares. Si prefieres evitarlo, elige \"Para todos los usuarios\" (Documentos públicos)."}
	}
	return nil
}
