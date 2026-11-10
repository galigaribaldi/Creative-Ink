package main

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// The native window colour must be the theme's --surface-base, or the app
// shows a band of another colour on startup.
func TestWindowBackgroundMatchesTheTokens(t *testing.T) {
	css, err := os.ReadFile("frontend/src/style.css")
	require.NoError(t, err)
	token := func(theme string) string {
		block := regexp.MustCompile(`(?s):root\[data-theme='` + theme + `'\]\s*\{(.*?)\n\}`).FindSubmatch(css)
		require.NotNil(t, block, "theme block %q", theme)
		value := regexp.MustCompile(`--surface-base:\s*(#[0-9a-fA-F]{6})`).FindSubmatch(block[1])
		require.NotNil(t, value, "--surface-base in %q", theme)
		return string(value[1])
	}
	hex := func(c *options.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

	assert.Equal(t, token("dark"), hex(windowBackground(true)))
	assert.Equal(t, token("light"), hex(windowBackground(false)))
	assert.EqualValues(t, 255, windowBackground(true).A, "opaque: Wails paints it under the WebView")
}
