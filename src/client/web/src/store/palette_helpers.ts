// Pure helpers extracted from store/palette.ts so palette.ts stays under the
// 500-line file-size limit (AGENTS.md). Behavior-preserving: each function is
// the verbatim implementation that used to live inline; comments are kept
// because they document the rationale palette.ts callers depend on.

import type { ApiHttpError } from "../api/sessions";
import { type DaemonSnapshot, type ToolCtx, type ToolDef, listTools } from "../lib/tools";
import type { PaletteScope } from "./palette";

// initialScopeFromSnapshot encodes FR-004: open with `push` selected iff there
// is an active session AND the daemon-global ActiveOccupant is 'frame'. Any
// other state (no active session, or active but occupant is 'main'/'log'/
// missing) opens at 'standard'. Pulled into a free function so the openPalette
// reducer reads as a one-line `scope: initialScope(snapshot)`.
export function initialScope(snapshot: DaemonSnapshot | undefined): PaletteScope {
  if (!snapshot) return "standard";
  if (!snapshot.activeSessionID) return "standard";
  if (snapshot.activeOccupant !== "frame") return "standard";
  return "push";
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
export function messageOf(e: unknown): string {
  if (e instanceof Error && e.message !== "") return e.message;
  const s = String(e);
  return s === "" ? "unknown error" : s;
}

// SubmitErrorBranch is the classifier output for submit()'s catch-block. Each
// branch maps 1:1 onto a side-effect bundle palette.submit executes:
//   - 'auth':    fixed Japanese toast + full close (FR-024 auth path)
//   - 'http':    inline state.error + clear submitting (FR-024 server-message)
//   - 'unknown': console.error + notify.error toast + inline state.error
// Keeping the classification pure (this function does no I/O, no state writes)
// lets palette.submit stay a thin dispatcher and shrinks the file under the
// 500-line cap. (FR-024 ADR-0046)
export type SubmitErrorBranch =
  | { kind: "auth" }
  | { kind: "http"; message: string }
  | { kind: "unknown"; message: string; cause: unknown };

export function classifySubmitError(e: unknown): SubmitErrorBranch {
  if (isApiHttpError(e) && e.status === 401) return { kind: "auth" };
  if (isApiHttpError(e)) return { kind: "http", message: messageOf(e) };
  return { kind: "unknown", message: messageOf(e), cause: e };
}
