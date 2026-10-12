# hatch

A barebones headless simulator for Expo/React Native apps. It executes a
real app's JavaScript behind a faithful native-module compatibility surface,
so scripted multi-actor scenarios (joins, votes, drops, reconnects) can run
N instances deep on weak hardware — no iOS Simulator, no device farm.

Status: Phase 1 (scaffold). Gate 0 PASSED Oct 11 (@agent-auditor, from
primary sources + independent rerun): a purpose-built counter-room app
(expo 55 / RN 0.83.10, 570 modules, no hostile deps) mounts behind the
Fabric C++ host on source-built Hermes 0.14.1 — real entry, committed
mount + observed effects, inspected tree, dispatched press, production
HTTP+WebSocket round-trip against a disposable backend (counter "0"→"1"),
clean teardown, ASan silent. Evidence in /tmp/fabric-spike/ (gate0-run8.txt,
gate0-asan-A/B.txt, SPIKE-LOG.md Gate-0 section); ledger in FIDELITY.md.
hear-me-out (Fabric + reanimated + expo-router store) is the parked STRETCH
case after graduation, priced separately.

Heterogeneous by design: VPS carries bulk headless actors, Mac anchors
device truth, same scenarios over the tailnet. Fidelity anchor = Hermes +
C++ host; mass path = V8 isolates (50 actors ≈ 410 MB measured).

Note: the Go runner in this repo (`cmd/hatch`, `internal/`, `modules/`)
is pre-Gate-0 (node-subprocess era) and is NOT the proven path — the proven
path is the probe binary + backend in /tmp. Phase 1 reworks orchestration
onto it (1.2 two-actor join, 1.3 lifecycle).

```
hatch run --bundle testdata/hello.js
hatch run --bundle <app-bundle.js> --api http://127.0.0.1:18787
```

Everything runs on your machine: the JS engine subprocess, the stub backend,
and the module doubles. There is no cloud dependency.

## Explicit non-goals

- Native Swift/Kotlin emulation is out of scope. Hatch covers the Expo JS
  layer only: JS-callable native-module surfaces (fetch, WebSocket, timers,
  …), never native binaries or UI rendering.
- No iOS/Android fidelity claims beyond what FIDELITY.md measures.
- No cloud infra, no hosted device farm. Local-first is the point.
- No silent mocks: unsupported native operations fail loudly, never
  succeed falsely. No determinism claims beyond what is measured.

## Spike evidence

Phase 0 ran in /tmp/hatch-spike (NOT in this repo, per plan):

- stub-server.mjs — stub backend speaking the worker's real wire format
  (POST /api/rooms, POST /api/rooms/:code/join, GET /api/rooms/:code,
  WS /api/rooms/:code/socket with {"type":"room-invalidated"} + pong).
- headless-client.mjs — app JS using only fetch/WebSocket/timers, replicating
  room-store.ts (createRoom, 25s ping, 80ms debounced refresh on
  invalidation).
- Result: two virtual clients joined one room; the host received the live
  invalidation broadcast and refreshed to see both members.
  `SPIKE PASS: virtual clients visible in a real room round trip`.

Gate-0 evidence ran in /tmp/fabric-spike + /tmp/gate0-app + /tmp/gate0-backend
(purpose-built gate0 app; Expo SDK 55 stack matching the pinned host):

- App: /tmp/gate0-app/App.js — View + Texts + Pressable, useEffect fetch +
  WebSocket, registerRootComponent("main"). Backend: /tmp/gate0-backend/
  server.mjs — HTTP count + WS broadcast room on :18788.
- Host: probe3.cpp (Fabric UIManager + RuntimeScheduler + StubViewTree
  mounting, Hermes 0.14.1) — build/fabric-probe3 (+ build-asan/).
- Runs: gate0-run8.txt (press → tx 3→4 → "0"→"1"), gate0-asan-A/B.txt
  (two actors, ASan silent), gate0-rss.txt (peak RSS). Static module IDs
  via findgate0.py (JSTimers 184, AppRegistry 243).

Engine honesty: Hermes 0.14.1 built from source (facebook/hermes @ RN pin)
because prebuilts ship x86_64-only and this box is aarch64. `hermes` on PATH
is the agent CLI, not the engine.
