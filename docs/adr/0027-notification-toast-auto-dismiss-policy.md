# ADR 0027 — NotificationToast auto-dismiss after 5s, stacked via LRU

Status: Accepted

## Context

`EvtAgentNotification` (OSC 9 / 99 / 777) fires when an agent wants the
user's attention — typically "task complete" or "ready for input".
Drivers fire these freely, so the UX must tolerate bursts (several in a
row) without consuming the whole viewport.

Two design choices:

1. **Auto-dismiss timer**: each toast disappears after `T` seconds.
2. **Stack vs replace**: simultaneous notifications either stack
   vertically (LRU max N) or replace the previous one.

The α reducer's `Subscribers.Surface` LRU of 32 entries (per-conn) is
already in place; the notifications store already implements a 32-entry
LRU in A1-β. Reusing both concepts here means the toast UI is a thin
view over the existing store.

## Decision

- Each toast auto-dismisses **5 seconds** after first render.
- A user click dismisses immediately.
- Toasts stack vertically (top-right corner of the viewport), newest
  on top.
- The on-screen stack is capped at **3 visible toasts**; older entries
  in the notifications store remain queryable but are not rendered as
  toasts.
- The notifications store retains the full LRU 32 entries (β behavior
  preserved); the on-screen toast view is a derived slice (last 3
  unseen entries).

Implementation hints:

- `NotificationToast` reads `notifications.unseen` (sorted by ts desc,
  sliced to 3) from the store.
- `useEffect` sets a 5-second `setTimeout` per toast to call the store's
  `dismiss(id)` action.
- Click handler calls `dismiss(id)` directly.

## Consequences

- The UX absorbs bursts gracefully (max 3 visible) without dropping
  history (full LRU 32 in store).
- Auto-dismiss timing is hard-coded; future per-cmd customization
  (e.g. Cmd 777 lingers longer) is a follow-up PR.
- 5 seconds is a reasonable default for "notice me" UX; too short feels
  rushed, too long feels noisy.
- The "3 visible" cap is arbitrary; tunable via a constant.

## Alternatives

- **No auto-dismiss** — user must click each one; rejected, fails the
  "tolerate bursts" requirement.
- **Replace (1 visible)** — loses information; rejected.
- **Unbounded stack** — fills the viewport; rejected.

## Related requirements

- FR-δ03
