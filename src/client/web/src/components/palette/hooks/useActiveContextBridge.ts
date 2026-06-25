// useActiveContextBridge — syncs active context snapshot to the palette store.
//
// FR-009 / ADR-0058: derive active context from daemon state and push to store
// so ActiveContextHeader can subscribe from store.
// Suppressed while submitting=true to preserve the frozen snapshot invariant (ADR-0055).

import { useEffect } from "react";
import type { SessionConfigSlice } from "../../../store/daemon";
import {
  type ActiveContextSnapshot,
  deriveActiveContext,
} from "../../../store/palette_active_context";
import type { SessionInfo } from "../../../wire/server";

export function useActiveContextBridge(
  submitting: boolean,
  activeSessionID: string | null,
  sessions: SessionInfo[],
  sessionConfig: SessionConfigSlice | null,
  setActiveContextSnapshot: (snap: ActiveContextSnapshot) => void,
): void {
  useEffect(() => {
    if (submitting) return;
    const snap = deriveActiveContext(
      activeSessionID,
      sessions,
      // ADR-0058: empty projects → 'unknown' kind, surfaced via
      // sessionConfigMissing warning when palette is opened. Consistent
      // with selectDaemonSnapshot's []-fallback (Y3 single source).
      sessionConfig?.projects ?? [],
    );
    setActiveContextSnapshot(snap);
  }, [sessions, activeSessionID, sessionConfig, submitting, setActiveContextSnapshot]);
}
