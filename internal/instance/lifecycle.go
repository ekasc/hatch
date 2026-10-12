// Actor lifecycle: start, kill, restart of one headless app instance.
//
// An Actor is one OS process running an engine binary (the Fabric probe,
// node, …) over a bundle. The supervisor polls application- or
// server-observed state (never the actor's in-flight output buffers) to
// decide when to kill; output is read only after Wait returns.
package instance

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// Actor is a single supervised instance process.
type Actor struct {
	cfg    Config
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   bool
}

// StartActor launches the engine over the bundle and returns immediately.
// Extra environment (HATCH_* knobs) travels on Config.Env; the parent
// environment (PATH, LD_LIBRARY_PATH, …) is inherited.
func StartActor(cfg Config) (*Actor, error) {
	engine := cfg.Engine
	if engine == "" {
		engine = "node"
	}
	if cfg.Bundle == "" {
		return nil, fmt.Errorf("instance: bundle path is required")
	}
	args := append([]string{cfg.Bundle}, cfg.Args...)
	cmd := exec.Command(engine, args...)
	cmd.Env = os.Environ()
	if cfg.APIURL != "" {
		cmd.Env = append(cmd.Env, "HATCH_API_URL="+cfg.APIURL)
	}
	cmd.Env = append(cmd.Env, cfg.Env...)
	a := &Actor{cfg: cfg, cmd: cmd}
	cmd.Stdout = &a.stdout
	cmd.Stderr = &a.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("instance: start %q: %w", engine, err)
	}
	return a, nil
}

// Kill terminates the actor immediately (SIGKILL semantics: no graceful
// shutdown — recovery must come from persisted state, not from cleanup).
func (a *Actor) Kill() error {
	if a.cmd.Process == nil {
		return fmt.Errorf("instance: actor never started")
	}
	return a.cmd.Process.Kill()
}

// Wait reaps the actor and returns its captured output and exit code.
// A killed actor reports ExitCode -1 (signaled); that is a Result, not an
// error — only reaping a finished/never-started process is an error.
func (a *Actor) Wait() (Result, error) {
	if a.done {
		return Result{}, fmt.Errorf("instance: actor already reaped")
	}
	a.done = true
	res := Result{Stdout: a.stdout.String(), Stderr: a.stderr.String()}
	err := a.cmd.Wait()
	// Buffers may have grown during Wait; re-read for the final content.
	res.Stdout = a.stdout.String()
	res.Stderr = a.stderr.String()
	if err == nil {
		return res, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return Result{}, fmt.Errorf("instance: wait: %w", err)
}

// Restart boots the same bundle+engine+env as a fresh process. The old
// process must have been reaped (Wait) first. Any in-memory JS state is
// gone by construction — what survives is what the backend persisted.
func (a *Actor) Restart() (*Actor, error) {
	if !a.done {
		return nil, fmt.Errorf("instance: reap the old process before restart")
	}
	return StartActor(a.cfg)
}
