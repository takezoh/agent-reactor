// ToolSelectPhase — the phase='toolSelect' renderer of the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// FR:
//   - FR-001 (enabled rows above separator), FR-002 (disabled rows below),
//     FR-003 (disabled inline reason), FR-004 (disabled skip),
//     FR-005 / FR-030 (disabled Enter → emitDisabledFeedback),
//   - FR-006 (pointermove enabled → cursor), FR-007 (pointermove disabled → no cursor),
//     FR-008 (mouseleave → cursor unchanged),
//   - FR-007 (listbox + activedescendant), FR-008 (fuzzy + <mark>),
//     FR-009 (↑/↓/Ctrl+P/N/Enter), FR-019 (IME suppression),
//     FR-020 (submitting disable), FR-026 (hover = cursor anchor),
//     FR-029 (color not sole indicator)
// ADRs:
//   - 0036 palette-2phase-store-architecture (thin container: reads state,
//     renders, translates DOM events into store actions; no HTTP, no DOM
//     imperative work beyond preventDefault and per-row flash refs)
//   - 0038 palette-fuzzy-pure-function (FuzzyRange consumption is local)
//   - 0040 palette-ime-suppression-in-store (composing flag in store)
//   - 0047 palette-disabledreason-single-source (disabledReason single source)
//   - 0049 english-ui (all user-visible text is English ASCII)
//   - 0055 submit-freeze-lift-state (frozenList/frozenCursor props bypass
//     internal selector when provided by CommandPalette)
//
// Cursor model: paramCursor in the store is an unbounded integer; this component
// maps it to a logical index in the separator-free sorted list. ArrowDown/Up skip
// disabled rows — the skip delta is computed here (store has no list awareness,
// ADR-0036). moveCursor(delta) receives the pre-computed skip delta. setSelectedToolId
// is updated via usePaletteStore.setState on hover so the single cursor source
// (FR-026) is always selectedToolId.

import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { useMemo } from "react";
import { type FuzzyRange, fuzzyRank } from "../../lib/fuzzy";
import { type ToolCtx, listTools } from "../../lib/tools";
import { usePaletteStore } from "../../store/palette";
import {
  type SortedTools,
  resolveCursorBySelectedToolId,
  sortToolsForList,
} from "../../store/palette_helpers";
import { useDaemonSnapshot } from "../../store/useDaemonSnapshot";
import { useToolCtx } from "./hooks/useToolCtx";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderWithRanges splits `text` into plain spans and <mark> spans according
// to half-open `ranges` produced by fuzzyRank. Ranges are pre-sorted by start
// and non-overlapping, so a single forward pass suffices. Keys are derived
// from range start indices — stable across renders for deterministic (text, query).
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

// clamp keeps the cursor within [0, max] inclusive (FR-009 clamp-not-wrap).
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
  // inputRef: lets the CommandPalette shell call .focus() in response to
  // store.refocusSeq increments.
  inputRef?: React.RefObject<HTMLInputElement>;
  // httpFactory: swaps SessionsApi for hermetic tests.
  httpFactory?: () => ToolCtx["http"];
  // ADR-0055 lift-state: when CommandPalette captures a submit snapshot it
  // passes frozenList + frozenCursor here. The component renders those instead
  // of subscribing to daemon selectors, and all interaction is disabled.
  frozenList?: SortedTools | null;
  frozenCursor?: number | null;
}

export function ToolSelectPhase(props: ToolSelectPhaseProps = {}) {
  // State-that-renders is subscribed via selector so React re-renders on change.
  // Actions are read off getState() inside handlers to always see the latest ref.
  const query = usePaletteStore((s) => s.query);
  const paramCursor = usePaletteStore((s) => s.paramCursor);
  const composing = usePaletteStore((s) => s.composing);
  const submitting = usePaletteStore((s) => s.submitting);
  const selectedToolId = usePaletteStore((s) => s.selectedToolId);

  // Daemon snapshot (live when not frozen).
  const daemon = useDaemonSnapshot();

  // ToolCtx assembled once per render via shared hook (M2: unify ToolCtx construction).
  // ToolSelectPhase does not need frozenActiveContext, so pass undefined.
  // When ctxOrNull is null (broken httpFactory), confirmTool fast-path falls
  // back safely: usePaletteStore.getState().confirmTool with null ctx logs a
  // warn and advances to paramSelect without submitting — the CommandPalette
  // shell's palette-ctx-error placeholder guards the paramSelect render.
  const ctx = useToolCtx(daemon, props.httpFactory, undefined);

  // isFrozen: use the provided frozenList/frozenCursor instead of live selectors.
  const isFrozen = props.frozenList !== undefined && props.frozenList !== null;

  // Fuzzy-ranked list (skipped when frozen).
  const rankedLive = useMemo(() => {
    if (isFrozen) return [];
    const all = listTools(daemon, daemon.pushCommands);
    return fuzzyRank(all, query, (t) => t.label);
  }, [daemon, query, isFrozen]);

  // Sorted tool list: enabled first, then disabled with reasons.
  const sorted: SortedTools = useMemo(() => {
    if (isFrozen && props.frozenList) return props.frozenList;
    return sortToolsForList(rankedLive, daemon);
  }, [isFrozen, props.frozenList, rankedLive, daemon]);

  // Logical cursor: derived from paramCursor clamped to enabled entries.
  // resolveCursorBySelectedToolId re-anchors when the list changes.
  const maxLogical = sorted.sorted.length - 1;
  const clampedCursor = isFrozen
    ? (props.frozenCursor ?? -1)
    : clamp(paramCursor, maxLogical < 0 ? 0 : maxLogical);

  // Re-anchor cursor when sorted list changes (selectedToolId × list change).
  // We skip this in frozen mode — the frozen cursor is authoritative.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — clampedCursor and selectedToolId are derived from store; only sorted/isFrozen drive re-anchor
  useEffect(() => {
    if (isFrozen) return;
    const newCursor = resolveCursorBySelectedToolId(selectedToolId, clampedCursor, sorted.sorted);
    if (newCursor !== clampedCursor) {
      // Compute delta so moveCursor adjusts the unbounded store counter correctly.
      usePaletteStore.getState().moveCursor(newCursor - clampedCursor);
    }
  }, [sorted, isFrozen]);

  // Per-row flash state for disabled-row Enter feedback (FR-030) and
  // disabled→enabled group transition flash (FR-011 / UAC-014).
  // { logicalIndex, seq } — seq changes trigger the effect even on same row.
  const [flashTarget, setFlashTarget] = useState<{ logicalIndex: number; seq: number } | null>(
    null,
  );
  const flashTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // FR-011 / UAC-014: track previous enabled set to detect disabled→enabled transitions.
  const prevEnabledIdsRef = useRef<Set<string>>(new Set());
  useEffect(() => {
    if (isFrozen) return;
    const currentEnabledIds = new Set(sorted.enabled.map((e) => e.tool.id));
    const prevEnabledIds = prevEnabledIdsRef.current;
    // Find tools that were NOT enabled before but are enabled now.
    for (const entry of sorted.enabled) {
      if (!prevEnabledIds.has(entry.tool.id) && prevEnabledIds.size > 0) {
        // Newly enabled: flash this row.
        setFlashTarget((prev) => ({
          logicalIndex: entry.logicalIndex,
          seq: (prev?.seq ?? 0) + 1,
        }));
        break; // Flash one row per transition cycle to avoid visual noise.
      }
    }
    prevEnabledIdsRef.current = currentEnabledIds;
  }, [sorted, isFrozen]);

  useEffect(() => {
    if (flashTarget === null) return;
    // Clear any previous timer, start new 500 ms window.
    if (flashTimerRef.current !== null) clearTimeout(flashTimerRef.current);
    flashTimerRef.current = setTimeout(() => {
      setFlashTarget(null);
      flashTimerRef.current = null;
    }, 500);
    return () => {
      if (flashTimerRef.current !== null) {
        clearTimeout(flashTimerRef.current);
        flashTimerRef.current = null;
      }
    };
  }, [flashTarget]);

  // jumpCursorTo updates paramCursor and selectedToolId to a given logicalIndex.
  function jumpCursorTo(logicalIndex: number) {
    const entry = sorted.sorted[logicalIndex];
    if (!entry) return;
    const delta = logicalIndex - clampedCursor;
    usePaletteStore.getState().moveCursor(delta);
    usePaletteStore.setState({ selectedToolId: entry.tool.id });
  }

  // enabledLogicalIndexes is the ordered logical-index list of enabled rows;
  // both ArrowDown / Ctrl+N (find next > cursor) and ArrowUp / Ctrl+P (find
  // previous < cursor) walk it. Memoised so the per-keystroke handler stays
  // cheap on long lists.
  const enabledLogicalIndexes = useMemo(
    () => sorted.sorted.filter((s) => s.enabled).map((s) => s.logicalIndex),
    [sorted],
  );

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    // FR-019 / ADR-0040: while IME composition is active do not intercept keys.
    if (composing) return;
    if (isFrozen) return;
    const k = e.key;
    const lower = k.toLowerCase();

    // Arrow / Ctrl+N — move down (skip disabled rows, FR-004 / UAC-015)
    if (k === "ArrowDown" || (e.ctrlKey && lower === "n")) {
      e.preventDefault();
      const next =
        enabledLogicalIndexes.find((i) => i > clampedCursor) ??
        enabledLogicalIndexes[enabledLogicalIndexes.length - 1];
      if (next !== undefined) jumpCursorTo(next);
      return;
    }
    // Arrow / Ctrl+P — move up (skip disabled rows, FR-004 / UAC-015)
    if (k === "ArrowUp" || (e.ctrlKey && lower === "p")) {
      e.preventDefault();
      // Walk backward through enabledLogicalIndexes looking for the nearest
      // entry below the cursor; wrap to the last enabled row when none exists
      // (mirrors ArrowDown's wrap-to-first behaviour).
      let prev: number | undefined;
      for (let i = enabledLogicalIndexes.length - 1; i >= 0; i--) {
        const idx = enabledLogicalIndexes[i];
        if (idx !== undefined && idx < clampedCursor) {
          prev = idx;
          break;
        }
      }
      if (prev === undefined) prev = enabledLogicalIndexes[0];
      if (prev !== undefined) jumpCursorTo(prev);
      return;
    }
    if (k === "Enter") {
      e.preventDefault();
      const target = sorted.sorted.find((s) => s.logicalIndex === clampedCursor);
      if (!target) return;
      if (!target.enabled) {
        // FR-005 / FR-030: disabled row Enter → feedback, palette stays open.
        usePaletteStore
          .getState()
          .emitDisabledFeedback(target.tool.label, target.reason ?? "Unavailable");
        setFlashTarget((prev) => ({
          logicalIndex: target.logicalIndex,
          seq: (prev?.seq ?? 0) + 1,
        }));
        return;
      }
      void usePaletteStore.getState().confirmTool(target.tool.id, ctx ?? undefined);
      return;
    }
  }

  // Compute aria-activedescendant from clampedCursor.
  const activeDescendant =
    sorted.sorted.length > 0 && clampedCursor >= 0 ? `palette-opt-${clampedCursor}` : undefined;

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
        aria-activedescendant={activeDescendant}
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
        ARIA combobox + listbox pattern: the combobox <input> owns focus via
        aria-activedescendant. We use a single <div role="listbox"> containing
        enabled options, a presentation separator, and disabled options (FR-001 / FR-002).
        Role='option' elements are NOT individually focusable (WAI-ARIA combobox pattern).
      */}
      <div
        id="palette-listbox"
        // biome-ignore lint/a11y/useSemanticElements: <select> cannot host a free-text combobox filter (FR-008); listbox role is the WAI-ARIA canonical fit
        role="listbox"
        tabIndex={-1}
        aria-disabled={submitting || isFrozen ? true : undefined}
        aria-activedescendant={activeDescendant}
        data-testid="palette-listbox"
        onMouseLeave={() => {
          // FR-008 / UAC-008: mouseleave keeps cursor unchanged — no action.
        }}
      >
        {/* Enabled rows (FR-001) */}
        {sorted.enabled.map((entry) => {
          const isActive = entry.logicalIndex === clampedCursor;
          const label = entry.tool.label;
          // Find fuzzy ranges from ranked list (live only; frozen has no ranges).
          const rangesForEntry = isFrozen
            ? []
            : (rankedLive.find((h) => h.item.id === entry.tool.id)?.ranges ?? []);
          // FR-011 / UAC-014: flash class when this row newly transitioned from disabled.
          const isFlashing =
            flashTarget !== null && flashTarget.logicalIndex === entry.logicalIndex;
          return (
            <div
              key={entry.tool.id}
              id={`palette-opt-${entry.logicalIndex}`}
              // biome-ignore lint/a11y/useSemanticElements: <option> only valid inside <select>; combobox/listbox uses role="option" on generic elements
              role="option"
              tabIndex={-1}
              aria-selected={isActive}
              data-tool-id={entry.tool.id}
              data-cursor={isActive ? "true" : "false"}
              className={isFlashing ? "palette-listbox__row--flash" : undefined}
              onPointerMove={() => {
                // FR-006 / UAC-006: pointermove on enabled row updates cursor.
                if (composing || submitting || isFrozen) return;
                jumpCursorTo(entry.logicalIndex);
              }}
              onMouseDown={(e) => {
                e.preventDefault();
                const state = usePaletteStore.getState();
                if (state.composing || state.submitting || isFrozen) return;
                void state.confirmTool(entry.tool.id, ctx ?? undefined);
              }}
            >
              {renderWithRanges(label, rangesForEntry)}
            </div>
          );
        })}

        {/* Separator between enabled and disabled rows (FR-002) */}
        {sorted.disabled.length > 0 && sorted.enabled.length > 0 && (
          <div
            role="presentation"
            aria-hidden="true"
            className="palette-listbox__separator"
            data-testid="palette-separator"
          />
        )}

        {/* Disabled rows (FR-002 / FR-003 / ADR-0047) */}
        {sorted.disabled.map((entry) => {
          const isActive = entry.logicalIndex === clampedCursor;
          const isFlashing =
            flashTarget !== null && flashTarget.logicalIndex === entry.logicalIndex;
          const rangesForEntry = isFrozen
            ? []
            : (rankedLive.find((h) => h.item.id === entry.tool.id)?.ranges ?? []);
          return (
            <div
              key={entry.tool.id}
              id={`palette-opt-${entry.logicalIndex}`}
              // biome-ignore lint/a11y/useSemanticElements: <option> only valid inside <select>; combobox/listbox uses role="option" on generic elements
              role="option"
              tabIndex={-1}
              aria-selected={isActive}
              aria-disabled="true"
              data-tool-id={entry.tool.id}
              data-cursor={isActive ? "true" : "false"}
              className={[
                "palette-listbox__row--disabled",
                isFlashing ? "palette-listbox__row--flash" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              // FR-007 / UAC-007: pointermove on disabled row does not move cursor.
              // We do NOT add onPointerMove; CSS :hover provides subtle visual feedback.
              onMouseDown={(e) => {
                e.preventDefault();
                const state = usePaletteStore.getState();
                if (state.composing || state.submitting || isFrozen) return;
                // FR-005 / FR-030: mousedown on disabled → feedback, no confirm.
                state.emitDisabledFeedback(entry.tool.label, entry.reason ?? "Unavailable");
                setFlashTarget((prev) => ({
                  logicalIndex: entry.logicalIndex,
                  seq: (prev?.seq ?? 0) + 1,
                }));
              }}
            >
              {/* FR-003: warning icon + reason inline */}
              <span className="palette-listbox__row--disabled-icon" aria-hidden="true">
                ⚠
              </span>
              <span className="palette-listbox__row--disabled-label">
                {renderWithRanges(entry.tool.label, rangesForEntry)}
              </span>
              <span className="palette-listbox__row--disabled-reason">
                {entry.reason ?? "Unavailable"}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
