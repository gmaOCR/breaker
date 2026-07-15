# 06 — Kill path (run mode)

`breaker run` wraps the agent as a child process and terminates it when the
breaker trips.

## Mechanism (POSIX)

- The child is started with `SysProcAttr{Setpgid: true}`, so it leads its own
  process group — killing the group takes down anything the agent spawned too.
- On trip, `runner` signals the **group**: `SIGTERM`, then after `--grace`
  (default 3s), `SIGKILL`. A negative pid (`-pgid`) targets the whole group.
- The run's exit code is the child's own code, or **137** when the breaker killed
  it (`runner.TripExitCode`). If the child slipped out via a 402 before the kill
  landed, the exit code is still forced to 137 so CI sees a failure.

## Selection between trip and normal exit

`runner.Run` selects over three channels: the engine's trip channel, the child's
own exit, and context cancellation. Whichever fires first wins; the others are
drained.

## Windows

Windows has no process groups here, so only the **direct child** is killed
(best-effort). This is a documented limitation; POSIX gets the full group kill.
