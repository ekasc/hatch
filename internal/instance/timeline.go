// Joint timelines (2.3): supervisor-side observations with wall-clock
// millis, merged with backend transport-event lines into one report.
//
// Timestamps are observation times on ONE host clock (backend ISO lines
// become UnixMillis for the merge). They order cross-side events; they
// claim no determinism and no cross-machine validity. Client-side fiber
// state is order-only (sequence within an actor log), never timestamped.
package instance

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Event is one timeline row.
type Event struct {
	T      int64  `json:"t"`
	Actor  string `json:"actor"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Timeline collects supervisor-side observations.
type Timeline struct {
	mu     sync.Mutex
	Events []Event
}

// Add records now (wall-clock millis) with an actor identity.
func (tl *Timeline) Add(actor, kind, detail string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.Events = append(tl.Events, Event{
		T:      time.Now().UnixMilli(),
		Actor:  actor,
		Kind:   kind,
		Detail: detail,
	})
}

// WriteJSONL writes events sorted by time (stable) as one JSON object per
// line and returns the path.
func (tl *Timeline) WriteJSONL(path string) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	evs := append([]Event(nil), tl.Events...)
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].T < evs[j].T })
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range evs {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// ParseBackendLog converts BACKEND_LOG lines ("[ISO] kind detail") into
// Events with actor "backend". Unparseable lines are skipped, counted.
func ParseBackendLog(path string) ([]Event, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var evs []Event
	skipped := 0
	for _, line := range splitLines(string(raw)) {
		if len(line) < 24 || line[0] != '[' {
			skipped++
			continue
		}
		end := -1
		for i := 1; i < len(line); i++ {
			if line[i] == ']' {
				end = i
				break
			}
		}
		if end < 0 {
			skipped++
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, line[1:end])
		if err != nil {
			skipped++
			continue
		}
		rest := strings.TrimLeft(line[end+1:], " ")
		kind, detail := rest, ""
		for i := 0; i < len(rest); i++ {
			if rest[i] == ' ' {
				kind, detail = rest[:i], rest[i+1:]
				break
			}
		}
		evs = append(evs, Event{T: ts.UnixMilli(), Actor: "backend", Kind: kind, Detail: detail})
	}
	return evs, skipped, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// MergeTimelines combines supervisor + backend events into time order.
func MergeTimelines(tl *Timeline, backend []Event) []Event {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	all := append(append([]Event(nil), tl.Events...), backend...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].T < all[j].T })
	return all
}
