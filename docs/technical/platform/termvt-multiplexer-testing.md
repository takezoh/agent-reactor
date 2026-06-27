# termvt multiplexer: fan-out isolation test harness

`platform/termvt` is the PTY multiplexer primitive: it runs a command in a
pty, parses the output through a server-side VT emulator (OSC handling + reattach
snapshots), and fans typed `Event`s out to any number of subscribers. Its one
safety-critical property is **fan-out isolation** — the termvt analogue of the
stream subsystem's [routing isolation](../client/stream-backend-testing.md).
Rationale: [ADR 0003](../../adr/0003-termvt-fanout-isolation.md).

## The invariant

> Every `Event` a `Session` produces reaches **exactly** the live subscribers of
> that session — all of them, in order, control-before-output within a chunk —
> and **no** subscriber of any other session. A subscriber that cannot keep up is
> **severed** (its channel closed), never silently dropped and never allowed to
> block or corrupt the others.

Two cross-talk shapes this rules out:

- **Manager cross-talk** — one session's bytes surfacing in another session's
  terminal. Prevented structurally: each `Session` owns its own subscriber set;
  the `Manager` only routes by exact id.
- **Back-pressure cross-talk** — a slow client wedging or corrupting a healthy
  one. Prevented by single-writer fan-out: `fanout` runs inside the Session's
  sole-owner `mainLoop` and does a non-blocking send per subscriber, closing
  any whose buffer is full, so one slow consumer can neither stall the read
  loop nor steal another's stream.

The single-writer discipline is structural rather than mutex-based: `Session`
is an actor (one `mainLoop` goroutine owns the emulator, subscriber map,
pending control buffer, and dimensions; public methods reach them via `cmdCh`
RPC, all routed through a single `call[R]` helper that pins the shutdown
branch). `ExitCode` is the one exception — it reads `atomic.Bool` +
`atomic.Int32` directly so the runtime's per-tick poll cannot freeze the IPC
under a slow chunk parse. See [ADR 0028](../../adr/0028-termvt-session-actor-model.md)
for the rationale and the deadlock that drove the refactor.

## Why there is no in-process fake (and no opt-in e2e tier)

The stream subsystem multiplexes over a codex **app-server**, so its harness
needs an in-process fake plus an [opt-in real-server backstop](../client/stream-backend-e2e.md)
([ADR 0002](../../adr/0002-optin-appserver-e2e-validates-fakes.md)) to prove the
fake is faithful. termvt has no such fake: its only backend is a **real pty**,
which is always available in CI. The contract therefore drives a real pty
directly — the wired and fidelity layers coalesce, so there is nothing to gate
behind a build tag.

## Files

| File | Role |
|---|---|
| `session.go` | actor public API: `Session` struct, `NewSession` (real pty) + `NewSessionWithDeps` (fake-injection seam), `Subscribe`/`Unsubscribe`/`Resize`/`Snapshot`/`Size`/`ExitCode`/`Close`, plus the generic `call[R]` RPC helper. |
| `session_actor.go` | actor internals: `mainLoop`, `readerLoop`, `responseLoop`, command types (`subscribeCmd` etc.), `fanout`, `registerOSC`, `handleExit`, `processChunk`. |
| `session_deps.go` | `Emulator` and `PTY` interfaces + the production wrappers (`realEmulator`, `realPTY`). The interfaces are the test seam. |
| `fanout_contract_test.go` | the isolation contract: multi-subscriber delivery, `Manager` cross-talk, slow-subscriber containment, control-before-output ordering. |
| `session_test.go` | wired single-session behaviour against a real pty: input echo, OSC 9/133/title capture, reattach-snapshot-first, resize, exit-on-close, default size, slow-subscriber sever, and the CSI-Report-Mode deadlock regression (`TestSessionExitCodeNeverBlocksDuringCSIReportMode`). |
| `session_actor_test.go` | actor-shape tests against a fake emulator + fake pty: chunk-vs-RPC ordering, post-shutdown Subscribe contract, lock-free ExitCode latency, unique non-zero subscriber ids. |
| `manager_test.go` | wired multi-session lifecycle: create/get/list, duplicate-id rejection, remove closing subscribers. |

`waitFor` / `assertNoOutput` are the shared observation helpers;
markers travel as ordinary pty output (`cat` echoes a written marker), so
"which subscriber received it" is read straight off the event channel.

## Running

```sh
# regression guards + fuzz seed corpus (the ci job's test step)
cd src && TMPDIR=/tmp go test ./platform/termvt/

# concurrency check — guards the single-writer fan-out under concurrent
# subscribe/drain. Use the project-level target so the audited subtree stays
# in one place; see docs/agent/testing.md.
make test-race
```

The untrusted client→server decode for the web gateway is fuzzed separately in
`server/web` (`FuzzInbound`): arbitrary client frames must never panic the reader
and must never drive the pty to a non-positive size. See
[.github/workflows/ci.yml](../../../.github/workflows/ci.yml)'s `fuzz` job.

## Invariant ↔ pinning tests

| Behaviour | Pinned by |
|---|---|
| Every live subscriber of a session receives its output | `TestFanoutDeliversToEverySubscriber` |
| Sessions never cross-talk (A's bytes never reach B's subscriber) | `TestManagerSessionsDoNotCrossTalk` |
| A slow subscriber is severed without starving a fast one | `TestSlowSubscriberDoesNotStarveFast`, `TestSessionDisconnectsSlowSubscriber` |
| Control events precede the raw output of the same chunk | `TestControlPrecedesOutputInChunk` |
| Reattach delivers a snapshot first, atomically with live writes | `TestSessionReattachSnapshotFirst` |
| OSC 9 / 133 / title captured as Control, not raw bytes | `TestSessionCapturesOSC9`, `…OSC133Prompt`, `…Title` |
| Process exit fans out EventExit then closes channels | `TestSessionEmitsExitOnClose`, `TestManagerRemove` |
| Resize dimensions are floored/capped (no uint16 overflow or OOM grid) | `TestNormalizeSizeClamp` |
| Malformed or out-of-range client frames can't panic or mis-resize | `server/web` `TestApplyInbound`, `FuzzInbound` |
| CSI Report Mode (DECRQM) reply cannot deadlock `ExitCode` | `TestSessionExitCodeNeverBlocksDuringCSIReportMode` |
| `Subscribe`/`Resize`/`Snapshot`/`Size` complete in deterministic order vs chunks | `TestActor_SubscribeReceivesSnapshotThenChunk` |
| `ExitCode` stays lock-free even while `mainLoop` is parked in a slow chunk | `TestActor_ExitCodeNeverGoesThroughMainLoop` |
| Subscriber ids never collide with the post-shutdown sentinel | `TestActor_SubscribeIDsAreUniqueAndNonZero` |
| Post-shutdown `Subscribe` returns a closed channel, no goroutine leak | `TestActor_SubscribeAfterShutdownReturnsClosedChannel` |
