// ParamSelectPhase — renders the second phase of the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
//   FR-011 縦並び全表示 / FR-012 listbox vs text / FR-013-014 worktree/host
//   toggles / FR-015 conditional toggle visibility / FR-016 Tab vs focus
//   trap / FR-017 Esc back / FR-019 IME / FR-020 submitting disabled
// ADRs:
//   - 0036 palette-2phase-store-architecture (component never touches HTTP;
//     submit goes through store.submit(ctx))
//   - 0038 palette-fuzzy-pure-function (this phase does NOT use fuzzy ranges
//     — listbox shows options in registry order, text inputs are free-form)
//   - 0042 palette-new-session-payload-wire-mirror (sandbox payload is the
//     "host" string on the wire; toggleHost flips paramValues.host bool
//     which the new-session ToolDef projects onto "host" | undefined)
//
// The component is invoked by CommandPalette (a later task) while
// phase==='paramSelect'. Its ToolCtx prop carries the runtime dependencies
// (http / daemon snapshot / notifications / store actions) the submit path
// needs — the component itself stays free of those wires.

import { useMemo } from "react";
import { type DaemonSnapshot, type ToolCtx, type ToolDef, listTools } from "../../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ParamSelectPhaseProps {
  ctx: ToolCtx;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// findSelectedProject locates the SessionConfigProject the user has chosen
// in the `project` field, so the command-field toggle visibility (FR-015)
// can branch on isGit / isSandboxed. Returns null when no project is yet
// selected or the path no longer exists in projects[] (the latter
// indicates a session-config snapshot diff between open and now).
function findSelectedProject(
  snapshot: DaemonSnapshot,
  paramValues: Record<string, unknown>,
): { path: string; isGit: boolean; isSandboxed: boolean } | null {
  const raw = paramValues.project;
  if (typeof raw !== "string" || raw === "") return null;
  return snapshot.projects.find((p) => p.path === raw) ?? null;
}

// isFinalField is the predicate FR-011 / spec point 4 documents for the
// "Enter submits vs Enter advances" branch. Centralized so the two
// listbox / text-input branches stay in sync if params grow.
function isFinalField(params: ReadonlyArray<unknown>, paramCursor: number): boolean {
  return paramCursor === params.length - 1;
}

// optionIndexOf returns the index of `value` in `options`, or -1 when not
// found. Used for ArrowUp/Down navigation within a listbox param: we treat
// the current paramValues[param.id] as the "cursor position" so the store
// holds zero per-param cursor state (paramCursor is field index, not
// option index — keeps the store schema flat).
function optionIndexOf<V>(options: ReadonlyArray<{ value: V }>, value: V | undefined): number {
  if (value === undefined) return -1;
  for (let i = 0; i < options.length; i++) {
    const opt = options[i];
    if (opt !== undefined && opt.value === value) return i;
  }
  return -1;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ParamSelectPhase({ ctx }: ParamSelectPhaseProps): JSX.Element | null {
  const selectedToolId = usePaletteStore((s) => s.selectedToolId);
  const paramValues = usePaletteStore((s) => s.paramValues);
  const paramCursor = usePaletteStore((s) => s.paramCursor);
  const submitting = usePaletteStore((s) => s.submitting);
  const composing = usePaletteStore((s) => s.composing);
  const setParam = usePaletteStore((s) => s.setParam);
  const toggleWorktree = usePaletteStore((s) => s.toggleWorktree);
  const toggleHost = usePaletteStore((s) => s.toggleHost);
  const moveCursor = usePaletteStore((s) => s.moveCursor);
  const submit = usePaletteStore((s) => s.submit);
  const setComposing = usePaletteStore((s) => s.setComposing);

  // Subscribe to the daemon fields selectDaemonSnapshot consults as
  // primitive selectors so React only re-renders when the consumed slice
  // actually changes. The snapshot itself is assembled via the centralized
  // selectDaemonSnapshot helper (Y3 single-source) so a future shape diff
  // (e.g. a new daemon-global gating signal) lands in exactly one place —
  // store/daemon.ts. We call selectDaemonSnapshot inside useMemo against
  // the latest store state, with the primitive deps below ensuring the
  // memo recomputes when (and only when) one of the consumed fields
  // changes — keeping the returned snapshot identity stable downstream
  // (listTools / scopeDisabledReason memoization).
  const sessions = useDaemonStore((s) => s.sessions);
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  const snapshot: DaemonSnapshot = useMemo(
    () => selectDaemonSnapshot({ sessions, activeSessionID, activeOccupant, sessionConfig }),
    [sessions, activeSessionID, activeOccupant, sessionConfig],
  );

  // Resolve the tool fresh every render so a registry diff (push tool
  // disappears, config reloads) does not leave a stale ToolDef captured in
  // a memo. listTools is pure and cheap on standard scope; on push scope
  // the dynamic-push task adds N tools where N is the curated command list
  // size (10s of entries) — still cheap.
  const tool: ToolDef | null = useMemo(() => {
    if (selectedToolId === null) return null;
    return listTools(snapshot, snapshot.pushCommands).find((t) => t.id === selectedToolId) ?? null;
  }, [selectedToolId, snapshot]);

  if (tool === null) {
    // ParamSelectPhase rendered without a selected tool is a bug at the
    // shell level (CommandPalette should render ToolSelectPhase when
    // selectedToolId is null). Render null so the user sees nothing rather
    // than crashing; the bug will surface in the empty palette body.
    return null;
  }

  const params = tool.params ?? [];

  // The selected project drives command-field toggle visibility (FR-015).
  // Computed once per render rather than inside onKeyDown so the rendered
  // affordance ("worktree: ON/OFF (Tab to toggle)") and the actual key
  // handler agree on the same source of truth.
  const selectedProject = findSelectedProject(snapshot, paramValues);
  const showWorktreeToggle = selectedProject?.isGit === true;
  const showHostToggle = selectedProject?.isSandboxed === true;
  const worktreeOn = paramValues.worktree === true;
  const hostOn = paramValues.host === true;

  // advanceOrSubmit is the shared "Enter pressed on a field" reducer per
  // spec point 4. Final field → submit (the store re-validates
  // disabledReason and routes errors); otherwise → move to next field
  // (the focus effect on each input syncs DOM focus with paramCursor).
  function advanceOrSubmit(): void {
    if (composing) return;
    if (isFinalField(params, paramCursor)) {
      void submit(ctx);
      return;
    }
    moveCursor(+1);
  }

  return (
    <form
      className="palette-param-select"
      aria-label="palette parameters"
      onSubmit={(e) => {
        // We handle Enter per-field via onKeyDown (advanceOrSubmit); the
        // form-level onSubmit only fires for stray default-submit cases
        // (e.g. a button with no type="button" that we accidentally
        // promote to submit). Always preventDefault so submit() never
        // gets called twice through different code paths.
        e.preventDefault();
      }}
    >
      {params.map((param, idx) => {
        const isCurrent = idx === paramCursor;
        const value = paramValues[param.id];
        if (param.options !== null) {
          // Listbox-style options (project / sessionId / future enum
          // params). ArrowUp/Down cycle through the registry-ordered list;
          // we never use fuzzy ranges here (ADR-0038), so the visible
          // order matches the ParamDef declaration / daemon snapshot
          // order — the user can predict where their target sits.
          return (
            <ParamListbox
              key={param.id}
              paramId={param.id}
              label={param.label}
              options={param.options}
              value={value}
              focused={isCurrent}
              disabled={submitting}
              composing={composing}
              onSelect={(v: unknown) => setParam(param.id, v)}
              onEnter={advanceOrSubmit}
              onCompositionStart={() => setComposing(true)}
              onCompositionEnd={() => setComposing(false)}
            />
          );
        }
        // Text input (command and any future free-form fields). FR-012
        // says free-form fields accept Enter even with no candidates;
        // that's structurally true here because there's no candidate
        // list to be empty.
        return (
          <ParamTextInput
            key={param.id}
            paramId={param.id}
            label={param.label}
            value={typeof value === "string" ? value : ""}
            focused={isCurrent}
            disabled={submitting}
            composing={composing}
            onChange={(v) => setParam(param.id, v)}
            onEnter={advanceOrSubmit}
            // Only the `command` field gets the worktree / host toggle
            // hijack (FR-013/014/016). Passing the project to the input
            // lets it gate Tab handling on isGit / isSandboxed without
            // the input having to import daemon state itself.
            isCommandField={param.id === "command"}
            commandToggles={
              param.id === "command"
                ? {
                    showWorktreeToggle,
                    showHostToggle,
                    worktreeOn,
                    hostOn,
                    toggleWorktree,
                    toggleHost,
                  }
                : null
            }
            onCompositionStart={() => setComposing(true)}
            onCompositionEnd={() => setComposing(false)}
          />
        );
      })}
    </form>
  );
}

// ---------------------------------------------------------------------------
// Listbox sub-component (param.options !== null)
// ---------------------------------------------------------------------------

interface ParamListboxProps {
  paramId: string;
  label: string;
  options: ReadonlyArray<{ value: unknown; getText: (v: unknown) => string }>;
  value: unknown;
  focused: boolean;
  disabled: boolean;
  composing: boolean;
  onSelect: (v: unknown) => void;
  onEnter: () => void;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
}

function ParamListbox(props: ParamListboxProps): JSX.Element {
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
  // Visible option labels are taken from getText(value) so display
  // formatting (ADR-0033 title→subtitle→id fallback for sessions) never
  // leaks back into paramValues — the value remains the wire-clean id.

  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (composing) return;
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
      // Empty listbox → no valid selection → Enter is dropped. Free-form
      // text inputs (FR-012) accept Enter even with zero candidates; a
      // listbox structurally cannot, since there is nothing to commit.
      if (options.length === 0) return;
      // If nothing selected yet, materialize the first option as the
      // current choice before advancing — the user's intent is "I want
      // the top of the list", same as ToolSelectPhase confirm.
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
          const text = opt.getText(opt.value);
          const selected = i === currentIdx;
          // Key combines the option text + index so reorderings inside a
          // stable param re-key correctly. Using the index alone trips
          // noArrayIndexKey; combining with text gives React identity that
          // tracks the visible option regardless of position.
          const optKey = `${paramId}-${text}-${i}`;
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
                // Mouse selection: prevent the listbox losing focus
                // before we record the click, then commit the value.
                e.preventDefault();
                if (disabled) return;
                onSelect(opt.value);
              }}
            >
              {text}
            </div>
          );
        })}
      </div>
    </fieldset>
  );
}

// ---------------------------------------------------------------------------
// Text input sub-component (param.options === null)
// ---------------------------------------------------------------------------

interface CommandToggles {
  showWorktreeToggle: boolean;
  showHostToggle: boolean;
  worktreeOn: boolean;
  hostOn: boolean;
  toggleWorktree: () => void;
  toggleHost: () => void;
}

interface ParamTextInputProps {
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

function ParamTextInput(props: ParamTextInputProps): JSX.Element {
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
    // FR-019: during IME composition, the component does NOT preventDefault
    // — every key must reach the IME so the composition can resolve.
    if (composing) return;
    // FR-013/014/016: command field hijacks Tab → worktree / Shift+Tab →
    // host. Other fields and the command field when the relevant flag is
    // off let Tab pass through to the focus trap.
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
      // Else: fall through to focus trap (FR-016).
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
          // FR-019: IME composition pre-empts writes here too. The store's
          // setParam is not IME-guarded (it's a generic writer), so we
          // guard at the field level — the only place "this change came
          // from a composition" is knowable.
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
