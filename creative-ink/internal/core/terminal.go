package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// TerminalTimeout bounds every command run from the integrated terminal.
const TerminalTimeout = 30 * time.Second

// maxTerminalOutput caps what crosses the IPC bridge (e.g. `cat` of a big log).
const maxTerminalOutput = 256 * 1024

// terminalTimeout is a variable so tests can shorten it.
var terminalTimeout = TerminalTimeout

var (
	// ErrCommandTimeout is returned when a command exceeds TerminalTimeout.
	ErrCommandTimeout = errors.New("el comando superó el límite de 30 s y se detuvo")
	// ErrCommandCanceled is returned when the user stops the command.
	ErrCommandCanceled = errors.New("comando cancelado")
	// ErrEmptyCommand is returned for blank input.
	ErrEmptyCommand = errors.New("el comando está vacío")
)

// nonInteractiveEnv makes common tools fail fast instead of waiting for
// input that this simple terminal cannot provide (there is no TTY).
var nonInteractiveEnv = []string{
	"TERM=dumb",
	"NO_COLOR=1",
	"CLICOLOR=0",
	"PAGER=cat",
	"GIT_PAGER=cat",
	"GIT_TERMINAL_PROMPT=0", // no username/password prompts
	"GIT_EDITOR=true",       // `git commit` without -m aborts instead of hanging
}

// ExecuteCommand runs cmdString through the system shell (sh -c on
// macOS/Linux, cmd /c on Windows) in workdir and returns Stdout+Stderr
// combined, with a 30 s timeout. A non-zero exit returns the output and an
// *exec.ExitError.
//
// SECURITY: this executes arbitrary commands by design (it is a terminal).
// Only call it with text the user typed, never with text derived from
// documents or other untrusted input.
func ExecuteCommand(cmdString string, workdir string) (string, error) {
	return ExecuteCommandContext(context.Background(), cmdString, workdir, nil)
}

// ExecuteCommandContext is ExecuteCommand with cancellation (the UI's stop
// button, app shutdown) and extra environment variables (e.g. PATH).
func ExecuteCommandContext(parent context.Context, cmdString, workdir string, extraEnv []string) (string, error) {
	return ExecuteCommandStream(parent, cmdString, workdir, extraEnv, nil)
}

// ExecuteCommandStream is ExecuteCommandContext that also copies the whole
// output, as it is produced, to tee (the terminal spool; nil = none). The
// returned string is still capped at 256 KB; tee is not.
func ExecuteCommandStream(parent context.Context, cmdString, workdir string, extraEnv []string, tee io.Writer) (string, error) {
	if strings.TrimSpace(cmdString) == "" {
		return "", ErrEmptyCommand
	}
	if workdir == "" {
		return "", ErrNoFolder
	}
	if info, err := os.Stat(workdir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("la carpeta de trabajo %q no existe", workdir)
	}

	ctx, cancel := context.WithTimeout(parent, terminalTimeout)
	defer cancel()

	cmd := shellCommand(ctx, cmdString)
	cmd.Dir = workdir
	// Later duplicates win in cmd.Env, so extraEnv can override PATH.
	cmd.Env = append(append(os.Environ(), nonInteractiveEnv...), extraEnv...)
	cmd.Stdin = nil // reads get EOF at once: interactive prompts fail fast
	out := &cappedBuffer{limit: maxTerminalOutput}
	var w io.Writer = out
	if tee != nil {
		w = io.MultiWriter(out, tee)
	}
	cmd.Stdout = w // same writer: os/exec serialises the writes
	cmd.Stderr = w
	cmd.WaitDelay = 2 * time.Second // do not wait forever for orphaned pipes

	runErr := cmd.Run()
	output := out.String()

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return output, ErrCommandTimeout
	case parent.Err() != nil:
		return output, ErrCommandCanceled
	}
	return output, runErr
}

// cappedBuffer keeps the first limit bytes and counts the rest.
type cappedBuffer struct {
	buf     []byte
	limit   int
	dropped int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - len(b.buf); room > 0 {
		n := min(room, len(p))
		b.buf = append(b.buf, p[:n]...)
		b.dropped += len(p) - n
	} else {
		b.dropped += len(p)
	}
	return len(p), nil // never fail the command because the UI view is full
}

func (b *cappedBuffer) String() string {
	if b.dropped == 0 {
		return string(b.buf)
	}
	return fmt.Sprintf("%s\n… salida truncada: %d bytes más", b.buf, b.dropped)
}
