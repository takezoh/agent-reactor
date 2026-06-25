// ParamTextInput — the text-input variant rendered by ParamSelectPhase
// for kind: 'text' ParamDefs. Includes the command-field Tab / Shift+Tab
// hijack for the worktree / sandbox=host toggles (FR-013/014/016).
//
// Split out of ParamSelectPhase.tsx alongside ParamListbox to keep the
// orchestrator file under the 500-line ceiling. Behavior unchanged.

import type { JSX } from "react";

export interface CommandToggles {
  showWorktreeToggle: boolean;
  showHostToggle: boolean;
  worktreeOn: boolean;
  hostOn: boolean;
  toggleWorktree: () => void;
  toggleHost: () => void;
}

export interface ParamTextInputProps {
  paramId: string;
  label: string;
  value: string;
  focused: boolean;
  disabled: boolean;
  composing: boolean;
  onChange: (v: string) => void;
  onEnter: () => void;
  isCommandField: boolean;
  commandToggles: CommandToggles | null;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
}

export function ParamTextInput(props: ParamTextInputProps): JSX.Element {
  const {
    paramId,
    label,
    value,
    focused,
    disabled,
    composing,
    onChange,
    onEnter,
    isCommandField,
    commandToggles,
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
    // FR-013/014/016: command field hijacks Tab → worktree, Shift+Tab →
    // host. Other fields and the command field when the relevant flag
    // is off let Tab pass through to the focus trap.
    if (isCommandField && commandToggles !== null && e.key === "Tab") {
      if (!e.shiftKey && commandToggles.showWorktreeToggle) {
        e.preventDefault();
        commandToggles.toggleWorktree();
        return;
      }
      if (e.shiftKey && commandToggles.showHostToggle) {
        e.preventDefault();
        commandToggles.toggleHost();
        return;
      }
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      onEnter();
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
      <input
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
      {isCommandField && commandToggles !== null && (
        <div className="palette-param-command-toggles" aria-label="command toggles">
          {commandToggles.showWorktreeToggle && (
            <span
              className="palette-param-toggle"
              data-toggle="worktree"
              data-on={commandToggles.worktreeOn ? "on" : "off"}
            >
              worktree: {commandToggles.worktreeOn ? "ON" : "OFF"} (Tab to toggle)
            </span>
          )}
          {commandToggles.showHostToggle && (
            <span
              className="palette-param-toggle"
              data-toggle="host"
              data-on={commandToggles.hostOn ? "on" : "off"}
            >
              sandbox=host: {commandToggles.hostOn ? "ON" : "OFF"} (Shift+Tab to toggle)
            </span>
          )}
        </div>
      )}
    </fieldset>
  );
}
