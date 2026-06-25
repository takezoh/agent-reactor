// ToolSelectPhase — the phase='toolSelect' renderer of the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// FR:
//   - FR-007 (listbox + activedescendant), FR-008 (fuzzy + <mark>),
//     FR-009 (↑/↓/Ctrl+P/N/Enter), FR-019 (IME suppression),
//     FR-020 (submitting disable)
// ADRs:
//   - 0036 palette-2phase-store-architecture (this component is a thin
//     container: it reads state, renders, and translates DOM events into
//     store actions; no HTTP, no DOM imperative work beyond preventDefault)
//   - 0038 palette-fuzzy-pure-function (FuzzyRange consumption is local to
//     this component; ParamSelectPhase does not call fuzzyRank)
//   - 0040 palette-ime-suppression-in-store (composing flag lives in the
//     store; this component just listens for compositionstart/end and
//     guards preventDefault on key events so the IME can consume Enter /
//     arrows for its own composition commit / candidate navigation)
//
// Cursor model: usePaletteStore.paramCursor is an unbounded integer (the
// store has no list-length awareness per ADR-0036). This component clamps
// the cursor on read to [0, ranked.length - 1] so it never points off the
// listbox. moveCursor(±1) still increments the unbounded counter; the
// clamp catches over-scrolls without wrapping (FR-009 explicitly says
// TUI-compatible clamp, not wrap).
//
// The ctx assembled here (http / daemon / notify / store) is the same
// shape PaletteActions.submit expects — confirmTool forwards it to submit
// for the paramless-tool fast path (FR-010).

import type { KeyboardEvent } from "react";
import { useMemo } from "react";
import { makeSessionsApi } from "../../api/sessions";
import { type FuzzyRange, fuzzyRank } from "../../lib/fuzzy";
import { type DaemonSnapshot, type ToolCtx, listTools } from "../../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "../../store/daemon";
import { useNotificationsStore } from "../../store/notifications";
import { usePaletteStore } from "../../store/palette";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderWithRanges splits `text` into plain spans and <mark> spans according
// to half-open `ranges` produced by fuzzyRank. Ranges are pre-sorted by start
// (fuzzy merges adjacent matches into runs) and non-overlapping, so a single
// forward pass is sufficient. The keys are derived from range start indices
// — stable across renders because ranges from fuzzy.ts are deterministic
// given (text, query).
function renderWithRanges(text: string, ranges: ReadonlyArray<FuzzyRange>) {
  const out: Array<string | JSX.Element> = [];
  let i = 0;
  for (const [s, e] of ranges) {
    if (i < s) out.push(text.slice(i, s));
    out.push(
      <mark key={`m-${s}-${e}`} data-testid="palette-mark">
        {text.slice(s, e)}
      </mark>,
    );
    i = e;
  }
  if (i < text.length) out.push(text.slice(i));
  return out;
}

// clamp keeps the cursor within [0, max] inclusive. We use this instead of
// modulo because FR-009 mandates clamp-not-wrap (TUI parity).
function clamp(n: number, max: number): number {
  if (max < 0) return 0;
  if (n < 0) return 0;
  if (n > max) return max;
  return n;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export interface ToolSelectPhaseProps {
  // inputRef and httpFactory are exposed for the CommandPalette shell and
  // tests respectively. inputRef lets the shell call .focus() in response to
  // store.refocusSeq increments without ToolSelectPhase knowing about the
  // shell. httpFactory swaps the SessionsApi for hermetic tests; production
  // callers omit it and get makeSessionsApi() at first render.
  inputRef?: React.RefObject<HTMLInputElement>;
  httpFactory?: () => ToolCtx["http"];
}

export function ToolSelectPhase(props: ToolSelectPhaseProps = {}) {
  // State-that-renders is subscribed via selector so React re-renders on
  // change. Actions (setQuery / moveCursor / confirmTool / setComposing) are
  // read off getState() inside the handlers so the handler always sees the
  // latest action reference — important for tests that swap actions via
  // setState after render, and harmless in production where actions are
  // stable refs.
  const query = usePaletteStore((s) => s.query);
  const scope = usePaletteStore((s) => s.scope);
  const paramCursor = usePaletteStore((s) => s.paramCursor);
  const composing = usePaletteStore((s) => s.composing);
  const submitting = usePaletteStore((s) => s.submitting);

  // Pull the underlying daemon fields with stable identity selectors so
  // React only re-renders when one of them actually changes. We then
  // project to DaemonSnapshot via the centralized selectDaemonSnapshot
  // helper (Y3 single-source) so the projection logic lives in exactly
  // one place — store/daemon.ts. useMemo dep keys are the primitive /
  // array-identity inputs, NOT the derived snapshot; the call inside
  // reads getState() so the resulting snapshot picks up every field
  // consulted by the helper without us having to enumerate them here.
  // (Using useSyncExternalStore with a getSnapshot that returns a fresh
  // object each call would trigger React 18's tearing guard, manifesting
  // as "Should not already be working".)
  const sessions = useDaemonStore((s) => s.sessions);
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  const daemon = useMemo<DaemonSnapshot>(
    () => selectDaemonSnapshot({ sessions, activeSessionID, activeOccupant, sessionConfig }),
    [sessions, activeSessionID, activeOccupant, sessionConfig],
  );

  // Build the ToolCtx once per render. httpFactory is captured so tests can
  // inject a fake SessionsApi without prop-drilling through CommandPalette.
  const ctx = useMemo<ToolCtx>(() => {
    const http = props.httpFactory ? props.httpFactory() : makeSessionsApi();
    const paletteState = usePaletteStore.getState();
    return {
      http,
      daemon,
      // FR-021 mirror of CommandPalette's ctx assembly: ToolSelectPhase's
      // paramless-tool fast path (FR-010) also routes through ToolDef.submit
      // and may write to daemon (e.g. a future paramless new-session-like
      // tool). Keep the two ctx constructions symmetrical so daemonActions
      // are always present regardless of which phase triggered submit.
      daemonActions: {
        selectSession(id) {
          useDaemonStore.getState().selectSession(id);
        },
      },
      notify: {
        success(m) {
          useNotificationsStore.getState().add({ level: "info", message: m });
        },
        error(m) {
          useNotificationsStore.getState().add({ level: "error", message: m });
        },
      },
      store: {
        close: paletteState.close,
      },
    };
  }, [daemon, props.httpFactory]);

  // tools filtered by scope, then fuzzy-ranked by query. fuzzy.ts treats
  // empty query as "passthrough with score 0 and empty ranges", so we don't
  // need to special-case query="" here.
  const ranked = useMemo(() => {
    const all = listTools(daemon, daemon.pushCommands);
    const inScope = all.filter((t) => t.scope === scope);
    return fuzzyRank(inScope, query, (t) => t.label);
  }, [daemon, scope, query]);

  const maxIdx = ranked.length - 1;
  const cursor = clamp(paramCursor, maxIdx);

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    // FR-019: while IME composition is active, every key (Enter, arrows,
    // Ctrl+N/P) belongs to the IME — we MUST NOT preventDefault, MUST NOT
    // moveCursor / confirmTool. The store also guards these actions (ADR-
    // 0040), so even if a stray event slipped through it would no-op; this
    // local short-circuit keeps the DOM behavior (preventDefault) correct.
    if (composing) return;
    const k = e.key;
    const lower = k.toLowerCase();
    const actions = usePaletteStore.getState();
    // Arrow / Ctrl+N — move down
    if (k === "ArrowDown" || (e.ctrlKey && lower === "n")) {
      e.preventDefault();
      actions.moveCursor(+1);
      return;
    }
    // Arrow / Ctrl+P — move up
    if (k === "ArrowUp" || (e.ctrlKey && lower === "p")) {
      e.preventDefault();
      actions.moveCursor(-1);
      return;
    }
    if (k === "Enter") {
      e.preventDefault();
      const selected = ranked[cursor]?.item;
      if (!selected) return;
      // confirmTool(id, ctx) — for paramless tools confirmTool will await
      // submit() internally (FR-010); for tools with params it transitions
      // to phase='paramSelect' synchronously.
      void actions.confirmTool(selected.id, ctx);
      return;
    }
  }

  return (
    <div
      className="palette-tool-select"
      data-phase="toolSelect"
      data-submitting={submitting ? "true" : "false"}
    >
      <input
        ref={props.inputRef}
        type="text"
        role="combobox"
        aria-controls="palette-listbox"
        aria-expanded="true"
        aria-autocomplete="list"
        aria-activedescendant={ranked.length > 0 ? `palette-opt-${cursor}` : undefined}
        value={query}
        onChange={(e) => usePaletteStore.getState().setQuery(e.currentTarget.value)}
        onCompositionStart={() => usePaletteStore.getState().setComposing(true)}
        onCompositionEnd={() => usePaletteStore.getState().setComposing(false)}
        onKeyDown={onKeyDown}
        readOnly={submitting}
        placeholder="Search commands..."
        data-testid="palette-input"
      />
      {/*
        ARIA combobox + listbox pattern (FR-007): the combobox <input> above
        owns focus via aria-activedescendant pointing at one role="option".
        We use <div> for the listbox/option containers rather than ul/li so
        biome's a11y rules don't false-positive on the "non-interactive ul
        with interactive role" pattern — the WAI-ARIA combobox shape allows
        either element, and the option elements are NOT individually
        focusable; the combobox input retains focus and signals selection
        via activedescendant.
      */}
      <div
        id="palette-listbox"
        // biome-ignore lint/a11y/useSemanticElements: <select> cannot host a free-text combobox filter (FR-008); listbox role is the WAI-ARIA canonical fit
        role="listbox"
        // tabIndex={-1} keeps the listbox programmatically focusable (so the
        // biome a11y rules don't flag aria-activedescendant + non-focusable)
        // without putting it in the tab order — the combobox <input> retains
        // focus per the WAI-ARIA combobox/listbox pattern.
        tabIndex={-1}
        aria-disabled={submitting ? true : undefined}
        aria-activedescendant={ranked.length > 0 ? `palette-opt-${cursor}` : undefined}
        data-testid="palette-listbox"
      >
        {ranked.map((hit, i) => {
          const label = hit.item.label;
          return (
            <div
              key={hit.item.id}
              id={`palette-opt-${i}`}
              // biome-ignore lint/a11y/useSemanticElements: <option> is only valid inside <select>; combobox/listbox pattern uses role="option" on generic elements (FR-007)
              role="option"
              // tabIndex={-1} matches the listbox: options are managed-focus,
              // not in the tab order.
              tabIndex={-1}
              aria-selected={i === cursor}
              data-tool-id={hit.item.id}
              data-cursor={i === cursor ? "true" : "false"}
            >
              {renderWithRanges(label, hit.ranges)}
            </div>
          );
        })}
      </div>
    </div>
  );
}
