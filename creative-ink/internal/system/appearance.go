package system

import "strings"

// SystemPrefersDark reports whether the operating system uses a dark
// appearance when the app starts. It is used once, before the window
// exists, to paint the native window with the same colour as the CSS theme
// (no flash of the wrong theme). It reports false when the OS cannot tell:
// the WebView then most likely resolves prefers-color-scheme to light too.
func SystemPrefersDark() bool { return prefersDark() }

// parseMacInterfaceStyle reads `defaults read -g AppleInterfaceStyle`: it
// prints "Dark" in dark mode and fails (key not set) in light mode.
func parseMacInterfaceStyle(out string) bool {
	return strings.EqualFold(strings.TrimSpace(out), "Dark")
}

// parseGSettingsColorScheme reads `gsettings get org.gnome.desktop.interface
// color-scheme` (GNOME 42+): 'prefer-dark', 'prefer-light' or 'default'.
// known is false for 'default' or anything unexpected.
func parseGSettingsColorScheme(out string) (dark, known bool) {
	switch strings.Trim(strings.TrimSpace(out), `'"`) {
	case "prefer-dark":
		return true, true
	case "prefer-light":
		return false, true
	}
	return false, false
}

// gtkThemeIsDark recognises dark GTK theme names ("Adwaita-dark",
// "Yaru-dark", "Adwaita:dark" from GTK_THEME, "Arc-Dark-solid").
func gtkThemeIsDark(name string) bool {
	n := strings.ToLower(strings.Trim(strings.TrimSpace(name), `'"`))
	return strings.HasSuffix(n, "-dark") || strings.Contains(n, ":dark") || strings.Contains(n, "-dark-")
}
