//go:build windows

package system

import "golang.org/x/sys/windows/registry"

// prefersDark reads the "app mode" of Windows 10/11 (Settings → Colours):
// AppsUseLightTheme = 0 means dark. WebView2's prefers-color-scheme
// follows the same setting.
func prefersDark() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	light, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err == nil && light == 0
}
