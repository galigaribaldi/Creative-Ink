package system

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLinux answers getenv from env and finds only the installed binaries.
func fakeLinux(env map[string]string, installed ...string) (func(string) string, func(string) (string, error)) {
	have := map[string]bool{}
	for _, b := range installed {
		have[b] = true
	}
	getenv := func(k string) string { return env[k] }
	lookPath := func(b string) (string, error) {
		if have[b] {
			return "/usr/bin/" + b, nil
		}
		return "", errors.New("not found")
	}
	return getenv, lookPath
}

func TestLinuxTerminalPrefersTheTERMINALVariable(t *testing.T) {
	getenv, lookPath := fakeLinux(map[string]string{"TERMINAL": "kitty --single-instance", "XDG_CURRENT_DESKTOP": "KDE"}, "kitty", "konsole")
	path, args, name, err := linuxTerminal("/home/u/Tesis", getenv, lookPath)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/kitty", path)
	assert.Equal(t, []string{"--single-instance"}, args) // the folder goes through cmd.Dir
	assert.Equal(t, "kitty", name)
}

func TestLinuxTerminalUsesTheDesktopsOwnEmulator(t *testing.T) {
	getenv, lookPath := fakeLinux(map[string]string{"XDG_CURRENT_DESKTOP": "ubuntu:GNOME"}, "x-terminal-emulator", "xterm", "gnome-terminal")
	path, args, name, err := linuxTerminal("/home/u/Tesis", getenv, lookPath)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/gnome-terminal", path)
	assert.Equal(t, []string{"--working-directory=/home/u/Tesis"}, args)
	assert.Equal(t, "GNOME Terminal", name)

	getenv, lookPath = fakeLinux(map[string]string{"XDG_CURRENT_DESKTOP": "KDE"}, "xterm", "konsole")
	_, args, name, err = linuxTerminal("/home/u/Tesis", getenv, lookPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"--workdir", "/home/u/Tesis"}, args)
	assert.Equal(t, "Konsole", name)
}

func TestLinuxTerminalFallsBackToAnyInstalledEmulator(t *testing.T) {
	// A missing $TERMINAL program and an unknown desktop are skipped.
	getenv, lookPath := fakeLinux(map[string]string{"TERMINAL": "no-such-term", "XDG_CURRENT_DESKTOP": "sway"}, "alacritty", "xterm")
	path, args, _, err := linuxTerminal("/home/u/Tesis", getenv, lookPath)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/alacritty", path)
	assert.Equal(t, []string{"--working-directory", "/home/u/Tesis"}, args)

	getenv, lookPath = fakeLinux(nil, "xterm")
	_, args, name, err := linuxTerminal("/home/u/Tesis", getenv, lookPath)
	require.NoError(t, err)
	assert.Empty(t, args) // xterm starts in cmd.Dir
	assert.Equal(t, "xterm", name)
}

func TestLinuxTerminalWithoutAnyEmulator(t *testing.T) {
	getenv, lookPath := fakeLinux(nil)
	_, _, _, err := linuxTerminal("/home/u/Tesis", getenv, lookPath)
	assert.ErrorIs(t, err, ErrNoTerminal)
}

func TestEveryFallbackEmulatorIsKnown(t *testing.T) {
	for _, id := range linuxFallbackOrder {
		assert.Contains(t, linuxLaunchers, id)
	}
	for desktop, ids := range desktopTerminals {
		for _, id := range ids {
			assert.Contains(t, linuxLaunchers, id, desktop)
		}
	}
}

func TestOpenTerminalRejectsAMissingFolder(t *testing.T) {
	_, err := OpenTerminal("/no/existe/creative-ink")
	assert.Error(t, err)
}
