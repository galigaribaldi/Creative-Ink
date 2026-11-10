package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMacInterfaceStyle(t *testing.T) {
	assert.True(t, parseMacInterfaceStyle("Dark\n"))
	assert.False(t, parseMacInterfaceStyle(""))
	assert.False(t, parseMacInterfaceStyle("Light"))
}

func TestParseGSettingsColorScheme(t *testing.T) {
	for out, want := range map[string][2]bool{
		"'prefer-dark'\n":  {true, true},
		"'prefer-light'\n": {false, true},
		"'default'\n":      {false, false},
		"":                 {false, false},
	} {
		dark, known := parseGSettingsColorScheme(out)
		assert.Equal(t, want, [2]bool{dark, known}, out)
	}
}

func TestGtkThemeIsDark(t *testing.T) {
	for name, want := range map[string]bool{
		"'Adwaita-dark'": true, "Adwaita:dark": true, "Yaru-dark": true, "Arc-Dark-solid": true,
		"'Adwaita'": false, "Yaru": false, "Darkly": false, "": false,
	} {
		assert.Equal(t, want, gtkThemeIsDark(name), name)
	}
}

func TestSystemPrefersDarkNeverPanics(t *testing.T) {
	_ = SystemPrefersDark() // real OS query: only checks it returns
}
