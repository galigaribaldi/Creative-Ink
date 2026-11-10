//go:build linux

package system

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// prefersDark follows what WebKitGTK uses for prefers-color-scheme: an
// explicit GTK_THEME, then GNOME's color-scheme, then the GTK theme name.
// gsettings missing (other desktops) or unknown → light.
func prefersDark() bool {
	if theme := os.Getenv("GTK_THEME"); theme != "" {
		return gtkThemeIsDark(theme)
	}
	if out, ok := gsettings("color-scheme"); ok {
		if dark, known := parseGSettingsColorScheme(out); known {
			return dark
		}
	}
	if out, ok := gsettings("gtk-theme"); ok {
		return gtkThemeIsDark(out)
	}
	return false
}

func gsettings(key string) (string, bool) {
	bin, err := exec.LookPath("gsettings")
	if err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "get", "org.gnome.desktop.interface", key).Output()
	return string(out), err == nil
}
