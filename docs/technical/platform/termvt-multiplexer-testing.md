# termvt multiplexer: fan-out isolation test harness

`platform/termvt` is the tmux-free multiplexer primitive: it runs a command in a
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
  one. Prevented by single-writer fan-out: `fanout` (holding `mu`) does a
  non-blocking send per subscriber and closes any whose buffer is full, so one
  slow consumer can neither stall the read loop nor steal another's stream.

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
| `fanout_contract_test.go` | the isolation contract: multi-subscriber delivery, `Manager` cross-talk, slow-subscriber containment, control-before-output ordering. |
| `session_test.go` | wired single-session behaviour against a real pty: input echo, OSC 9/133/title capture, reattach-snapshot-first, resize, exit-on-close, default size, slow-subscriber sever. |
| `manager_test.go` | wired multi-session lifecycle: create/get/list, duplicate-id rejection, remove closing subscribers. |

`waitFor` / `assertNoOutput` are the shared observation helpers;
markers travel as ordinary pty output (`cat` echoes a written marker), so
"which subscriber received it" is read straight off the event channel.

## Running

```sh
# regression guards + fuzz seed corpus (the ci job's test step)
cd src && TMPDIR=/tmp go test ./platform/termvt/

# concurrency check — guards the single-writer fan-out under concurrent
# subscribe/drain (the slow-vs-fast case races fan-out against a draining reader)
cd src && go test -race ./platform/termvt/
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
