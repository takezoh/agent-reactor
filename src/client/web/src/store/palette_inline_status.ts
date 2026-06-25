// palette_inline_status.ts — InlineStatus slice for usePaletteStore.
//
// Provides transient user-feedback messages that appear inline in the palette
// (e.g. "save is unavailable: No active session") and auto-clear after 4 s.
//
// ADRs:
//   - 0036 palette-2phase-store-architecture: no document/window/HTMLElement.
//     setTimeout return value (NodeJS.Timeout / number) is a Web API global —
//     not a DOM reference — so it is permitted.
//   - 0040 palette-ime-suppression-in-store: emitDisabledFeedback is a no-op
//     while composing=true (FR-023).
//
// FR refs: FR-005 / FR-023 / FR-031

import type { StateCreator } from "zustand";

export type InlineStatusKind = "warning" | "info";

export interface InlineStatusSliceState {
  inlineStatus: {
    message: string; // empty string means no visible status
    kind: InlineStatusKind;
    seq: number; // monotonic; incremented on every emit, even same-message re-emits (FR-031)
    timerId: ReturnType<typeof setTimeout> | null;
  };
}

export interface InlineStatusSliceActions {
  emitDisabledFeedback(toolLabel: string, reason: string): void;
  // clearInlineStatus is exported for tests and for the internal 4 s timer callback.
  clearInlineStatus(): void;
}

export type InlineStatusSlice = InlineStatusSliceState & InlineStatusSliceActions;

export const initialInlineStatusState: InlineStatusSliceState = {
  inlineStatus: {
    message: "",
    kind: "warning",
    seq: 0,
    timerId: null,
  },
};

// INLINE_STATUS_AUTO_CLEAR_MS is a named export so tests can import and fast-
// forward the fake timer to exactly this boundary.
export const INLINE_STATUS_AUTO_CLEAR_MS = 4000;

export const createInlineStatusSlice: StateCreator<
  // The full store shape includes `composing` from the palette base state;
  // we declare the dependency here so TypeScript enforces the composition root
  // provides it via `create<PaletteState & ... & InlineStatusSlice>()`.
  InlineStatusSlice & { composing: boolean },
  [],
  [],
  InlineStatusSlice
> = (set, get) => ({
  ...initialInlineStatusState,

  emitDisabledFeedback(toolLabel, reason) {
    // FR-023: composing=true means the user is mid-IME; do not interrupt with
    // a feedback message that would shift layout or trigger screen-reader
    // announce during composition.
    if (get().composing) return;

    const prev = get().inlineStatus;
    // Cancel any in-flight auto-clear timer so the new message gets its own
    // full 4 s window (FR-031 re-announce: even the same message resets the
    // timer and increments seq).
    if (prev.timerId !== null) clearTimeout(prev.timerId);

    const timerId = setTimeout(() => {
      // Auto-clear: blank the message but keep seq so the screen-reader
      // seq-watch only fires on new emits, not on natural expiry.
      const cur = get().inlineStatus;
      set({
        inlineStatus: { message: "", kind: cur.kind, seq: cur.seq, timerId: null },
      });
    }, INLINE_STATUS_AUTO_CLEAR_MS);

    set({
      inlineStatus: {
        message: `"${toolLabel}" is unavailable: ${reason}`,
        kind: "warning",
        seq: prev.seq + 1,
        timerId,
      },
    });
  },

  clearInlineStatus() {
    const prev = get().inlineStatus;
    if (prev.timerId !== null) clearTimeout(prev.timerId);
    set({
      inlineStatus: { message: "", kind: prev.kind, seq: prev.seq, timerId: null },
    });
  },
});
