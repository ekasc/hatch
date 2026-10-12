# FIDELITY.md

Everywhere the sim diverges from real devices. Updated in the same commit
as any behavior change. Nothing here is a TODO in disguise: each row states
what the sim does today and what a real device does instead.

Categories: **Real** (protocol over real transport) / **Simulated** (works,
needs device comparison) / **Unsupported** (no claim — fails loudly).

Provenance: Gate 0 (Oct 11, @agent-auditor PASS) on the purpose-built
gate0 counter-room app (expo 55 / RN 0.83.10, 570 modules) behind the
Fabric C++ host + source-built Hermes 0.14.1. Evidence: /tmp/fabric-spike/
gate0-run8.txt (press run), gate0-asan-A/B.txt (two-actor ASan), SPIKE-LOG.md
Gate-0 section. hear-me-out (Fabric + reanimated + expo-router store) is the
parked STRETCH case, not the Gate-0 claim.

## Real

| # | Area | Sim behavior | Real device behavior | Notes |
|---|------|--------------|----------------------|-------|
| 1 | JS engine | Source-built Hermes 0.14.1 (RN-pinned) executes the Metro bundle. | Production runs Hermes bytecode. | Same engine family; source-eval vs bytecode is the residual gap. |
| 2 | Renderer commit | Upstream Fabric UIManager + RuntimeScheduler; JS reconciler commits, StubViewTree applies mutations (Gate 0: 3–4 tx, 31–35 mutations). | Same Fabric pipeline + GPU mounting. | Tree structure, testIDs, Yoga frames asserted headless. |
| 3 | HTTP | Host fetch client over real TCP (http://). | Same HTTP semantics. | Pinned non-configurable: RN's bundle otherwise overwrites global.fetch with XHR-backed whatwg-fetch (diagnosed via String(fetch)=[bytecode]; log proof in run8: `fetch GET/POST … -> 200`). Real client underneath. |
| 4 | WebSocket | Real RFC-6455 client as the global (ws://). | Same protocol. | Installed behind a constructible JS wrapper: Hermes 0.14.1 host functions are not constructible and app code does `new WebSocket` (threw before the wrapper). Real socket underneath. |
| 5 | Effects | useEffect runs after committed mount (fetch + WS connect observed in state). | Same lifecycle. | Pre-press fiber dump / GET-value trace kept in run logs (carry-through 1). |
| 6 | Native modules | Real upstream TUs where implemented (Microtasks, DOM, TurboModule registry); unimplemented ops throw loudly, never silent-mock. | Full native surface. | expo-constants verified live (app-name "gate0"). |

## Simulated

| # | Area | Sim behavior | Real device behavior | Impact on results |
|---|------|--------------|----------------------|-------------------|
| 7 | Press | Synthetic dispatch: the exact registered onPress closure is invoked; no touch hit-testing exists headless. | Touch pipeline (hit-test, responder arbitration) → onPress. | Handler + network + reconciler path fully real; touch targeting itself untested. |
| 8 | Text | Paragraph/RawText mount via fallback descriptor (structure + content asserted through fiber HostText). | Real text measurement/layout. | Text content assertions valid; layout-sensitive text bugs invisible. |
| 9 | Timers | Single-thread DueCalls queue + wall-clock pump (50ms cadence, quiescence-capped). | Platform timer arbitration. | Ordering valid; timing-sensitive races are scripted, not real. |
| 10 | Mount variance | Mount-only runs settle at tx=3 with 31–35 mutations / 13–14 nodes depending on GET-vs-WS arrival order (batching). | Same race exists on device with different interleavings. | Final trees equivalent; counts are not asserted exactly. |

## Unsupported

| # | Area | Sim behavior | Real device behavior |
|---|------|--------------|----------------------|
| 11 | Reanimated / worklets | UI runtime not linked; first Animated mount needs a Fabric shadow node the paper host never has (priced kill-condition). | Reanimated 4 Fabric-only animations. |
| 12 | expo-router store | Navigation init stalls (navReady=false; location arrives at ExpoRoot but does not propagate). | OS intent → initial URL → routes. |
| 13 | Other native modules | Anything outside the ladder (screens, gesture-handler, camera, haptics, secure-store, sqlite, …) fails loudly at the TurboModule proxy. | Full Expo Go surface. |
| 14 | Transport | Localhost TCP only (fetch http://, WS ws://); no radio, NAT, or OS backgrounding. | Real networks. |

## Resource ledger

| Run | Peak RSS | Wall | Result |
|-----|----------|------|--------|
| Gate-0 press (non-ASan) | 35,064 kB (34 MB) | 2.34 s wall | run8/rss: tx 3→4, counter "0"→"1", exit 0 |
| ASan actor A (press) | — | — | exit 0, zero reports |
| ASan actor B (mount-only) | — | — | exit 0, zero reports, converged to "1" via backend read |
| V8 isolate density (prior) | 8–12 MB/actor (50 actors ≈ 410 MB) | 2038 ms / 50 actors | 0 failures |
| 3.1 scale-6 (Hermes+Fabric) | 34.7 MB/actor mean (203 MB total) | 27.1 s batch wall | 0 mount failures; tx spread 3–6 (peer-join broadcast crosstalk) |
| 3.1 scale-20 (Hermes+Fabric) | 34.6 MB/actor mean (677 MB total) | 29.1 s batch wall | 0 mount failures; tx spread 3–5 |
| 3.1 scale-50 (Hermes+Fabric) | 34.4 MB/actor mean (1680 MB total) | 35.4 s batch wall | 0 mount failures; tx spread 3–23 (N-way join broadcasts, all processed) |

RSS methodology (precise): per-actor numbers are summed `/usr/bin/time -v`
Maximum-RSS values — each process's lifetime peak, added up. This OVERSTATES
true marginal cost: shared mapped pages (system libs, Hermes/folly .so
mappings) are counted once per process instead of once. Directionally fine
for 8GB budgeting (the overstatement is conservative), but it is an upper
bound, not a measured total. Scale tooling committed as tools/scale.sh +
tools/scalesum.py (box paths baked in: probe binary, bundle, env).

Sequencing note (closed by 1.2): the Gate-0 "two actors" ran near-sequentially
(B converged via backend read). True concurrency was demonstrated in 1.2
(gate12-A/B.txt): B lingered 30s with no press, observed A's broadcast
mid-run (tx 3→6, counter "0"→"1"), A observed peers:2 live.

## Device comparison (iPhone Expo Go, 2026-10-11, Ekas's thumbs)

Device profile: physical iPhone, Expo Go SDK 57, Metro prod bundle
(dev=false, minify, served over tailnet at exp://100.117.193.79:8081),
backend on 0.0.0.0:18788 reached via tailnet IP. Harness counterpart: SDK55
static export (dist3) behind probe3 — the SDK55-vs-57 delta (RN 0.83→0.86,
Hermes 0.14-build→0.17, index vs deep imports) is an explicit confound on
every row below; app-logic findings transfer, timing/layout diffs do not
without a same-artifact rerun (Path 2, priced separately).

| Step | Device observed (verbatim) | Harness counterpart | Verdict |
|------|---------------------------|---------------------|---------|
| S1 mount | "0", "+1", "peers: 1", backend line = tailnet URL | counter "0", peers "peers:1", base 127.0.0.1 | MATCH — mount + both effects live on hardware; URL line proves the hostUri/getDevServer override works on device |
| S2 press | tap +1 → "1"; server exactly 1 | synthetic dispatch → tx 3→4, "0"→"1", server 1 | MATCH on outcome; INTERACTION diff: real touch pipeline (device) vs synthetic onPress call-through (harness, FIDELITY row 7) — same downstream path from the handler on |
| S3 backend-kill | "nothing happens. same screen. no reload or anything." | victim frozen, 1 socket, exit 0 | MATCH — silent freeze reproduced on hardware: no banner, no crash, no OS kill; socket death invisible at the JS layer on both |
| S4 no-reload read | (lost — reloaded instead of reading) | crisp lab proof (frozen "0" vs server 1) | SOFT — "same screen" through reload implies continuity (a reconnect-reset to "0" would have shown), but the clean pre-reload read never happened; crisp proof stays lab-grade |
| S5 reload | reload → "1" (server=1) | gate-2 restart → fiber "1" (server=1) | MATCH — recovery-via-resync on hardware; mechanisms differ (Expo Go reload vs process restart) but both ride production GET + WS-hello paths |

Carry-throughs from the run: AppRegistry named-only export (my import bug,
fixed + audited); centered status lines (presentation-only, dist6);
SDK57 upgrade + deep-import dodge + blockList ledger (SPIKE-LOG.md).
