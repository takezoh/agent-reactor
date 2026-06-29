// useFontSize — the in-memory fontSize state + persistence + refit fan-out
// (ADR 0070 / 0034 / 0077, FR-MOB-PERSIST-001/002 / FR-MOB-FONT-CLAMP-001 /
// FR-MOB-STEPPER-001).
//
// ADR 0077 supersedes the original pinch write path: only the FontSizeControl
// stepper (+/-/reset) and the initial localStorage restore now write fontSize.
// `applyPinch` / `beginPinch` are removed as dead code; the persist + clamp
// contract is unchanged.
//
// Two write paths converge here and all go through one `set`:
//   - +/-/Reset       : increase()/decrease()/reset() from FontSizeControl.
//   - restore on boot : usePersistedValue reads `web.term.fontSize`.
//
// Every mutation clamps to [8,28] (FR-MOB-FONT-CLAMP-001 — below 8px the font
// is unreadable and `fit()` can emit NaN cols), persists through the adapter,
// and invokes the ADR-0034 rAF-coalesced `scheduleFit` exactly once so the grid
// re-flows. Keeping `scheduleFit` un-guarded (even when the value is unchanged,
// e.g. Reset while already 14) preserves the invariant that every activate re-fits.

import { useCallback, useRef } from "react";
import { type StorageLike, usePersistedValue } from "./usePersistedValue";

/** localStorage key — device-scoped, independent of theme (ADR 0059 / 0070). */
export const FONT_SIZE_KEY = "web.term.fontSize";
export const FONT_SIZE_MIN = 8;
export const FONT_SIZE_MAX = 28;
export const FONT_SIZE_DEFAULT = 14;
/** +/- stepper increment (FR-MOB-STEPPER-001). */
export const FONT_SIZE_STEP = 2;

/** Clamp to the legible [8,28] window (FR-MOB-FONT-CLAMP-001). */
export function clampFontSize(px: number): number {
  return Math.min(FONT_SIZE_MAX, Math.max(FONT_SIZE_MIN, px));
}

export interface UseFontSizeOptions {
  /** ADR-0034 rAF-coalesced refit, invoked once per mutation. */
  scheduleFit: () => void;
  /** Injected storage (tests pass an in-memory Map adapter; prod = localStorage). */
  storage?: StorageLike | null;
}

export interface UseFontSizeApi {
  /** Current fontSize in px, always within [8,28]. */
  fontSize: number;
  /** Set an absolute px value; clamps, persists, refits. */
  set: (px: number) => void;
  /** +2px (FR-MOB-STEPPER-001). */
  increase: () => void;
  /** -2px (FR-MOB-STEPPER-001). */
  decrease: () => void;
  /** Back to a default (14px by default); persists + refits. */
  reset: (px?: number) => void;
}

export function useFontSize(options: UseFontSizeOptions): UseFontSizeApi {
  const { scheduleFit, storage } = options;

  const [fontSize, persist] = usePersistedValue<number>({
    key: FONT_SIZE_KEY,
    parse: (raw) => Number.parseInt(raw, 10),
    serialize: (px) => String(px),
    fallback: FONT_SIZE_DEFAULT,
    // Split contract (ADR 0070): NaN (parse fail) → reject → fallback;
    // parsed-but-out-of-range → clamp into [8,28]. '999' → 28, '' / 'foo' → 14.
    validate: (px) => (Number.isFinite(px) ? clampFontSize(px) : null),
    storage,
  });

  // Mirror the latest fontSize into a ref so stepper math reads a synchronous value.
  const fontSizeRef = useRef(fontSize);
  fontSizeRef.current = fontSize;

  const scheduleFitRef = useRef(scheduleFit);
  scheduleFitRef.current = scheduleFit;

  const set = useCallback(
    (px: number): void => {
      const clamped = clampFontSize(Math.round(px));
      persist(clamped);
      scheduleFitRef.current();
    },
    [persist],
  );

  const increase = useCallback(() => set(fontSizeRef.current + FONT_SIZE_STEP), [set]);
  const decrease = useCallback(() => set(fontSizeRef.current - FONT_SIZE_STEP), [set]);
  const reset = useCallback((px: number = FONT_SIZE_DEFAULT) => set(px), [set]);

  return { fontSize, set, increase, decrease, reset };
}
