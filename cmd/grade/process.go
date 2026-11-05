package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const pipeWaitDelay = 250 * time.Millisecond

type result struct {
	status   int
	stdout   string
	stderr   string
	timedOut bool
	started  time.Time
	finished time.Time
}

func invoke(executable, dir string, args []string, timeout time.Duration) (result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, args...) // #nosec G204 G702 -- grading the user-selected executable is the purpose of this command.
	cmd.Dir = dir
	cmd.WaitDelay = pipeWaitDelay
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	var r result
	r.started = time.Now()
	err := cmd.Run()
	r.finished = time.Now()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.Is(err, exec.ErrWaitDelay):
		r.timedOut = true
	case errors.As(err, &exit):
		r.status = exit.ExitCode()
		r.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	default:
		return result{}, err
	}
	r.stdout = strings.ReplaceAll(stdout.String(), "\r\n", "\n")
	r.stderr = stderr.String()
	return r, nil
}
