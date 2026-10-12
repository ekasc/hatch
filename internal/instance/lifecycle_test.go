// Gate 2 (1.3): kill/restart lifecycle with recovery via persisted state.
//
// Scenario against the gate0 counter backend (unmodified): actor boots,
// presses once (server count 0→1), is SIGKILLed mid-linger, restarts fresh
// with NO press, and must converge to the persisted count ("1") purely by
// re-reading backend state. Backend count must stay exactly 1 (no duplicate
// increment from the restart).
//
// External needs (env-gated, Skipped when absent — no hardcoded /tmp paths
// in the repo): HATCH_PROBE_BIN (probe binary), HATCH_GATE0_BUNDLE (Metro
// bundle), HATCH_GATE2_API (backend base URL, default :18788). The probe's
// JSTimers module id travels as HATCH_JSTIMERS_ID (default 184).
package instance

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gate2env(t *testing.T) (bin, bundle, api string) {
	t.Helper()
	bin, bundle = os.Getenv("HATCH_PROBE_BIN"), os.Getenv("HATCH_GATE0_BUNDLE")
	if bin == "" || bundle == "" {
		t.Skip("HATCH_PROBE_BIN and HATCH_GATE0_BUNDLE required")
	}
	api = os.Getenv("HATCH_GATE2_API")
	if api == "" {
		api = "http://127.0.0.1:18788"
	}
	if _, err := http.Get(api + "/count"); err != nil {
		t.Skipf("backend %s unreachable: %v", api, err)
	}
	return bin, bundle, api
}

func serverCount(api string) (int, error) {
	resp, err := http.Get(api + "/count")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var body struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Count, nil
}

func waitServerCount(t *testing.T, api string, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		n, err := serverCount(api)
		if err == nil && n == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server count did not reach %d (err=%v)", want, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func probeEnv(extra ...string) []string {
	jsid := os.Getenv("HATCH_JSTIMERS_ID")
	if jsid == "" {
		jsid = "184"
	}
	base := []string{"HATCH_JSTIMERS_ID=" + jsid, "HATCH_PUMP_CAP=200", "HATCH_FIBER=1"}
	return append(base, extra...)
}

// requireFreshCount is the count-0 precondition: freshness must be asserted,
// never inferred from forensics. Fail fast with the operator fix.
func requireFreshCount(t *testing.T, api string) {
	t.Helper()
	n, err := serverCount(api)
	if err != nil {
		t.Fatalf("backend %s unreachable: %v", api, err)
	}
	if n != 0 {
		t.Fatalf("backend count = %d, want 0 — restart the backend for a fresh run", n)
	}
}

func TestGate2KillRestartRecovery(t *testing.T) {
	bin, bundle, api := gate2env(t)
	requireFreshCount(t, api)

	// Actor 1: boots, presses once, lingers (kill window).
	a1, err := StartActor(Config{
		Engine: bin,
		Bundle: bundle,
		Env:    probeEnv("HATCH_PRESS=1", "HATCH_LINGER=60"),
	})
	if err != nil {
		t.Fatalf("start actor 1: %v", err)
	}
	// Server-observed press (not actor output): the increment landed.
	waitServerCount(t, api, 1, 60*time.Second)

	if err := a1.Kill(); err != nil {
		t.Fatalf("kill actor 1: %v", err)
	}
	res1, err := a1.Wait()
	if err != nil {
		t.Fatalf("wait actor 1: %v", err)
	}
	if res1.ExitCode == 0 {
		t.Fatalf("actor 1 exit = 0, want non-zero (killed)")
	}
	t.Logf("actor 1 killed, exit=%d", res1.ExitCode)
	if !strings.Contains(res1.Stdout, "PRESS dispatched:counter-bump") {
		t.Fatal("actor 1 log lacks press dispatch")
	}

	// Actor 2: FRESH boot of the same bundle, no press. Recovery must come
	// purely from backend-persisted state. (Restart reuses the exact cfg
	// including PRESS, which would re-press — so this is a fresh StartActor
	// without the press knob; Restart itself is unit-tested below.)
	a2, err := StartActor(Config{Engine: bin, Bundle: bundle, Env: probeEnv()})
	if err != nil {
		t.Fatalf("start actor 2: %v", err)
	}
	res2, err := a2.Wait()
	if err != nil {
		t.Fatalf("wait actor 2: %v", err)
	}
	if res2.ExitCode != 0 {
		t.Fatalf("actor 2 exit = %d, stderr tail: %s", res2.ExitCode, tail(res2.Stderr, 500))
	}
	if !strings.Contains(res2.Stdout, `text="1"`) {
		t.Fatal("actor 2 fiber lacks recovered counter \"1\"")
	}
	if strings.Contains(res2.Stdout, "PRESS dispatched") {
		t.Fatal("actor 2 must not press (recovery is read-only)")
	}
	n, err := serverCount(api)
	if err != nil || n != 1 {
		t.Fatalf("server count = %d, err = %v, want exactly 1 (no duplicate)", n, err)
	}
	t.Logf("gate2: kill exit=%d, restart converged, server=%d", res1.ExitCode, n)
}

// TestRestartReusesConfig: Restart reboots the identical cfg (same bundle,
// engine, env) after the old process is reaped; reaping is enforced.
func TestRestartReusesConfig(t *testing.T) {
	root := repoRoot(t)
	a, err := StartActor(Config{Bundle: filepath.Join(root, "testdata", "hello.js")})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := a.Restart(); err == nil {
		t.Fatal("Restart before Wait must fail")
	}
	res, err := a.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Stdout, "hatch instance alive") {
		t.Fatalf("first run = %+v", res)
	}
	b, err := a.Restart()
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	res2, err := b.Wait()
	if err != nil {
		t.Fatalf("wait restarted: %v", err)
	}
	if res2.ExitCode != 0 || !strings.Contains(res2.Stdout, "hatch instance alive") {
		t.Fatalf("restarted run = %+v", res2)
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
