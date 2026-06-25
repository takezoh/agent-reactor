// ParamListbox — the listbox variant rendered by ParamSelectPhase for
// kind: 'static-options' and kind: 'dynamic-options' (N>=1) ParamDefs.
//
// Split out of ParamSelectPhase.tsx for the file-size budget: keeping the
// orchestrator (ParamSelectPhase) under 500 lines required moving the
// two leaf renderers (this file + ParamTextInput.tsx) to siblings. The
// behavior is unchanged; ParamSelectPhase still owns option
// materialization, empty-state handling, and the form-level submit flow.

import type { JSX } from "react";
import type { ParamOption } from "../../lib/tools";

export interface ParamListboxProps {
  paramId: string;
  label: string;
  options: ReadonlyArray<ParamOption>;
  value: unknown;
  focused: boolean;
  disabled: boolean;
  composing: boolean;
  onSelect: (v: unknown) => void;
  onEnter: () => void;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
}

// optionIndexOf returns the index of `value` in `options`, or -1 when
// not found. Pure helper colocated with the only call site.
function optionIndexOf(options: ReadonlyArray<ParamOption>, value: unknown): number {
  if (value === undefined) return -1;
  for (let i = 0; i < options.length; i++) {
    const opt = options[i];
    if (opt !== undefined && opt.value === value) return i;
  }
  return -1;
}

export function ParamListbox(props: ParamListboxProps): JSX.Element {
  const {
    paramId,
    label,
    options,
    value,
    focused,
    disabled,
    composing,
    onSelect,
    onEnter,
    onCompositionStart,
    onCompositionEnd,
  } = props;

  const currentIdx = optionIndexOf(options, value);

  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    // FR-IME: composing pre-empts every navigation/commit key so the
    // IME can resolve the composition. Both the store flag and the
    // native event are checked — belt-and-suspenders against missed
    // compositionstart events.
    if (composing) return;
    if (e.nativeEvent.isComposing) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (options.length === 0) return;
      const nextIdx = currentIdx + 1 >= options.length ? 0 : currentIdx + 1;
      const nextOpt = options[nextIdx];
      if (nextOpt !== undefined) onSelect(nextOpt.value);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (options.length === 0) return;
      const nextIdx = currentIdx <= 0 ? options.length - 1 : currentIdx - 1;
      const nextOpt = options[nextIdx];
      if (nextOpt !== undefined) onSelect(nextOpt.value);
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      // Empty listbox structurally cannot commit (no candidate). N=0
      // dynamic-options is handled upstream by ParamEmptyState (FR-A4);
      // static-options declaring an empty array still lands here.
      if (options.length === 0) return;
      if (currentIdx < 0) {
        const first = options[0];
        if (first !== undefined) onSelect(first.value);
      }
      onEnter();
      return;
    }
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
      <div
        id={`palette-param-${paramId}`}
        className="palette-param-listbox"
        // biome-ignore lint/a11y/useSemanticElements: ARIA listbox pattern uses div+role=listbox; <select> cannot host aria-activedescendant
        role="listbox"
        tabIndex={disabled ? -1 : 0}
        aria-activedescendant={
          currentIdx >= 0 ? `palette-param-${paramId}-opt-${currentIdx}` : undefined
        }
        aria-disabled={disabled || undefined}
        onKeyDown={onKeyDown}
        onCompositionStart={onCompositionStart}
        onCompositionEnd={onCompositionEnd}
      >
        {options.map((opt, i) => {
          const selected = i === currentIdx;
          const optKey = `${paramId}-${opt.label}-${i}`;
          return (
            // biome-ignore lint/a11y/useFocusableInteractive: focus stays on parent listbox via aria-activedescendant; options are not individually tabbable
            <div
              key={optKey}
              id={`palette-param-${paramId}-opt-${i}`}
              // biome-ignore lint/a11y/useSemanticElements: ARIA listbox uses div+role=option; <option> only works inside <select>
              role="option"
              aria-selected={selected}
              className={`palette-param-option ${selected ? "selected" : ""}`}
              onMouseDown={(e) => {
                e.preventDefault();
                if (disabled) return;
                onSelect(opt.value);
              }}
            >
              {opt.label}
            </div>
          );
        })}
      </div>
    </fieldset>
  );
}
