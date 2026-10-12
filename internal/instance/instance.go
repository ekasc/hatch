// Package instance boots one headless app instance: a JS engine subprocess
// running a bundle with the hatch environment (API URL, module registry).
package instance

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// Config describes one headless instance.
type Config struct {
	// Engine is the executable to run (probe binary, node, …).
	Engine string
	// Bundle is the path to the JS bundle to execute.
	Bundle string
	// APIURL is exposed to the bundle as HATCH_API_URL.
	APIURL string
	// Env carries extra KEY=VALUE pairs (HATCH_* knobs); the parent
	// environment is always inherited.
	Env []string
	// Args are passed to the bundle after the bundle path.
	Args []string
}

// Result captures a finished instance run.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes the bundle to completion and returns its output and exit code.
// A non-zero engine exit is returned as a Result, not an error; only failures
// to start the engine itself are errors.
func Run(cfg Config) (Result, error) {
	engine := cfg.Engine
	if engine == "" {
		engine = "node"
	}
	if cfg.Bundle == "" {
		return Result{}, fmt.Errorf("instance: bundle path is required")
	}
	args := append([]string{cfg.Bundle}, cfg.Args...)
	cmd := exec.Command(engine, args...)
	if cfg.APIURL != "" {
		cmd.Env = append(os.Environ(), "HATCH_API_URL="+cfg.APIURL)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return res, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return Result{}, fmt.Errorf("instance: start engine %q: %w", engine, err)
}
