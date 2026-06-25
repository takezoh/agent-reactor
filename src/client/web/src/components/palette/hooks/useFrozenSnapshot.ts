// useFrozenSnapshot — captures/releases a FrozenSnapshot on submitting transitions.
//
// FR-012: frozenSnapshotRef captures snapshot when submitting transitions false→true.
// FR-013: frozenSnapshotRef is released when submitting transitions true→false.
// ADR-0055: frozenSnapshot is React render state (useRef), not store state.
// M3: FrozenSnapshot does NOT include chipVisibility.

import { useEffect, useRef } from "react";
import type React from "react";
import { fuzzyRank } from "../../../lib/fuzzy";
import type { DaemonSnapshot } from "../../../lib/tools";
import { listTools } from "../../../lib/tools";
import { usePaletteStore } from "../../../store/palette";
import type { ActiveContextSnapshot } from "../../../store/palette_active_context";
import { type SortedTools, sortToolsForList } from "../../../store/palette_helpers";

// ---------------------------------------------------------------------------
// FrozenSnapshot type (M3: no chipVisibility field)
// ---------------------------------------------------------------------------

export interface FrozenSnapshot {
  activeContext: ActiveContextSnapshot;
  sortedList: SortedTools;
  sortedListCursor: number;
  // flashSeq frozen at the moment submitting transitioned false→true.
  // Passed to ActiveContextHeader so the flash animation is locked and
  // cannot re-trigger from store updates during the frozen window (ADR-0055).
  flashSeq: number;
}

export interface UseFrozenSnapshotResult {
  frozenSnapshotRef: React.RefObject<FrozenSnapshot | null>;
}

/**
 * Captures a FrozenSnapshot when submitting transitions false→true,
 * and releases it when submitting transitions true→false.
 */
export function useFrozenSnapshot(
  submitting: boolean,
  daemon: DaemonSnapshot,
  activeContextSnapshot: ActiveContextSnapshot,
  flashSeq: number,
): UseFrozenSnapshotResult {
  const frozenSnapshotRef = useRef<FrozenSnapshot | null>(null);
  const prevSubmittingRef = useRef(false);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — only submitting drives this effect; daemon/flashSeq/etc are sampled via getState() at run time
  useEffect(() => {
    const wasSubmitting = prevSubmittingRef.current;
    prevSubmittingRef.current = submitting;

    if (!wasSubmitting && submitting) {
      // false→true: capture snapshot
      const all = listTools(daemon, daemon.pushCommands);
      const ranked = fuzzyRank(all, usePaletteStore.getState().query, (t) => t.label);
      const sortedList = sortToolsForList(ranked, daemon);
      const cursor = usePaletteStore.getState().paramCursor;
      const snap: FrozenSnapshot = {
        activeContext: activeContextSnapshot,
        sortedList,
        sortedListCursor: cursor,
        // Capture the current flashSeq so ActiveContextHeader's flash
        // animation cannot re-trigger from store bumps during the frozen
        // window (ADR-0055: frozen side seq must be fixed at capture time).
        flashSeq,
      };
      frozenSnapshotRef.current = snap;
    } else if (wasSubmitting && !submitting) {
      // true→false: release snapshot
      frozenSnapshotRef.current = null;
    }
  }, [submitting]);

  return { frozenSnapshotRef };
}
