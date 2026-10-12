// Joint-timeline report test (2.3): supervisor observations + backend
// transport events merged into one time-ordered artifact.
//
// Scenario: B mounts first and lingers (observer); A boots later and
// presses (server 0→1) mid-B-linger, then is killed. The report must show,
// from BOTH sides, the causal chain: B-connect → A-press → server POST →
// B-converge, with actor identities throughout.
//
// Needs (env-gated, Skip when absent): HATCH_PROBE_BIN, HATCH_GATE0_BUNDLE,
// HATCH_BACKEND_LOG (backend running WITH BACKEND_LOG pointing at it).
// The backend log must be fresh — the test asserts a listen line newer
// than its own start (stale logs from older runs fail loudly instead of
// merging silently).
package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var txRe = regexp.MustCompile(`GATE0_TRANSACTIONS=(\d+)`)

func actorTx(output string) int {
	m := txRe.FindStringSubmatch(output)
	if m == nil {
		return -1
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func TestJointTimeline(t *testing.T) {
	bin, bundle, api := gate2env(t)
	requireFreshCount(t, api)
	blog := os.Getenv("HATCH_BACKEND_LOG")
	if blog == "" {
		t.Skip("HATCH_BACKEND_LOG required (backend transport-event log)")
	}
	tl := &Timeline{}
	start := time.Now().UnixMilli()
	tl.Add("director", "scenario-start", "B-observe then A-press-then-kill")

	// B first: mounts at 0, lingers across A's whole run.
	b, err := StartActor(Config{Engine: bin, Bundle: bundle, Env: probeEnv("HATCH_FIBER=1", "HATCH_LINGER=30")})
	if err != nil {
		t.Fatalf("start B: %v", err)
	}
	tl.Add("B", "start", "mount-only linger=30")
	time.Sleep(8 * time.Second) // B settled (fixed wait, stated)

	// A: boots, presses 0→1 mid-B-linger.
	a, err := StartActor(Config{Engine: bin, Bundle: bundle, Env: probeEnv("HATCH_PRESS=1", "HATCH_LINGER=30")})
	if err != nil {
		t.Fatalf("start A: %v", err)
	}
	tl.Add("A", "start", "press=1 linger=30")
	waitServerCount(t, api, 1, 60*time.Second)
	tl.Add("director", "server-observed", "count=1")

	if err := a.Kill(); err != nil {
		t.Fatalf("kill A: %v", err)
	}
	tl.Add("A", "killed", "SIGKILL post-press")
	ares, err := a.Wait()
	if err != nil {
		t.Fatalf("wait A: %v", err)
	}
	tl.Add("A", "reaped", fmt.Sprintf("exit=%d", ares.ExitCode))
	if ares.ExitCode == 0 {
		t.Fatal("A exit = 0, want non-zero (killed)")
	}

	bres, err := b.Wait()
	if err != nil {
		t.Fatalf("wait B: %v", err)
	}
	tl.Add("B", "reaped", fmt.Sprintf("exit=%d tx=%d", bres.ExitCode, actorTx(bres.Stdout)))
	if bres.ExitCode != 0 {
		t.Fatalf("B exit = %d", bres.ExitCode)
	}
	if !strings.Contains(bres.Stdout, `text="1"`) {
		t.Fatal("B fiber lacks converged \"1\"")
	}
	if tx := actorTx(bres.Stdout); tx < 4 {
		t.Fatalf("B tx = %d, want >= 4 (mid-run broadcast commit)", tx)
	}

	// Merge with backend transport events; backend log must be fresh.
	bevs, skipped, err := ParseBackendLog(blog)
	if err != nil {
		t.Fatalf("read backend log: %v", err)
	}
	fresh := false
	// Freshness window: the backend must have (re)started within the last
	// 10 minutes — it is SUPPOSED to predate the test (it must already be
	// up for the precondition), but a log from an older incarnation must
	// never merge silently. Window, not forensics: stated, generous.
	for _, e := range bevs {
		if e.Kind == "lifecycle" && e.T >= start-600000 {
			fresh = true
		}
	}
	if !fresh {
		var first, last int64
		for i, e := range bevs {
			if i == 0 {
				first = e.T
			}
			last = e.T
		}
		t.Fatalf("backend log stale: %d events [%d..%d], test start %d — restart backend with BACKEND_LOG", len(bevs), first, last, start)
	}
	merged := MergeTimelines(tl, bevs)

	dir := os.Getenv("HATCH_ARTIFACTS_DIR")
	if dir == "" {
		dir = "/tmp/fabric-spike"
	}
	out := filepath.Join(dir, fmt.Sprintf("timeline-%d.jsonl", start))
	var tl2 Timeline
	tl2.Events = merged
	if err := tl2.WriteJSONL(out); err != nil {
		t.Fatalf("write timeline: %v", err)
	}
	for _, e := range merged {
		t.Logf("  t=%d %s %s %s", e.T, e.Actor, e.Kind, e.Detail)
	}

	// Causal assertions across sides: server POST precedes the
	// supervisor's count-flip observation; both actors present.
	var postT, flipT int64 = -1, -1
	actors := map[string]bool{}
	for _, e := range merged {
		actors[e.Actor] = true
		if e.Actor == "backend" && e.Kind == "http" && strings.Contains(e.Detail, "POST /increment") {
			postT = e.T
		}
		if e.Actor == "director" && e.Kind == "server-observed" {
			flipT = e.T
		}
	}
	if postT < 0 {
		t.Fatal("no server POST in timeline")
	}
	if !(postT <= flipT) {
		t.Fatalf("causality: POST t=%d after flip-observed t=%d", postT, flipT)
	}
	for _, want := range []string{"A", "B", "backend", "director"} {
		if !actors[want] {
			t.Fatalf("timeline lacks actor %q", want)
		}
	}
	t.Logf("timeline: %d events (%d backend skipped-unparseable), artifact %s", len(merged), skipped, out)
}
