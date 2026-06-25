import { useEffect } from "react";
import { isMacPlatform } from "../lib/platform";
import { usePaletteStore } from "../store/palette";

/**
 * useGlobalHotkey — installs a single document-level `keydown` listener that
 * intercepts Cmd+K (macOS) / Ctrl+K (other) on the capture phase and toggles
 * the command palette.
 *
 * Design (ADR-0037 — "Listen for Cmd/Ctrl+K on the document capture phase,
 * and pair it with an always-on header button as a fallback"):
 *
 *   - xterm.js' internal `<textarea>` consumes `keydown` events on the bubble
 *     phase, so a normal `window.addEventListener("keydown", ...)` would never
 *     see Cmd+K while the terminal has focus. Registering on `document` with
 *     `{ capture: true }` runs the handler *before* the textarea, so the
 *     palette wins regardless of which element currently owns focus.
 *
 *   - We call `preventDefault()` + `stopPropagation()` on a matching combo to
 *     suppress browser-native Ctrl+K bindings (e.g. Firefox's search-bar
 *     focus). preventDefault is best-effort — some browsers/OSes still claim
 *     the chord — which is why ADR-0037 mandates a permanent header button
 *     as the fallback (wired in a sibling task).
 *
 *   - When the palette is already open, the hotkey delegates to
 *     `refocusInput()` instead of `openPalette()`. The store treats a
 *     second `openPalette()` as a no-op (FR-029 idempotency), but a user
 *     who hits the chord again typically means "put the caret back in the
 *     search input" — we keep the in-flight phase, query, paramValues
 *     intact and just bump `refocusSeq`.
 *
 * IME note (ADR-0040): IME composition is suppressed inside the palette
 * store via `composing`, not here. Cmd/Ctrl+K is unrelated to the IME's
 * commit-Enter, so this hook does not need to consult `composing`.
 *
 * Lifecycle: `useGlobalHotkey()` is intended to be mounted exactly once at
 * the top of `App.tsx`. The effect's cleanup removes the listener on
 * unmount so HMR / tests reload cleanly.
 */

export function useGlobalHotkey(): void {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Compare on lowercased `key` so Shift+Cmd+K (uppercase "K") also
      // matches — we treat it as the same hotkey rather than a separate
      // combo, since users frequently leave Shift held without intent.
      const k = e.key.toLowerCase();
      if (k !== "k") return;
      const mod = isMacPlatform() ? e.metaKey : e.ctrlKey;
      if (!mod) return;
      // preventDefault first so Firefox's Ctrl+K search-bar focus is
      // suppressed even if our store call throws (defensive ordering).
      e.preventDefault();
      e.stopPropagation();
      const s = usePaletteStore.getState();
      if (s.open) {
        // FR-029: re-pressing the hotkey while open keeps the in-flight
        // phase / paramValues / query intact and just nudges focus.
        s.refocusInput();
      } else {
        // FR-001 / FR-004: open at the default phase, scope chosen by the
        // store from the current daemon snapshot (store reads its own).
        s.openPalette();
      }
    };
    // capture: true is load-bearing — see ADR-0037 / module doc above.
    document.addEventListener("keydown", onKey, { capture: true });
    return () => {
      document.removeEventListener("keydown", onKey, { capture: true });
    };
  }, []);
}
