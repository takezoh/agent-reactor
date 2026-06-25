// ParamSelectPhase — renders the second phase of the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
//   FR-011 vertical stack, show all / FR-012 listbox vs text / FR-013-014 worktree/host
//   toggles / FR-015 conditional toggle visibility / FR-016 Tab vs focus
//   trap / FR-017 Esc back / FR-019 IME / FR-020 submitting disabled
// Spec: docs/specs/2026-06-24-web-ui-fixes (palette-bugfix)
//   FR-A1  dynamic-options listbox with leading-option preset
//   FR-A4  dynamic-options empty -> ParamEmptyState + suppress later
//          params + submit unreachable
//   FR-Det preselect-direct and toolSelect-then-confirm entries land on
//          identical DOM
// ADRs:
//   - 0036 palette-2phase-store-architecture (component never touches HTTP;
//     submit goes through store.submit(ctx))
//   - 0038 palette-fuzzy-pure-function (this phase does NOT use fuzzy
//     ranges — listbox shows options in registry order)
//   - 0042 palette-new-session-payload-wire-mirror (host toggle → wire
//     "sandbox": "host")
//   - 0049 dynamic-options-materialize-at-view (display layer projects
//     materializeKey → ParamOption[]; store / wire stay daemon-free)

import { useMemo } from "react";
import {
  type DaemonSnapshot,
  type ParamDef,
  type ParamOption,
  type ToolCtx,
  type ToolDef,
  listTools,
  projectOptions,
} from "../../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import { ParamEmptyState } from "./ParamEmptyState";
import { ParamListbox } from "./ParamListbox";
import { ParamTextInput } from "./ParamTextInput";
import { useDynamicParamPreset } from "./useDynamicParamPreset";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ParamSelectPhaseProps {
  ctx: ToolCtx;
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

// materializeOptions projects a ParamDef + DaemonSnapshot to the concrete
// listbox option list (or null when the param has no listbox at all).
// Kept a free function so the wire/store layer stays daemon-free
// (ADR-0049): the union just declares 'materializeKey: "projects"' and
// the view layer owns the lookup.
//
//   'text'             -> null     (caller renders a text input)
//   'static-options'   -> options  (baked into the ToolDef)
//   'dynamic-options'  -> projected from snapshot via materializeKey
//
// The two `never` defaults below force the type-checker to surface any
// future ParamDef / materializeKey widening that forgets a branch here.
export function materializeOptions(
  param: ParamDef,
  snapshot: DaemonSnapshot,
): ParamOption[] | null {
  switch (param.kind) {
    case "text":
      return null;
    case "static-options":
      return param.options;
    case "dynamic-options": {
      switch (param.materializeKey) {
        case "projects":
          return projectOptions(snapshot);
        default: {
          const _exhaustive: never = param.materializeKey;
          return _exhaustive;
        }
      }
    }
    default: {
      const _exhaustive: never = param;
      return _exhaustive;
    }
  }
}

// findSelectedProject locates the SessionConfigProject the user has
// chosen in the `project` field, so the command-field toggle visibility
// (FR-015) can branch on isGit / isSandboxed. Returns null when no
// project is selected or the path no longer exists (snapshot diff
// between open and now).
function findSelectedProject(
  snapshot: DaemonSnapshot,
  paramValues: Record<string, unknown>,
): { path: string; isGit: boolean; isSandboxed: boolean } | null {
  const raw = paramValues.project;
  if (typeof raw !== "string" || raw === "") return null;
  return snapshot.projects.find((p) => p.path === raw) ?? null;
}

// isFinalField is the predicate FR-011 / spec point 4 documents for the
// "Enter submits vs Enter advances" branch.
function isFinalField(params: ReadonlyArray<unknown>, paramCursor: number): boolean {
  return paramCursor === params.length - 1;
}

// sentinelParam is filler used when the active tool declares fewer than
// the maximum slot count, so the fixed-arity useDynamicParamPreset hook
// calls keep their order/key invariants across renders. Never rendered
// (params.slice clips it) and useDynamicParamPreset early-returns for
// kind !== 'dynamic-options', so setParam is never called against it.
const sentinelParam: ParamDef = {
  id: "__sentinel__",
  kind: "text",
  label: "",
};

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

  // Subscribe to daemon primitives so React re-renders only when a
  // consumed field changes; the snapshot is reassembled by
  // selectDaemonSnapshot (Y3 single-source).
  const sessions = useDaemonStore((s) => s.sessions);
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  const snapshot: DaemonSnapshot = useMemo(
    () => selectDaemonSnapshot({ sessions, activeSessionID, activeOccupant, sessionConfig }),
    [sessions, activeSessionID, activeOccupant, sessionConfig],
  );

  // Resolve the tool fresh every render so a registry diff does not
  // leave a stale ToolDef captured in a memo.
  const tool: ToolDef | null = useMemo(() => {
    if (selectedToolId === null) return null;
    return listTools(snapshot, snapshot.pushCommands).find((t) => t.id === selectedToolId) ?? null;
  }, [selectedToolId, snapshot]);

  // params is stable across re-renders of the same tool so downstream
  // useMemo / useEffect deps don't churn on identity.
  const params = useMemo<ParamDef[]>(() => tool?.params ?? [], [tool]);

  // The selected project drives command-field toggle visibility (FR-015).
  const selectedProject = findSelectedProject(snapshot, paramValues);
  const showWorktreeToggle = selectedProject?.isGit === true;
  const showHostToggle = selectedProject?.isSandboxed === true;
  const worktreeOn = paramValues.worktree === true;
  const hostOn = paramValues.host === true;

  // Today's tools declare at most 2 params (new-session: project +
  // command). To keep useDynamicParamPreset's hook ordering stable
  // across renders we materialize both potential slots up-front and
  // pass them unconditionally. A tool with fewer params falls back to
  // sentinelParam so the hook count is constant. A future N>2 tool
  // would warrant promoting this to an array-driven custom hook.
  const slot0 = params[0] ?? sentinelParam;
  const slot1 = params[1] ?? sentinelParam;
  const options0 = useMemo(() => materializeOptions(slot0, snapshot), [slot0, snapshot]);
  const options1 = useMemo(() => materializeOptions(slot1, snapshot), [slot1, snapshot]);
  useDynamicParamPreset(slot0, options0, paramValues[slot0.id], setParam);
  useDynamicParamPreset(slot1, options1, paramValues[slot1.id], setParam);
  const materialized: Array<ParamOption[] | null> = [options0, options1];

  if (tool === null) {
    // Bug at the shell level (CommandPalette should render
    // ToolSelectPhase when selectedToolId is null). Render null so the
    // user sees nothing rather than crashing.
    return null;
  }

  // advanceOrSubmit: Enter on a field. Final field → submit (store
  // re-validates disabledReason and routes errors); otherwise →
  // moveCursor(+1).
  function advanceOrSubmit(): void {
    if (composing) return;
    if (isFinalField(params, paramCursor)) {
      void submit(ctx);
      return;
    }
    moveCursor(+1);
  }

  // FR-A4: a dynamic-options param with zero materialized options
  // renders the empty-state body AND suppresses every subsequent param
  // + the submit-trigger Enter handler the final field would carry.
  let stopIdx = params.length;
  for (let i = 0; i < params.length; i++) {
    const p = params[i];
    if (p === undefined) continue;
    const opts = materialized[i] ?? null;
    if (p.kind === "dynamic-options" && opts !== null && opts.length === 0) {
      stopIdx = i + 1;
      break;
    }
  }

  return (
    <form
      className="palette-param-select"
      aria-label="palette parameters"
      onSubmit={(e) => {
        // Enter is handled per-field via onKeyDown (advanceOrSubmit);
        // form-level onSubmit only fires for stray default-submits.
        e.preventDefault();
      }}
    >
      {params.slice(0, stopIdx).map((param, idx) => {
        const isCurrent = idx === paramCursor;
        const value = paramValues[param.id];
        const options = materialized[idx] ?? null;

        // FR-A4: dynamic-options with zero materialized options renders
        // the empty-state body. ParamEmptyState carries no key handlers
        // so Esc still bubbles to the palette shell.
        if (param.kind === "dynamic-options" && options !== null && options.length === 0) {
          return (
            <fieldset
              key={param.id}
              className={`palette-param ${isCurrent ? "focused" : ""}`}
              data-param-id={param.id}
              aria-label={param.label}
            >
              <div className="palette-param-label">{param.label}</div>
              <ParamEmptyState message="No projects available - add a project first" />
            </fieldset>
          );
        }

        if (options !== null) {
          // Listbox (static-options / dynamic-options with N>=1).
          return (
            <ParamListbox
              key={param.id}
              paramId={param.id}
              label={param.label}
              options={options}
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
        // Free-form text input (param.kind === 'text').
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
