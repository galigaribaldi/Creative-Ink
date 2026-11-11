package core

// cmdLine is the raw Windows command line for running command through
// cmd.exe: `"<shell>" /d /s /c "<command>"`. With /s, cmd.exe removes only
// the outer pair of quotes added here and keeps the command's own quotes
// intact (see shellCommand in terminal_windows.go). It lives in a file
// without build tags so it is tested on every OS.
func cmdLine(shell, command string) string {
	return `"` + shell + `" /d /s /c "` + command + `"`
}
