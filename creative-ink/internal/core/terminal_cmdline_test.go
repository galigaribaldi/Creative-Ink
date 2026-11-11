package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCmdLineWrapsTheCommandForSlashS(t *testing.T) {
	shell := `C:\Windows\System32\cmd.exe`
	cases := map[string]string{
		"dir":                     `"C:\Windows\System32\cmd.exe" /d /s /c "dir"`,
		`git commit -m "mensaje"`: `"C:\Windows\System32\cmd.exe" /d /s /c "git commit -m "mensaje""`,
		// The case that broke with a bare /c: it starts with a quote and has four.
		`"C:\Program Files\Git\bin\git.exe" log "--format=%h %s"`: `"C:\Windows\System32\cmd.exe" /d /s /c ""C:\Program Files\Git\bin\git.exe" log "--format=%h %s""`,
	}
	for command, want := range cases {
		assert.Equal(t, want, cmdLine(shell, command), command)
	}
}
