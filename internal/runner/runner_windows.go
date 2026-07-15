//go:build windows

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// TripExitCode is the exit code returned when a run is killed by the breaker.
const TripExitCode = 137

// Run on Windows kills only the direct child — no process-group support here.
// Documented best-effort limitation; POSIX gets the full group kill.
func Run(ctx context.Context, argv []string, env []string, trips <-chan core.TripReason, grace time.Duration) (int, core.TripReason, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return -1, core.TripReason{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case r := <-trips:
		_ = cmd.Process.Kill()
		<-done
		return TripExitCode, r, nil
	case err := <-done:
		return exitCode(err), core.TripReason{}, nil
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return TripExitCode, core.TripReason{}, nil
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
