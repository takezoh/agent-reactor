// ToolDef / ToolRegistry — declarative command-palette tool layer.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// ADRs:
//   - 0036 palette-2phase-store-architecture (store is pure state; I/O lives
//     in ToolDef.submit, ctx-injected)
//   - 0042 palette-new-session-payload-wire-mirror (new-session sends
//     sandbox: "host" on the wire; the in-UI "host" toggle maps to that)
//   - 0047 palette-disabledreason-single-source (push availability and
//     submit-time re-validation share a single disabledReason(daemon)
//     function per ToolDef)
//   - 0033 display-label-empty-policy (stop-session listbox getText respects
//     the title→subtitle→id fallback)
//
// This file is the framework + standard scope plus dynamic push expansion.
// session-config-extension supplies the curated `pushCommands` list on the
// daemon snapshot; `listTools()` calls `toolsForPush()` to materialize one
// scope='push' ToolDef per entry, each sharing scopeDisabledReason('push',
// daemon) so the segment UI and the submit-time re-validation can never
// drift (ADR-0047).

import type { SessionsApi } from "../api/sessions";
import type { SessionConfigProject } from "../api/sessions";
import { displayLabel } from "../components/SessionList";
import type { SessionInfo } from "../wire/server";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type ToolScope = "standard" | "push";

// ParamDef describes one input field of a ToolDef. `options` is the listbox
// source (one entry per choice, with a getText projection so callers can
// format values without leaking display formatting back into the wire
// payload). `options=null` means "text input" — the value is taken from a
// free-form input field rather than a listbox selection.
export interface ParamDef<V = unknown> {
  id: string;
  label: string;
  options: ReadonlyArray<{ value: V; getText: (v: V) => string }> | null;
}

// DaemonSnapshot is the read-only slice of daemon state that ToolDef.submit
// and ToolDef.disabledReason can observe. We define it locally instead of
// reusing store/daemon DaemonState to keep the tool layer decoupled from
// zustand internals (snapshot is a plain value, no actions) and to allow
// later sources (e.g. session-config-extension) to extend `projects` /
// `pushCommands` without touching the store shape. ADR-0036 keeps store
// pure; ctx.daemon is the projection palette-store hands to submit().
// DaemonOccupant mirrors the Go-side OccupantKind ('main' | 'log' | 'frame'):
// what currently occupies pane 0.1 in the TUI. Web UI only cares about this
// to gate push scope (push targets the active session's frame pane); see
// FR-004 (initial scope selection) and FR-006 (push-segment disabled state).
// Optional because not all callers (older tests, stub snapshots) provide it;
// `undefined` is treated as "no frame" for scope purposes.
export type DaemonOccupant = "main" | "log" | "frame";

export interface DaemonSnapshot {
  sessions: SessionInfo[];
  activeSessionID: string | null;
  activeOccupant?: DaemonOccupant;
  projects: SessionConfigProject[];
  pushCommands: string[];
}

// NotificationsApi is the minimal surface ToolDef.submit needs for toast
// emission. Defined here (rather than imported from store/notifications)
// because the store today exposes add({level, message}); we want a
// success/error helper signature on ctx so submit code reads naturally
// ("notify.success(...)") without leaking the store's action shape into
// every tool. The palette-store adapter (later task) supplies the concrete
// implementation by wrapping useNotificationsStore.getState().add.
export interface NotificationsApi {
  success(message: string): void;
  error(message: string): void;
}

// ToolStoreCtx is the narrow PaletteActions slice ToolDef.submit consumes
// through ctx.store. Renamed from `PaletteActions` to avoid a collision with
// the broader interface of the same name in store/palette.ts (the full
// PaletteState action set used by the React layer). We expose only the two
// callbacks ToolDef.submit needs today:
//   - close()                — close the palette after a successful submit
//   - clearActiveIf(id)      — if `id` is the currently active session,
//                              clear daemon activeSessionID (stop-session
//                              must not leave a dangling active pointer).
// ToolStoreCtx stays minimal on purpose: every new method here is one
// more thing test fakes must implement. Tools that need richer access
// should go through ctx.http or ctx.daemon instead.
export interface ToolStoreCtx {
  close(): void;
  clearActiveIf(sessionId: string): void;
}

// ToolDaemonActions is the narrow daemon-action surface a ToolDef.submit
// needs to invoke (as opposed to ctx.daemon, which is a read-only snapshot).
// FR-021: new-session.submit must call daemon.selectSession(rc.id) so the
// freshly-created session becomes active in the SessionList without waiting
// for the next view-update tick. Kept minimal — adding daemon writes to
// ToolDef is an exception (ADR-0036), not the rule; widening this contract
// should require a fresh plan.
export interface ToolDaemonActions {
  selectSession(id: string | null): void;
}

// PaletteActions is the legacy name retained as a deprecated alias for
// ToolStoreCtx so the existing imports in tools.test.ts compile without
// churn. New code should reference ToolStoreCtx directly. We do NOT
// re-export PaletteActions from store/palette through this name to keep
// the two layers' contracts visibly distinct at the import site.
/** @deprecated Use ToolStoreCtx instead. Kept for the existing test imports. */
export type PaletteActions = ToolStoreCtx;

export interface ToolCtx {
  http: SessionsApi;
  daemon: DaemonSnapshot;
  // daemonActions: write-side daemon API (selectSession). Distinct from
  // ctx.daemon (read-only snapshot) so the read/write asymmetry stays
  // visible at the ToolDef.submit call site (FR-021).
  daemonActions: ToolDaemonActions;
  notify: NotificationsApi;
  store: ToolStoreCtx;
}

export interface ToolDef {
  id: string;
  label: string;
  scope: ToolScope;
  params: ParamDef[] | null;
  // disabledReason returns null when the tool is currently usable, and a
  // short human-readable Japanese sentence when not. ADR-0047 makes this
  // the single source of truth for both ScopeSegment "disabled" rendering
  // and submit-time re-validation. Standard scope is always null in this
  // task; push scope adds real predicates in tools-registry-dynamic-push.
  disabledReason(daemon: DaemonSnapshot): string | null;
  submit(ctx: ToolCtx, payload: Record<string, unknown>): Promise<void>;
}

// ---------------------------------------------------------------------------
// Standard tools
// ---------------------------------------------------------------------------

// readString narrows a payload value to a non-empty string and throws
// otherwise. Tools are called by the palette store, which guards against
// missing required params at param-select time; but a defensive check here
// keeps the wire payload clean if a future caller forgets a field, and
// gives the developer a clear error message instead of "TypeError: cannot
// read property of undefined" deep in fetch.
function readString(payload: Record<string, unknown>, key: string): string {
  const v = payload[key];
  if (typeof v !== "string" || v === "") {
    throw new Error(`tool payload: missing or empty string field "${key}"`);
  }
  return v;
}

// readOptionalBool returns the boolean value for `key` if present, or
// undefined when the field is absent. Anything other than boolean / absent
// throws — silent coercion of "true" / 1 / null would mask wire bugs.
function readOptionalBool(payload: Record<string, unknown>, key: string): boolean | undefined {
  if (!(key in payload)) return undefined;
  const v = payload[key];
  if (v === undefined) return undefined;
  if (typeof v !== "boolean") {
    throw new Error(`tool payload: field "${key}" must be boolean, got ${typeof v}`);
  }
  return v;
}

// readOptionalHostFlag interprets the in-UI "host" toggle (boolean) and
// projects it onto the wire vocabulary defined by ADR-0042: true → "host",
// false / absent → undefined (= "auto" on the server). We keep "host" the
// only legal string so a future "sandbox": "auto" leak in the payload is
// caught at the type level.
function readSandboxFromHostToggle(payload: Record<string, unknown>): "host" | undefined {
  const host = readOptionalBool(payload, "host");
  return host === true ? "host" : undefined;
}

const newSessionTool: ToolDef = {
  id: "new-session",
  label: "新しいセッション",
  scope: "standard",
  // project = listbox (filled by ctx.daemon.projects at param-select time);
  // command = free-form text input. worktree / sandbox are NOT declared as
  // params here because the ParamSelectPhase gates them via Tab / Shift+Tab
  // toggles whose visibility depends on the selected project's isGit /
  // isSandboxed flags (FR-013, FR-014). They flow through `payload` as
  // optional fields ("worktree": bool, "host": bool) — see submit below.
  params: [
    {
      id: "project",
      label: "プロジェクト",
      // Note: options=[] is a placeholder. The palette store fills the
      // listbox at param-select time from ctx.daemon.projects, since the
      // available projects change as session-config updates. Keeping the
      // ParamDef declaration static (no closure over daemon) makes
      // listTools() deterministic and trivially testable.
      options: [],
    },
    {
      id: "command",
      label: "コマンド",
      options: null,
    },
  ],
  disabledReason: () => null,
  async submit(ctx, payload) {
    const project = readString(payload, "project");
    const command = readString(payload, "command");
    const worktree = readOptionalBool(payload, "worktree");
    const sandbox = readSandboxFromHostToggle(payload);
    // Build the payload with only present fields so JSON.stringify doesn't
    // serialize "worktree": undefined / "sandbox": undefined (which would
    // break the server's omitempty contract on round-trip).
    const req: {
      project: string;
      command: string;
      worktree?: boolean;
      sandbox?: "host";
    } = { project, command };
    if (worktree !== undefined) req.worktree = worktree;
    if (sandbox !== undefined) req.sandbox = sandbox;
    const rc = await ctx.http.createSession(req);
    // FR-021: select the freshly-created session as active immediately.
    // Without this, SessionList stays focused on whatever was active
    // before (or nothing), and the user has to manually click the new
    // row — which defeats the whole point of "New Session" being a CTA.
    // We MUST do this before notify+close so that even if notify or close
    // throws the activeSessionID write has already landed.
    ctx.daemonActions.selectSession(rc.id);
    // Notification first, then close: if notify throws the palette stays
    // open with the active error toast visible. close() is idempotent so a
    // future re-entry that calls submit() twice is fine.
    ctx.notify.success("セッションを作成しました");
    ctx.store.close();
  },
};

const stopSessionTool: ToolDef = {
  id: "stop-session",
  label: "セッションを停止",
  scope: "standard",
  // sessionId = listbox of current sessions. The palette store materializes
  // the options from ctx.daemon.sessions at param-select time (same reason
  // newSessionTool.params[0].options stays empty here).
  params: [
    {
      id: "sessionId",
      label: "対象セッション",
      options: [],
    },
  ],
  disabledReason: () => null,
  async submit(ctx, payload) {
    const sessionId = readString(payload, "sessionId");
    await ctx.http.deleteSession(sessionId);
    // Clear the active pointer if we just killed the active session.
    // Going through ctx.store keeps palette/daemon state coordination in
    // one place (ADR-0036) instead of having tools poke daemon directly.
    ctx.store.clearActiveIf(sessionId);
    ctx.notify.success("セッションを停止しました");
    ctx.store.close();
  },
};

// ---------------------------------------------------------------------------
// Public ParamDef option-source helpers
// ---------------------------------------------------------------------------
//
// These are exposed so the palette store / ParamSelectPhase can materialize
// listbox options at param-select time without duplicating the projection
// logic. ADR-0033 mandates the title → subtitle → id fallback for session
// display labels; centralizing it here keeps stop-session and any future
// per-session listbox in sync.

export function projectOptions(
  daemon: DaemonSnapshot,
): ReadonlyArray<{ value: string; getText: (v: string) => string }> {
  return daemon.projects.map((p) => ({
    value: p.path,
    getText: (v: string) => v,
  }));
}

export function sessionOptions(
  daemon: DaemonSnapshot,
): ReadonlyArray<{ value: string; getText: (v: string) => string }> {
  // Snapshot the per-session labels at materialization time so the listbox
  // entry's getText is a pure projection of `value` (the session id). This
  // matches ParamDef.options' contract: getText is a function of value
  // alone, not of an enclosing scope that might be stale by the time the
  // listbox renders.
  const labelById = new Map<string, string>(
    daemon.sessions.map((s) => [s.id, displayLabel(s.view.card, s.id)]),
  );
  return daemon.sessions.map((s) => ({
    value: s.id,
    getText: (v: string) => labelById.get(v) ?? v,
  }));
}

// ---------------------------------------------------------------------------
// Scope availability
// ---------------------------------------------------------------------------

// scopeDisabledReason is the single source of truth (ADR-0047) for whether a
// given palette scope is currently usable, returning either a short Japanese
// reason (rendered as the disabled sub-text in ScopeSegment) or null when the
// scope is enabled. The same helper is consumed by tools-registry-dynamic-push
// to gate per-push ToolDef.disabledReason — keeping the literal strings in one
// place so the segment UI and the submit-time re-validation can never drift.
//
// - 'standard' is always enabled (FR-004 baseline): there is no precondition
//   the standard tools (new-session, stop-session) can fail to meet from the
//   daemon's perspective; any HTTP-level rejection surfaces inside submit().
// - 'push' requires an active session AND an active-occupant of 'frame'
//   (FR-005, FR-006). occupant is optional on the snapshot because the wire
//   may not yet populate it; treat undefined as "no frame" — failing closed
//   keeps push disabled in the worst case rather than letting a stale push
//   fire against a session whose pane just shifted to the main/log buffer.
export function scopeDisabledReason(scope: ToolScope, daemon: DaemonSnapshot): string | null {
  if (scope === "standard") return null;
  if (!daemon.activeSessionID) return "アクティブセッションなし";
  if (daemon.activeOccupant !== "frame") return "push 対象 driver なし";
  return null;
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// makePushToolDef builds a paramless ToolDef for one curated push command.
// Each push ToolDef:
//   - id is "push:" + command, keyed so palette-store can find it via the
//     same lookup it uses for standard tools
//   - label is the bare command string (e.g. "save"), so the listbox row
//     shows exactly what will be sent
//   - params=null marks the tool as paramless (FR-010): hitting Enter on the
//     listbox entry fires submit() immediately without entering ParamSelectPhase
//   - disabledReason delegates to scopeDisabledReason('push', daemon) so the
//     ScopeSegment "disabled" sub-text and the per-ToolDef gate are the same
//     function call (ADR-0047). Standard tools have their own permanent-null
//     disabledReason; push tools mirror the scope-level predicate exactly.
//   - submit calls ctx.http.pushCommand(activeSessionID, command). The store
//     (palette-store.submit) re-evaluates disabledReason before calling submit
//     (FR-023), so this implementation does NOT re-check — duplicating the
//     guard here would defeat the single-source rule and risk the two checks
//     drifting. We still defensively read activeSessionID and throw rather
//     than send a "" sessionId on the wire, in case a test calls submit
//     directly without going through palette-store.
function makePushToolDef(command: string): ToolDef {
  return {
    id: `push:${command}`,
    label: command,
    scope: "push",
    params: null,
    disabledReason(daemon: DaemonSnapshot): string | null {
      return scopeDisabledReason("push", daemon);
    },
    async submit(ctx: ToolCtx): Promise<void> {
      const sessionId = ctx.daemon.activeSessionID;
      if (!sessionId) {
        // palette-store should have blocked this via the disabledReason
        // re-check (FR-023); reaching here means the store skipped the gate
        // or a test exercised submit() directly. Either way we cannot send a
        // push to "no session", so throw a clear message instead of POSTing
        // to /api/sessions//push (which would 404 silently on some routers).
        throw new Error("no active session");
      }
      await ctx.http.pushCommand(sessionId, command);
      // Same ordering as standard tools: notify first, then close. If notify
      // throws the palette stays open with the error visible; ctx.store.close
      // is a no-op when the palette is already closed.
      ctx.notify.success(`push: ${command}`);
      ctx.store.close();
    },
  };
}

// toolsForPush materializes one push ToolDef per entry in pushCommands. The
// list is taken straight from the daemon snapshot's pushCommands (populated
// by session-config-extension); we do NOT filter on disabledReason here so
// that ScopeSegment can render the segment as "disabled with reason" rather
// than disappearing the segment entirely when push is unavailable. Order
// preserves the configured pushCommands order — the curated server list is
// the source of truth for surfacing priority.
export function toolsForPush(
  _daemon: DaemonSnapshot,
  pushCommands: ReadonlyArray<string>,
): ToolDef[] {
  return pushCommands.map((cmd) => makePushToolDef(cmd));
}

// listTools is the single entry point palette-store calls to enumerate
// available tools. Ordering is deterministic: standard tools first (new-
// session, stop-session — the user-visible "create / kill" pair), then
// one push-scope ToolDef per entry in pushCommands. shutdown is
// intentionally NOT registered here (FR-028) — palette-store handles
// shutdown via a different code path (a dedicated confirm modal) so it does
// not appear in the tool list, even if "shutdown" or "/quit" happens to be
// in the curated push list.
export function listTools(daemon: DaemonSnapshot, pushCommands: ReadonlyArray<string>): ToolDef[] {
  return [newSessionTool, stopSessionTool, ...toolsForPush(daemon, pushCommands)];
}
