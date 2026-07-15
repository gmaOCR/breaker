//go:build unix

// Package runner launches a child process, injects the proxy env, and kills the
// whole child process group when the breaker trips.
package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// TripExitCode is the exit code returned when a run is killed by the breaker.
const TripExitCode = 137

// Run executes argv with env, killing the child's process group when a trip
// arrives on trips. It returns the child's exit code and the trip reason (zero
// if the child exited on its own).
func Run(ctx context.Context, argv []string, env []string, trips <-chan core.TripReason, grace time.Duration) (int, core.TripReason, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return -1, core.TripReason{}, err
	}
	pgid := cmd.Process.Pid // child is its own group leader (Setpgid), so pgid == pid

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case r := <-trips:
		terminate(pgid, grace)
		<-done
		return TripExitCode, r, nil
	case err := <-done:
		return exitCode(err), core.TripReason{}, nil
	case <-ctx.Done():
		terminate(pgid, grace)
		<-done
		return TripExitCode, core.TripReason{}, nil
	}
}

// terminate signals the whole process group: SIGTERM, a grace period, then SIGKILL.
func terminate(pgid int, grace time.Duration) {
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(grace)
	_ = syscall.Kill(-pgid, syscall.SIGKILL) // no-op (ESRCH) if already gone
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
