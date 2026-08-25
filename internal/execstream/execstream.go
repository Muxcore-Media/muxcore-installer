// Package execstream runs subprocesses (or long-running Go funcs) and
// forwards their output line-by-line to a sink, so the TUI's bottom
// viewport can tail real command output while the interactive wizard stays
// on the main pane above it.
package execstream

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
)

// Line is one unit of streamed output.
type Line struct {
	Source string // logical step name, e.g. "up.sh", "download", "bootstrap-auth.sh"
	Text   string
	Stderr bool
}

// Sink receives lines as they arrive. Implementations must be safe to call
// from a background goroutine (the TUI wires this to tea.Program.Send).
type Sink func(Line)

// Command runs name(args...) with dir/env, streaming combined stdout+stderr
// line-by-line to sink, and returns the process error (if any).
func Command(ctx context.Context, source, dir string, env []string, sink Sink, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{}, 2)
	pump := func(r io.Reader, isErr bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			if sink != nil {
				sink(Line{Source: source, Text: sc.Text(), Stderr: isErr})
			}
		}
		done <- struct{}{}
	}
	go pump(stdout, false)
	go pump(stderr, true)
	<-done
	<-done
	return cmd.Wait()
}

// Env builds a full environment slice (os.Environ() + overrides), the shape
// exec.Cmd wants.
func Env(overrides map[string]string) []string {
	env := os.Environ()
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}
