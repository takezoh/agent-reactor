import { useEffect } from "react";
import { usePaletteStore } from "../store/palette";

// useChipHotkey — document capture-phase listener for Alt+W / Alt+H chip toggles.
// FR-018: Alt+W toggles the worktree chip; Alt+H toggles the host chip.
// FR-023: IME composition suppresses the hotkey (composing guard).
// ADR-0037: capture phase is load-bearing — same rationale as useGlobalHotkey.
// Only active when the palette is open in paramSelect phase with the command
// field and the corresponding chip visible.

export interface ChipHotkeyOptions {
  // Dynamic visibility (FR-018: only activate Alt+W/H when the chip is visible).
  worktreeChipVisible: boolean;
  hostChipVisible: boolean;
  commandFieldVisible: boolean;
}

export function useChipHotkey(opts: ChipHotkeyOptions): void {
  const { worktreeChipVisible, hostChipVisible, commandFieldVisible } = opts;

  useEffect(() => {
    const listener = (e: KeyboardEvent): void => {
      // (1) Palette conditions: open + paramSelect phase + commandField visible.
      const s = usePaletteStore.getState();
      if (!s.open) return;
      if (s.phase !== "paramSelect") return;
      if (!commandFieldVisible) return;
      // (2) Composing guard (FR-023): IME composition takes priority.
      if (s.composing) return;
      // (3) altKey + event.code (physical key — absorbs OS mnemonic differences
      //     such as macOS Option+W = U+2211 SUM; e.key would differ per OS but
      //     e.code is always 'KeyW' for the W physical key regardless of OS).
      if (!e.altKey) return;
      if (e.code === "KeyW" && worktreeChipVisible) {
        // (4) preventDefault to suppress browser / OS Alt mnemonic.
        e.preventDefault();
        // (5) Action call.
        s.toggleWorktree();
        return;
      }
      if (e.code === "KeyH" && hostChipVisible) {
        e.preventDefault();
        s.toggleHost();
        return;
      }
    };
    // capture: true — run before ParamTextInput / browser Alt mnemonic handlers
    // so preventDefault can suppress them (FR-018).
    document.addEventListener("keydown", listener, true);
    return () => document.removeEventListener("keydown", listener, true);
  }, [worktreeChipVisible, hostChipVisible, commandFieldVisible]);
}
