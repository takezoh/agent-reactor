// Pure helpers extracted from store/palette.ts so palette.ts stays under the
// 500-line file-size limit (AGENTS.md). Behavior-preserving: each function is
// the verbatim implementation that used to live inline; comments are kept
// because they document the rationale palette.ts callers depend on.
//
// sortToolsForList and resolveCursorBySelectedToolId were added by the
// palette-redesign m1 (pure helpers) milestone:
//   - sortToolsForList: UAC-004 / FR-001 / FR-002
//   - resolveCursorBySelectedToolId: UAC-006 / UAC-014 / FR-026

import type { ApiHttpError } from "../api/sessions";
import { type DaemonSnapshot, type ToolCtx, type ToolDef, listTools } from "../lib/tools";

// ---------------------------------------------------------------------------
// sortToolsForList
// ---------------------------------------------------------------------------

// SortedToolEntry is a single row in the logical tool list (separator-free).
// logicalIndex is the 0-based index within the enabled+disabled concatenation;
// enabled entries come first, disabled entries follow (FR-001 / FR-002).
export interface SortedToolEntry {
  tool: ToolDef;
  enabled: boolean;
  reason: string | null;
  // logicalIndex is the separator-excluded 0-based position.
  // enabled rows occupy 0..enabled.length-1; disabled rows follow.
  logicalIndex: number;
}

export interface SortedTools {
  enabled: SortedToolEntry[];
  disabled: SortedToolEntry[];
  // sorted is enabled+disabled concatenated in logicalIndex order.
  // The presentation layer inserts the visual separator between them.
  sorted: SortedToolEntry[];
}

// sortToolsForList partitions a fuzzy-ranked ToolDef list into enabled and
// disabled groups while preserving the intra-group ordering supplied by
// fuzzyRanked (which reflects registry order when no query is active and
// fuzzy-score order when one is). Invariants (UAC-004 / FR-001 / FR-002):
//   - Each ToolDef's disabledReason(daemon) is called exactly once.
//   - All entries in fuzzyRanked appear in sorted (no filtering here).
//   - enabled entries precede disabled entries; within each group the original
//     fuzzyRanked order is preserved.
//   - logicalIndex is a contiguous 0-based sequence across sorted.
export function sortToolsForList<T extends { item: ToolDef }>(
  fuzzyRanked: ReadonlyArray<T>,
  daemon: DaemonSnapshot,
): SortedTools {
  const enabled: SortedToolEntry[] = [];
  const disabled: SortedToolEntry[] = [];

  for (const hit of fuzzyRanked) {
    const reason = hit.item.disabledReason(daemon);
    const isEnabled = reason === null;
    if (isEnabled) {
      enabled.push({ tool: hit.item, enabled: true, reason: null, logicalIndex: -1 });
    } else {
      disabled.push({ tool: hit.item, enabled: false, reason, logicalIndex: -1 });
    }
  }

  // Assign contiguous logicalIndex values: enabled group first, then disabled.
  const sorted: SortedToolEntry[] = [];
  let idx = 0;
  for (const entry of enabled) {
    entry.logicalIndex = idx++;
    sorted.push(entry);
  }
  for (const entry of disabled) {
    entry.logicalIndex = idx++;
    sorted.push(entry);
  }

  return { enabled, disabled, sorted };
}

// ---------------------------------------------------------------------------
// resolveCursorBySelectedToolId
// ---------------------------------------------------------------------------

// resolveCursorBySelectedToolId recalculates the logical cursor position after
// the sorted list changes (e.g. query update or daemon snapshot change).
// Rules (UAC-006 / UAC-014 / FR-026):
//   (a) If prevSelectedId is still in sorted AND is enabled → return its new
//       logicalIndex (identity preservation).
//   (b) Otherwise search forward from prevLogicalIndex for the nearest enabled
//       entry (forward-first per FR-026).
//   (c) If none found forward, search backward.
//   (d) If sorted has no enabled entries → return -1.
//   (e) If prevSelectedId is null and there is an enabled entry → return 0
//       (anchor=0, which is the first enabled entry via forward search).
//
// NOTE: sortToolsForList guarantees sorted[i].logicalIndex === i (contiguous
// 0-based), so we can index directly by position instead of searching by
// logicalIndex (avoids O(N²) find loops).
export function resolveCursorBySelectedToolId(
  prevSelectedId: string | null,
  prevLogicalIndex: number,
  sorted: ReadonlyArray<SortedToolEntry>,
): number {
  // Fast path: no enabled entries at all.
  const hasEnabled = sorted.some((e) => e.enabled);
  if (!hasEnabled) return -1;

  // Case (a): prevSelectedId found among enabled entries.
  if (prevSelectedId !== null) {
    const same = sorted.find((e) => e.enabled && e.tool.id === prevSelectedId);
    if (same) return same.logicalIndex;
  }

  // Cases (b)+(c)+(e): search nearest enabled by logical index proximity.
  // prevSelectedId is null (case e) or the id is gone/disabled.
  // Use prevLogicalIndex as the anchor (0 when prevSelectedId is null).
  const anchor = prevSelectedId === null ? 0 : prevLogicalIndex;

  // Forward search: from anchor onward (FR-026 forward-first).
  for (let i = anchor; i < sorted.length; i++) {
    if (sorted[i]?.enabled) return i;
  }

  // Backward search: from anchor-1 down to 0 (fallback).
  for (let i = anchor - 1; i >= 0; i--) {
    if (sorted[i]?.enabled) return i;
  }

  // Should not reach here given hasEnabled check above, but be safe.
  return -1;
}

// findToolForSubmit re-runs listTools and returns the ToolDef matching id.
// We re-run instead of caching the tool from confirmTool time so push tools
// whose set changed between confirm and submit (e.g. session disappeared)
// resolve consistently with the live daemon snapshot — and so a missing tool
// is detectable as `null` rather than fired stale.
export function findToolForSubmit(ctx: ToolCtx, selectedToolId: string | null): ToolDef | null {
  if (selectedToolId === null) return null;
  const tools = listTools(ctx.daemon, ctx.daemon.pushCommands);
  return tools.find((t) => t.id === selectedToolId) ?? null;
}

// isParamless decides "submit immediately on confirm" per FR-010: a tool whose
// params is null or empty has nothing to ask the user, so the confirm itself
// IS the submit. We treat `null` and `[]` as equivalent here even though the
// ParamDef type distinguishes them — the user-visible behavior is the same
// (no param-select phase) and centralizing the check avoids two callsites
// (confirmTool + ParamSelectPhase render) drifting on whether `[]` should
// render a 0-field form.
export function isParamless(tool: ToolDef): boolean {
  if (tool.params === null) return true;
  if (tool.params.length === 0) return true;
  return false;
}

// isApiHttpError is a structural type-guard for the ApiHttpError shape
// (`Error` with a numeric `status` field). We avoid `instanceof` because the
// API layer constructs the error via `new Error(...) as ApiHttpError` — there
// is no class to instanceof against, and the shape check is what actually
// distinguishes HTTP failures from network / wire-format / synchronous
// throws inside ToolDef.submit.
export function isApiHttpError(e: unknown): e is ApiHttpError {
  if (!(e instanceof Error)) return false;
  const status = (e as { status?: unknown }).status;
  return typeof status === "number";
}

// messageOf normalizes an unknown thrown value to a non-empty string we can
// stash in state.error. Synchronous bugs inside ToolDef.submit (TypeError,
// ReferenceError) need to surface SOMETHING in the palette rather than vanish;
// the empty-string guard catches `throw ''` / `throw null` style anti-patterns.
// The fallback is overridable so HTTP / unknown branches can substitute their
// own English default ('HTTP error' vs. 'Unknown error') without each call
// site re-implementing the empty-string guard. Errors with an empty message
// also fall back (instead of degrading to the default "Error" String(e)
// projection) so an HTTP error with no server message reads as 'HTTP error'
// rather than the meaningless class name.
export function messageOf(e: unknown, fallback = "Unknown error"): string {
  if (e instanceof Error) {
    return e.message !== "" ? e.message : fallback;
  }
  const s = String(e);
  return s === "" ? fallback : s;
}

// SubmitErrorBranch is the classifier output for submit()'s catch-block. Each
// branch maps 1:1 onto a side-effect bundle palette.submit executes:
//   - 'auth':    fixed English toast + full close (FR-024 auth path)
//   - 'http':    inline state.error + clear submitting (FR-024 server-message
//                pass-through; classifier substitutes 'HTTP error' when the
//                server returns an empty message so the inline error is never
//                a blank string)
//   - 'unknown': console.error + notify.error toast + inline state.error
//                (classifier defaults the message to 'Unknown error' so the
//                inline state and the prefixed toast are both readable when
//                the thrown value carries no message)
// Keeping the classification pure (this function does no I/O, no state writes)
// lets palette.submit stay a thin dispatcher and shrinks the file under the
// 500-line cap. (FR-024 ADR-0046)
export type SubmitErrorBranch =
  | { kind: "auth" }
  | { kind: "http"; message: string }
  | { kind: "unknown"; message: string; cause: unknown };

export function classifySubmitError(e: unknown): SubmitErrorBranch {
  if (isApiHttpError(e) && e.status === 401) return { kind: "auth" };
  if (isApiHttpError(e)) return { kind: "http", message: messageOf(e, "HTTP error") };
  return { kind: "unknown", message: messageOf(e, "Unknown error"), cause: e };
}
