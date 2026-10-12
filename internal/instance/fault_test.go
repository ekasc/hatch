// Gate 3 (2.1): fault injection — backend death + restart, detecting the
// reconnect gap.
//
// Defect under test (real, in the application's production code path):
// gate0's WebSocket is opened once in useEffect with NO onclose/onerror
// handler and NO reconnect. When the backend dies, the socket dies
// silently; when the backend returns, the actor never resynchronizes —
// its view freezes while the world moves on.
//
// Fault (all real processes/signals, no mocks): the supervisor SIGTERMs
// the backend mid-run, restarts it (in-memory count resets to 0 —
// ledgered backend behavior of this disposable), then a witness actor
// mounts and presses (server 0→1). Detection: the victim — connected
// throughout, never pressed — still shows its stale counter while the
// server has moved on, and its log contains exactly one `ws new`
// (no reconnect attempt ever made).
//
// Needs (env-gated, Skip when absent): HATCH_PROBE_BIN, HATCH_GATE0_BUNDLE,
// HATCH_GATE0_SERVER (path to server.mjs for the restart). The harness is
// expected to SURVIVE the socket death (exit 0); a crash here would be a
// harness robustness defect, reported as failure with the log attached.
package instance

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func gate3env(t *testing.T) (bin, bundle, api, server string) {
	t.Helper()
	bin, bundle, api = gate2env(t)
	server = os.Getenv("HATCH_GATE0_SERVER")
	if server == "" {
		t.Skip("HATCH_GATE0_SERVER required (path to server.mjs)")
	}
	requireFreshCount(t, api)
	return bin, bundle, api, server
}

func killBackend(t *testing.T, api string) {
	t.Helper()
	// Best-effort SIGTERM to the disposable backend (proven pattern); then
	// assert death via the production endpoint, not via process table.
	exec.Command("pkill", "-f", "[g]ate0-backend").Run()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := serverCount(api); err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("backend survived pkill")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func startBackend(t *testing.T, api, server string) {
	t.Helper()
	cmd := exec.Command("node", server)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("restart backend: %v", err)
	}
	// Left running on purpose: subsequent scenarios need a live backend.
	// Its in-memory count resets to 0 on restart (ledgered).
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := serverCount(api); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("restarted backend never came up")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// logVictimTrace persists the victim-side evidence (fiber text lines +
// socket lifecycle lines) into the detection log (go test -v output).
func logVictimTrace(t *testing.T, combined string) {
	t.Helper()
	t.Log("victim trace begin")
	for _, line := range strings.Split(combined, "\n") {
		if strings.Contains(line, "text=") ||
			strings.Contains(line, "ws new") ||
			strings.Contains(line, "ws recv") ||
			strings.Contains(line, "ws flush") ||
			strings.Contains(line, "ws dispatch") ||
			strings.Contains(line, "GATE0_PRESS") ||
			strings.Contains(line, "GATE0_TRANSACTIONS=") ||
			strings.Contains(line, "GATE0_LINGERED") ||
			strings.Contains(line, "fetch GET") ||
			strings.Contains(line, "fetch POST") {
			t.Log("  " + strings.TrimSpace(line))
		}
	}
	t.Log("victim trace end")
}

func TestGate3ReconnectGap(t *testing.T) {
	bin, bundle, api, server := gate3env(t)

	// Victim: mounts at count 0, lingers long past the fault window.
	victim, err := StartActor(Config{
		Engine: bin,
		Bundle: bundle,
		Env:    probeEnv("HATCH_TRACE=1", "HATCH_LINGER=45"),
	})
	if err != nil {
		t.Fatalf("start victim: %v", err)
	}
	// Fixed mount window (measured mount ~2–4s; no determinism claimed).
	time.Sleep(12 * time.Second)

	// FAULT: backend death. The victim's socket dies with it.
	killBackend(t, api)
	time.Sleep(3 * time.Second) // let EOF propagate to the reader thread
	startBackend(t, api, server)

	// Witness: fresh boot post-restart, mounts at reset count 0, presses.
	wit, err := StartActor(Config{
		Engine: bin,
		Bundle: bundle,
		Env:    probeEnv("HATCH_PRESS=1"),
	})
	if err != nil {
		t.Fatalf("start witness: %v", err)
	}
	wres, err := wit.Wait()
	if err != nil {
		t.Fatalf("wait witness: %v", err)
	}
	if wres.ExitCode != 0 {
		t.Fatalf("witness exit = %d", wres.ExitCode)
	}
	if !strings.Contains(wres.Stdout, `text="1"`) {
		t.Fatal("witness did not reach counter \"1\"")
	}
	n, err := serverCount(api)
	if err != nil || n != 1 {
		t.Fatalf("server count = %d, err = %v, want 1", n, err)
	}

	// DETECTION on the victim (reaped after its linger ends).
	vres, err := victim.Wait()
	if err != nil {
		t.Fatalf("wait victim: %v", err)
	}
	// Carry-through (gate-3 verification): persist the victim-side trace
	// in the detection log BEFORE asserting, so the verdict rides on
	// preserved evidence, not assertions alone.
	logVictimTrace(t, vres.Stdout+"\n"+vres.Stderr)
	if vres.ExitCode != 0 {
		t.Fatalf("victim exit = %d — harness must survive socket death (log above is the defect)", vres.ExitCode)
	}
	if c := strings.Count(vres.Stdout+"\n"+vres.Stderr, "ws new"); c != 1 {
		t.Fatalf("victim opened %d sockets, want exactly 1 (no reconnect attempt)", c)
	}
	if !strings.Contains(vres.Stdout, `text="0"`) {
		t.Fatal("victim fiber lacks frozen counter \"0\"")
	}
	t.Logf("gate3: server=%d witness=\"1\" victim frozen at \"0\" with 1 socket — reconnect gap REPRODUCED and DETECTED", n)
}
