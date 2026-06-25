// ScopeSegment — standard / push 2-value segment for the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// ADRs:
//   - 0036 palette-2phase-store-architecture (this is a pure dumb component —
//     no local state, no useEffect; reads usePaletteStore + useDaemonStore,
//     calls setScope on click)
//   - 0047 palette-disabledreason-single-source (disabledReason lives in
//     lib/tools.ts::scopeDisabledReason; this component only renders the
//     returned reason string)
//
// FR coverage: FR-004 (initial scope), FR-005 (push disabled when no active
// session), FR-006 (push disabled when occupant != frame).

import { scopeDisabledReason } from "../../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "../../store/daemon";
import { type PaletteScope, usePaletteStore } from "../../store/palette";
import type { SessionInfo } from "../../wire/server";

// EMPTY_SESSIONS is a module-level reference-stable empty array. ScopeSegment
// only consults activeSessionID / activeOccupant via selectDaemonSnapshot;
// passing `[]` inline would re-allocate per render and (harmlessly) churn the
// snapshot identity. Keeping a single const here also keeps the dep-shape
// audit-able — if a future scope predicate starts reading `sessions`, swap
// this for a real subscription, don't sneak it in via inline literals.
const EMPTY_SESSIONS: SessionInfo[] = [];

// scopeButton renders one tab in the segment. Pulled out so the standard /
// push pair share a single source of truth for disabled / aria / data-active
// wiring — a regression in either button (e.g. forgetting aria-pressed) would
// otherwise have to be caught in two places.
function scopeButton(props: {
  scope: PaletteScope;
  label: string;
  active: boolean;
  reason: string | null;
  onClick: () => void;
}) {
  const { scope, label, active, reason, onClick } = props;
  const disabled = reason !== null;
  return (
    <button
      type="button"
      role="tab"
      disabled={disabled}
      aria-pressed={active}
      data-active={active ? "true" : undefined}
      data-scope={scope}
      className="palette-scope-segment__tab"
      onClick={onClick}
    >
      <span className="palette-scope-segment__label">{label}</span>
      {reason !== null && <span className="palette-scope-segment__sub">{reason}</span>}
    </button>
  );
}

export function ScopeSegment() {
  const scope = usePaletteStore((s) => s.scope);
  const setScope = usePaletteStore((s) => s.setScope);
  // Subscribe to the daemon fields scopeDisabledReason actually consults
  // as PRIMITIVE selectors so this component re-renders when (and only
  // when) those fields change. The composite DaemonSnapshot itself is
  // assembled via selectDaemonSnapshot (Y3 single source) — re-deriving
  // it locally would risk drifting from the rest of the palette
  // (ParamSelectPhase / ToolSelectPhase / CommandPalette all funnel
  // through the same helper). We pass empty defaults for sessions /
  // sessionConfig because scopeDisabledReason only consults activeSessionID
  // and activeOccupant; future signals added to selectDaemonSnapshot would
  // surface here as a structural type error.
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  const daemon = selectDaemonSnapshot({
    sessions: EMPTY_SESSIONS,
    activeSessionID,
    activeOccupant,
    sessionConfig: null,
  });
  const standardReason = scopeDisabledReason("standard", daemon);
  const pushReason = scopeDisabledReason("push", daemon);
  return (
    <div role="tablist" aria-label="palette scope" className="palette-scope-segment">
      {scopeButton({
        scope: "standard",
        label: "standard",
        active: scope === "standard",
        reason: standardReason,
        onClick: () => setScope("standard"),
      })}
      {scopeButton({
        scope: "push",
        label: "push",
        active: scope === "push",
        reason: pushReason,
        onClick: () => setScope("push"),
      })}
    </div>
  );
}
