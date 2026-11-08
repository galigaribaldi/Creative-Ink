//go:build windows

package core

import "golang.org/x/sys/windows"

// hidePath sets FILE_ATTRIBUTE_HIDDEN: on Windows a leading dot does not
// hide a folder.
func hidePath(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return err
	}
	return windows.SetFileAttributes(p, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
}
