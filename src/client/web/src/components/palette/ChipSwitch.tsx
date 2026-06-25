// ChipSwitch.tsx — role='switch' aria-checked chip for Worktree / Host toggles.
// FR-016 / FR-017 / FR-019 / FR-020 / FR-023 / FR-029 / UAC-008 / UAC-009 / UAC-010
import type { JSX, KeyboardEvent, PointerEvent } from "react";

export interface ChipSwitchProps {
  // hintKey: 'W' (worktree) or 'H' (host) — '[W]' / '[H]' icon hint.
  hintKey: "W" | "H";
  label: string; // 'Worktree' / 'Host (sandbox)' etc.
  checked: boolean; // aria-checked
  onToggle: () => void;
  disabled?: boolean;
  composing: boolean; // FR-023 IME guard
  // testid suffix (data-toggle=worktree|host) — for existing test DOM query compat
  testId?: string;
}

export function ChipSwitch(props: ChipSwitchProps): JSX.Element {
  const { hintKey, label, checked, onToggle, disabled, composing, testId } = props;

  function safeToggle(): void {
    if (disabled) return;
    if (composing) return; // FR-023
    onToggle();
  }

  function onPointerDown(e: PointerEvent<HTMLButtonElement>): void {
    // FR-017: pointerdown + preventDefault prevents stealing focus from the text input.
    e.preventDefault();
    safeToggle();
  }

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>): void {
    if (composing) return;
    if (e.key === " " || e.code === "Space") {
      // FR-019: Space toggles the chip.
      e.preventDefault();
      safeToggle();
      return;
    }
    if (e.key === "Enter") {
      // FR-020: Enter toggles the chip and prevents form submission.
      e.preventDefault();
      safeToggle();
      return;
    }
  }

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      data-toggle={testId}
      data-on={checked ? "on" : "off"}
      className={`palette-chip palette-chip--${checked ? "on" : "off"}`}
      onPointerDown={onPointerDown}
      onKeyDown={onKeyDown}
    >
      <span aria-hidden="true" className="palette-chip__hint">
        [{hintKey}]
      </span>{" "}
      <span className="palette-chip__label">{label}</span>{" "}
      <span aria-hidden="true" className="palette-chip__state">
        {checked ? "ON" : "OFF"}
      </span>
    </button>
  );
}
