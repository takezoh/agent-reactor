// useCoachmarkOnce — first-run-only Coachmark gate with idempotent persistence
// (ADR 0072, FR-MOB-COACH-001/002).
//
// On the first view-mode entry the terminal shows a one-line dismissible hint next
// to the KeyboardFAB. ADR 0072 nails two contracts that this hook owns:
//
//   1. Idempotent write (FR-MOB-COACH-001): the localStorage key
//      `web.term.hintSeen` is written to `'1'` AT FIRST RENDER, not on
//      dismiss. Writing on dismiss leaves a window where a crash / navigation
//      before the user taps re-shows the hint forever (the "coachmark keeps reappearing" bug).
//      Persisting up-front makes "seen once" mechanically true even if the user
//      leaves immediately.
//   2. Dismiss = tap OR 5s, whichever is first (FR-MOB-COACH-002): the 5s auto
//      timer lives here; `dismiss()` (tap) clears it early. After either, the
//      Coachmark is gone and never returns this session or any later one.
//
// Persistence flows through `usePersistedValue<boolean>` (ADR 0070), the single
// localStorage adapter, so private-mode throws degrade to in-memory just like
// fontSize. The `storage` adapter is injected for deterministic tests.

import { useCallback, useEffect, useRef, useState } from "react";
import { type StorageLike, usePersistedValue } from "./usePersistedValue";

/** Device-scoped key marking the coachmark as already shown (ADR 0072). */
export const HINT_SEEN_KEY = "web.term.hintSeen";
/** Auto-dismiss delay; the tap path can end the coachmark sooner. */
export const COACHMARK_AUTO_DISMISS_MS = 5000;

export interface UseCoachmarkOnceOptions {
  /**
   * True while the mobile gate + view mode are active. The first time this is true
   * with an unseen hint the coachmark shows and `hintSeen='1'` is written.
   */
  active: boolean;
  /** Injected storage (tests pass an in-memory Map; prod = localStorage). */
  storage?: StorageLike | null;
}

export interface UseCoachmarkOnceApi {
  /** True while the one-shot coachmark should be rendered. */
  showCoachmark: boolean;
  /** Tap handler — dismisses immediately (cancels the 5s auto timer). */
  dismiss: () => void;
}

/**
 * useCoachmarkOnce decides whether the first-run coachmark renders, writes the
 * `hintSeen` flag up-front, and arms the tap-or-5s auto dismiss.
 */
export function useCoachmarkOnce(options: UseCoachmarkOnceOptions): UseCoachmarkOnceApi {
  const { active, storage } = options;

  const [seen, setSeen] = usePersistedValue<boolean>({
    key: HINT_SEEN_KEY,
    parse: (raw) => raw === "1",
    serialize: (v) => (v ? "1" : "0"),
    fallback: false,
    // Any successfully parsed boolean is accepted as-is (no clamp dimension).
    validate: (v) => v,
    storage,
  });

  const [visible, setVisible] = useState(false);
  // Latches so the first-render write + show happens exactly once per mount even
  // as `seen` flips to true synchronously after the idempotent write.
  const firedRef = useRef(false);

  // FR-MOB-COACH-001: first active+unseen render → show AND persist immediately.
  useEffect(() => {
    if (!active || seen || firedRef.current) return;
    firedRef.current = true;
    setVisible(true);
    setSeen(true);
  }, [active, seen, setSeen]);

  // FR-MOB-COACH-002: 5s auto-dismiss (tap path clears it early via setVisible).
  useEffect(() => {
    if (!visible) return;
    const timer = setTimeout(() => setVisible(false), COACHMARK_AUTO_DISMISS_MS);
    return () => clearTimeout(timer);
  }, [visible]);

  const dismiss = useCallback(() => setVisible(false), []);

  return { showCoachmark: visible, dismiss };
}
