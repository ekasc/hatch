package instance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func TestRunHelloBundle(t *testing.T) {
	root := repoRoot(t)
	res, err := Run(Config{Bundle: filepath.Join(root, "testdata", "hello.js")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "hatch instance alive") {
		t.Fatalf("stdout = %q, want hello line", res.Stdout)
	}
}

func TestRunMissingBundle(t *testing.T) {
	if _, err := Run(Config{}); err == nil {
		t.Fatal("expected error for empty bundle path")
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	root := repoRoot(t)
	res, err := Run(Config{Bundle: filepath.Join(root, "testdata", "fail.js")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestRunSetsAPIEnv(t *testing.T) {
	root := repoRoot(t)
	res, err := Run(Config{
		Bundle: filepath.Join(root, "testdata", "env.js"),
		APIURL: "http://127.0.0.1:18787",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "http://127.0.0.1:18787") {
		t.Fatalf("stdout = %q, want API URL", res.Stdout)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
