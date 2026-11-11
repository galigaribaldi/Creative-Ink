package system

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNoTerminal is returned when no terminal emulator could be found
// (Linux without $TERMINAL and none of the known emulators installed).
var ErrNoTerminal = errors.New("no se encontró una terminal del sistema: instala una o define la variable TERMINAL")

// OpenTerminal opens the OS's own terminal (Terminal on macOS, the
// desktop's emulator on Linux, Windows Terminal or cmd.exe on Windows) in
// dir, and returns its name. The opener runs directly, without a shell,
// and receives no command: only the folder, which callers must validate
// first (the App passes its open folder). It does not wait for the window.
func OpenTerminal(dir string) (string, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("la carpeta %q no existe", dir)
	}
	cmd, name, err := terminalCommand(dir)
	if err != nil {
		return "", err
	}
	cmd.Dir = dir // emulators without a working-directory flag start here
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("abrir %s: %w", name, err)
	}
	// Some emulators (xterm, cmd.exe) live as long as their window: reap
	// them in the background instead of blocking the caller.
	go func() { _ = cmd.Wait() }()
	return name, nil
}

// terminalLauncher is one way to start a terminal emulator in a folder.
type terminalLauncher struct {
	Name string                    // shown to the user: "Konsole"
	Bin  string                    // looked up on PATH: "konsole"
	Args func(dir string) []string // its working-directory flags (nil = none: cmd.Dir)
}

// linuxLaunchers are the known emulators (the order is linuxFallbackOrder),
// with the flag each one uses to start in a folder.
var linuxLaunchers = map[string]terminalLauncher{
	"x-terminal-emulator": {"x-terminal-emulator", "x-terminal-emulator", nil}, // Debian/Ubuntu default (alternatives)
	"ptyxis":              {"Ptyxis", "ptyxis", func(d string) []string { return []string{"--new-window", "--working-directory=" + d} }},
	"gnome-terminal":      {"GNOME Terminal", "gnome-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	"kgx":                 {"GNOME Console", "kgx", func(d string) []string { return []string{"--working-directory=" + d} }},
	"konsole":             {"Konsole", "konsole", func(d string) []string { return []string{"--workdir", d} }},
	"xfce4-terminal":      {"Xfce Terminal", "xfce4-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	"mate-terminal":       {"MATE Terminal", "mate-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	"qterminal":           {"QTerminal", "qterminal", func(d string) []string { return []string{"--workdir", d} }},
	"tilix":               {"Tilix", "tilix", func(d string) []string { return []string{"--working-directory=" + d} }},
	"terminator":          {"Terminator", "terminator", func(d string) []string { return []string{"--working-directory=" + d} }},
	"kitty":               {"kitty", "kitty", func(d string) []string { return []string{"--directory", d} }},
	"alacritty":           {"Alacritty", "alacritty", func(d string) []string { return []string{"--working-directory", d} }},
	"wezterm":             {"WezTerm", "wezterm", func(d string) []string { return []string{"start", "--cwd", d} }},
	"foot":                {"foot", "foot", func(d string) []string { return []string{"--working-directory=" + d} }},
	"xterm":               {"xterm", "xterm", nil},
}

// desktopTerminals is the emulator each desktop ships, keyed by an entry
// of $XDG_CURRENT_DESKTOP (a colon-separated list, e.g. "ubuntu:GNOME").
var desktopTerminals = map[string][]string{
	"GNOME":    {"ptyxis", "gnome-terminal", "kgx"},
	"UNITY":    {"gnome-terminal"},
	"CINNAMON": {"gnome-terminal"},
	"KDE":      {"konsole"},
	"XFCE":     {"xfce4-terminal"},
	"MATE":     {"mate-terminal"},
	"LXQT":     {"qterminal"},
}

// linuxFallbackOrder is tried after $TERMINAL and the desktop's own.
var linuxFallbackOrder = []string{
	"x-terminal-emulator", "ptyxis", "gnome-terminal", "kgx", "konsole", "xfce4-terminal",
	"mate-terminal", "qterminal", "tilix", "terminator", "kitty", "alacritty", "wezterm", "foot", "xterm",
}

// linuxTerminal picks the emulator to open on Linux (and other Unix
// desktops): $TERMINAL first (a program, optionally with its own
// arguments; it starts in the folder through cmd.Dir), then the current
// desktop's emulator, then the first known one installed. It returns the
// program's path and arguments. getenv and lookPath are injected for tests.
func linuxTerminal(dir string, getenv func(string) string, lookPath func(string) (string, error)) (path string, args []string, name string, err error) {
	if fields := strings.Fields(getenv("TERMINAL")); len(fields) > 0 {
		if p, lookErr := lookPath(fields[0]); lookErr == nil {
			return p, fields[1:], fields[0], nil
		}
	}
	var order []string
	for _, desktop := range strings.Split(getenv("XDG_CURRENT_DESKTOP"), ":") {
		order = append(order, desktopTerminals[strings.ToUpper(strings.TrimSpace(desktop))]...)
	}
	order = append(order, linuxFallbackOrder...)
	for _, id := range order {
		l := linuxLaunchers[id]
		p, lookErr := lookPath(l.Bin)
		if lookErr != nil {
			continue
		}
		if l.Args != nil {
			args = l.Args(dir)
		}
		return p, args, l.Name, nil
	}
	return "", nil, "", ErrNoTerminal
}
