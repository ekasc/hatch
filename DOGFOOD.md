# Dogfood: the reconnect gap (found via hatch, confirmed on hardware)

## The bug (real app code, gate0 counter room)

The app opens its WebSocket once in `useEffect` with no `onclose`/`onerror`
handler and no reconnect. When the backend dies, the socket dies silently;
when the backend returns, the actor never resynchronizes — its view freezes
while the world moves on. Worse, the press path fails silently too
(`fetch(...).catch(() => {})` swallows a dead-backend POST with zero user
feedback), so the user can "vote" into the void.

## How hatch found it

Gate 3 (2.1, `TestGate3ReconnectGap`): supervisor SIGTERMed the backend
mid-run, restarted it, pressed from a witness. The victim — connected
throughout, never pressed — reaped clean with exactly one `ws new` in its
log and its counter frozen while the server had moved on. The harness
surviving the socket death (exit 0) was itself part of the proof: a crash
there would have been a harness defect instead.

## Device confirmation (2.2, Ekas's iPhone, Expo Go SDK 57)

S3 verbatim: "nothing happens. same screen. no reload or anything." No
banner, no crash, no OS kill — the iPhone froze exactly like the lab
victim. S5 (reload → server value) confirmed recovery-via-resync on
hardware, matching the lab's restart recovery.

## What the sim missed

1. The S4 coincidence: lab-victim froze at "0" vs server 1 (crisp); on the
   phone the frozen "1" vs server 1 overlapped by construction (soft).
   Lesson: design fault scenarios so stale and current values cannot
   coincide (kill → witness-press to N+1, never back to a previously
   displayed value).
2. Real-touch feel: the harness dispatches onPress synthetically (FIDELITY
   row 7). It cannot feel a dead button (no haptic, no visual press state
   change on a frozen UI) — the device run showed the frozen screen gives
   no affordance feedback at all, which a screenshot-diffing harness would
   also miss without pixel comparison (explicitly out of scope).
3. Reload semantics: Expo Go reload ≠ process restart (module cache,
   DevTools hooks persist). Both converged here, but a state-leak class of
   bugs could hide in that difference — flagged, not covered.
4. Timing: lab pumps are quiescence-driven (fast-forward); the phone ran
   wall-clock. A race with a <100ms window could reproduce on hardware and
   never in the lab. No such race observed in these scenarios.

## What the sim did not miss

The defect itself, its shape (silent, no feedback), the recovery path
(resync on fresh boot), and the server-side sequence (exactly one
increment, broadcast received by the living). The 10-minute phone run
confirmed rather than corrected the lab.
