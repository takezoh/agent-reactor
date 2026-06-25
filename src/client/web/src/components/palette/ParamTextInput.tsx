// ParamTextInput — the text-input variant rendered by ParamSelectPhase
// for kind: 'text' ParamDefs.
//
// ADR-0053: Tab / Shift+Tab hijack for worktree / sandbox=host toggles
// has been removed. Tab now performs natural focus traversal. Toggle
// affordances are rendered by ChipSwitch (pointer/Space/Enter) with
// useChipHotkey (Alt+W/H) — wired in ParamSelectPhase, not here.
//
// Split out of ParamSelectPhase.tsx alongside ParamListbox to keep the
// orchestrator file under the 500-line ceiling.

import { forwardRef } from "react";
import type { JSX } from "react";

export interface ParamTextInputProps {
  paramId: string;
  label: string;
  value: string;
  focused: boolean;
  disabled: boolean;
  composing: boolean;
  onChange: (v: string) => void;
  onEnter: () => void;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
}

export const ParamTextInput = forwardRef<HTMLInputElement, ParamTextInputProps>(
  function ParamTextInput(props, ref): JSX.Element {
    const {
      paramId,
      label,
      value,
      focused,
      disabled,
      composing,
      onChange,
      onEnter,
      onCompositionStart,
      onCompositionEnd,
    } = props;

    const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
      // FR-019 / FR-IME: during IME composition the component does NOT
      // preventDefault — every key must reach the IME so the composition
      // can resolve. Both the store flag (composing) and the native
      // event (nativeEvent.isComposing) are checked.
      if (composing) return;
      if (e.nativeEvent.isComposing) return;
      if (e.key === "Enter") {
        e.preventDefault();
        onEnter();
      }
      // Tab performs natural focus traversal (ADR-0053).
    };

    return (
      <fieldset
        className={`palette-param ${focused ? "focused" : ""}`}
        data-param-id={paramId}
        aria-label={label}
      >
        <label className="palette-param-label" htmlFor={`palette-param-${paramId}`}>
          {label}
        </label>
        <input
          ref={ref}
          id={`palette-param-${paramId}`}
          className="palette-param-input"
          type="text"
          value={value}
          disabled={disabled}
          onChange={(e) => {
            if (composing) return;
            onChange(e.target.value);
          }}
          onKeyDown={onKeyDown}
          onCompositionStart={onCompositionStart}
          onCompositionEnd={onCompositionEnd}
        />
      </fieldset>
    );
  },
);
