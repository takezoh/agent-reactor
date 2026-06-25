// usePaletteStore — pure state + actions for the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// ADRs:
//   - 0036 palette-2phase-store-architecture (this store holds only state and
//     state-transition actions; HTTP / DOM live in ToolDef.submit / the
//     CommandPalette component respectively)
//   - 0040 palette-ime-suppression-in-store (`composing` is the single source
//     of truth for IME suppression; every input action guards on it)
//   - 0047 palette-disabledreason-single-source (submit re-evaluates
//     tool.disabledReason(ctx.daemon) so a push tool that becomes invalid
//     between open and confirm cannot fire the HTTP request)
//
// DOM-touching responsibilities deliberately deferred to the React layer:
//   - input.focus() is triggered by CommandPalette observing `refocusSeq`
//     (incremented by refocusInput()).
//   - opener.focus() restoration on close lives in CommandPalette's effect
//     cleanup, reading the `opener` field this store carries.
//   - TerminalPane blur / cursor clamp inside listboxes live in the phase
//     components.
//
// The store therefore has zero references to `document`, `window`, or any
// HTMLElement member. The only HTMLElement value it touches is the opener
// stored verbatim and handed back to the React layer.

import { create } from "zustand";
import { type DaemonSnapshot, type ToolCtx, type ToolScope, listTools } from "../lib/tools";
import { useDaemonStore } from "./daemon";
import {
  classifySubmitError,
  findToolForSubmit,
  initialScope,
  isParamless,
} from "./palette_helpers";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type PalettePhase = "toolSelect" | "paramSelect";

// PaletteScope is the palette-layer alias for ToolScope (lib/tools): both
// describe the same "standard | push" axis (Y4 — keeping two literal unions
// risked drifting when a new scope is added). Re-exporting the canonical
// ToolScope keeps the type identity single-sourced while preserving the
// PaletteScope name at every existing call site (store actions, tests).
export type PaletteScope = ToolScope;

export interface OpenPaletteOptions {
  opener?: HTMLElement | null;
  // daemonSnapshot is provided by the opener (Header / hotkey hook) so the
  // store can decide the initial scope at open time without sampling the
  // daemon store itself (keeps the openPalette transition deterministic in
  // unit tests — the test passes whatever snapshot it wants).
  daemonSnapshot?: DaemonSnapshot;
  // preselectToolId pre-selects a tool by id and skips ToolSelectPhase
  // (ADR-0043 / FR-021 — the Header's "New Session" CTA needs to land on
  // 'new-session' regardless of the fuzzy query, since the Japanese label
  // won't match an English search). When the id resolves under the chosen
  // scope we advance to phase='paramSelect'; otherwise we fall back to the
  // unfiltered toolSelect open with a console.warn (the alternative —
  // closing immediately — would leave the CTA feeling broken).
  // Paramless tools are NOT auto-submitted because openPalette has no
  // ToolCtx; the user lands on a 0-field paramSelect whose Enter routes
  // through submit(ctx) normally.
  preselectToolId?: string;
}

export interface PaletteState {
  open: boolean;
  phase: PalettePhase;
  scope: PaletteScope;
  selectedToolId: string | null;
  paramValues: Record<string, unknown>;
  paramCursor: number;
  query: string;
  composing: boolean;
  submitting: boolean;
  error: string | null;
  opener: HTMLElement | null;
  // refocusSeq is a monotonic counter. CommandPalette's useEffect watches it
  // and calls input.focus() each time it increments — that way the store
  // signals "please refocus" without ever holding a ref to the input itself
  // (FR-029, ADR-0036).
  refocusSeq: number;
}

export interface PaletteActions {
  openPalette(opts?: OpenPaletteOptions): void;
  close(): void;
  back(): void;
  setScope(scope: PaletteScope): void;
  setQuery(query: string): void;
  moveCursor(delta: number): void;
  // confirmTool optionally takes a ToolCtx so the paramless-tool fast path
  // (FR-010: tools whose params is null/empty submit immediately on confirm)
  // can fire submit() without a second round-trip through the React layer.
  // When ctx is omitted, paramless tools fall through to phase='paramSelect'
  // with empty params — which renders as a 0-field form whose "Enter to
  // submit" press will route through submit(ctx) as usual.
  confirmTool(id: string, ctx?: ToolCtx): void | Promise<void>;
  setParam(key: string, value: unknown): void;
  toggleWorktree(): void;
  toggleHost(): void;
  setComposing(composing: boolean): void;
  submit(ctx: ToolCtx): Promise<void>;
  refocusInput(): void;
  // clearActiveIf is the palette-layer wrapper around daemon's selectSession.
  // ToolDef.submit calls this through ctx.store after stop-session so the
  // stale active pointer is dropped without the tool importing the daemon
  // store directly (keeps the ToolDef → store coupling 1-direction:
  // ToolDef → PaletteActions → daemon).
  clearActiveIf(sessionId: string): void;
}

// ---------------------------------------------------------------------------
// Initial state
// ---------------------------------------------------------------------------

// initialClosedState is the state we reset to on close() and on initial
// store construction. selectedToolId / paramValues / paramCursor / query /
// error are scrubbed so a re-open from a stale half-filled form starts fresh
// (FR-029 says re-open is idempotent on already-open; this is the SHAPE we
// converge to on close-then-open).
const initialClosedState: Pick<
  PaletteState,
  | "open"
  | "phase"
  | "scope"
  | "selectedToolId"
  | "paramValues"
  | "paramCursor"
  | "query"
  | "composing"
  | "submitting"
  | "error"
  | "opener"
> = {
  open: false,
  phase: "toolSelect",
  scope: "standard",
  selectedToolId: null,
  paramValues: {},
  paramCursor: 0,
  query: "",
  composing: false,
  submitting: false,
  error: null,
  opener: null,
};

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------
//
// Pure helpers (initialScope, findToolForSubmit, isParamless, isApiHttpError,
// messageOf) live in ./palette_helpers.ts so this file stays under the
// 500-line limit; their bodies + rationale comments are unchanged.

export const usePaletteStore = create<PaletteState & PaletteActions>()((set, get) => ({
  ...initialClosedState,
  refocusSeq: 0,

  openPalette(opts) {
    const state = get();
    // FR-029 idempotency: a second openPalette while already open is a no-op
    // for state. Callers that want "re-focus the input" should call
    // refocusInput() explicitly — separating those two responsibilities lets
    // the hotkey handler call openPalette() unconditionally on Cmd/Ctrl+K
    // without disturbing in-progress param-select work.
    if (state.open) return;
    const scope = initialScope(opts?.daemonSnapshot);
    // preselectToolId (ADR-0043 / FR-021): when caller omits daemonSnapshot
    // we fall back to a minimal empty snapshot — standard tools (new-session,
    // stop-session) register without daemon state, so the lookup still works.
    // Push tools cannot be preselected without a snapshot by design (the New
    // Session use case is standard scope).
    if (opts?.preselectToolId !== undefined) {
      const snap: DaemonSnapshot = opts.daemonSnapshot ?? {
        sessions: [],
        activeSessionID: null,
        projects: [],
        pushCommands: [],
      };
      const tool = listTools(snap, snap.pushCommands).find(
        (t) => t.id === opts.preselectToolId && t.scope === scope,
      );
      if (tool) {
        set({
          ...initialClosedState,
          open: true,
          scope,
          opener: opts?.opener ?? null,
          phase: "paramSelect",
          selectedToolId: tool.id,
        });
        return;
      }
      // Unknown / wrong-scope preselect id is a contract miss but not a
      // user-facing error: fall through to the unfiltered toolSelect open
      // and leave a console.warn so the regression is traceable.
      console.warn("[palette] openPalette preselectToolId did not resolve", {
        preselectToolId: opts.preselectToolId,
        scope,
      });
    }
    set({
      ...initialClosedState,
      open: true,
      scope,
      opener: opts?.opener ?? null,
    });
  },

  close() {
    // Reset all fields except refocusSeq (which is a counter — resetting it
    // would let a stale "please focus" signal fire on the next open). We do
    // NOT increment refocusSeq here: close means "stop interacting with the
    // input"; the CommandPalette unmount effect handles opener.focus().
    set({ ...initialClosedState });
  },

  back() {
    const state = get();
    if (state.phase === "paramSelect") {
      // From paramSelect we step one phase back to toolSelect (FR-017) and
      // wipe the half-filled param form. query is intentionally preserved so
      // the user lands back on the same filtered tool list they came from —
      // re-typing the search would be hostile to fat-finger Esc.
      set({
        phase: "toolSelect",
        selectedToolId: null,
        paramValues: {},
        paramCursor: 0,
        error: null,
      });
      return;
    }
    // From toolSelect, back closes the palette (FR-017). Same code path as
    // close() — kept as a single set() rather than a recursive call so the
    // intent ("back = close at the top level") is visible at the action site.
    set({ ...initialClosedState });
  },

  setScope(scope) {
    // Scope change always returns to the tool-select phase and drops the
    // in-progress param form: the new scope has a different tool list, so any
    // pending selectedToolId is by definition stale. paramCursor / query are
    // reset because the ToolSelectPhase rebinds the listbox to a new source.
    set({
      scope,
      phase: "toolSelect",
      selectedToolId: null,
      paramValues: {},
      paramCursor: 0,
      query: "",
      error: null,
    });
  },

  setQuery(query) {
    // FR-019: IME composition pre-empts all input writes. We honor it here so
    // the CommandPalette can wire input.onChange → setQuery without a
    // per-event composition check. cursor resets to 0 so the new top result
    // of the freshly-filtered list is highlighted (matches TUI palette UX).
    if (get().composing) return;
    set({ query, paramCursor: 0 });
  },

  moveCursor(delta) {
    // FR-019: same IME gate as setQuery. Clamping to the actual list length is
    // the phase component's job (only it knows the current filtered list
    // size); the store holds an unbounded integer that the phase clamps on
    // read. Keeping clamp here would require the store to know about
    // tools-registry / fuzzy filtering, which violates ADR-0036.
    if (get().composing) return;
    set((s) => ({ paramCursor: s.paramCursor + delta }));
  },

  confirmTool(id, ctx) {
    // FR-019: IME guard. A confirm key (Enter) is meaningful to IME as
    // "commit current composition", not "submit form"; routing it as confirm
    // would double-fire.
    const state = get();
    if (state.composing) return;

    // toParamSelect captures the common "advance to paramSelect for id"
    // transition used by both the ctx-bearing params-tool branch and the
    // no-ctx optimistic branch — keeps the action body free of duplicated
    // set({...}) blocks.
    const toParamSelect = () =>
      set({
        phase: "paramSelect",
        selectedToolId: id,
        paramValues: {},
        paramCursor: 0,
        error: null,
      });

    // ctx is the source of truth for the tool list when present. When ctx
    // is supplied we fail-fast on an unknown id so the failure surfaces at
    // the call site (registry diff / typo / push-tool expiry) instead of
    // being laundered into a generic submit-time error one frame later.
    if (ctx) {
      const tool = findToolForSubmit(ctx, id);
      if (tool === null) {
        // Unknown tool id with ctx: hard contract break. Surface loudly via
        // notify.error (user signal) + console.error (devtools/Sentry
        // attribution), leave state untouched (no fake paramSelect).
        ctx.notify.error(`内部エラー: ツール '${id}' が見つかりません (scope=${state.scope})`);
        console.error("[palette] confirmTool unknown id", { id, scope: state.scope });
        return;
      }
      if (isParamless(tool)) {
        // FR-010 fast path: a paramless tool's confirm IS its submit. We
        // briefly transition to paramSelect so submitting=true renders
        // against a consistent phase before submit's close() collapses it.
        toParamSelect();
        return get().submit(ctx);
      }
      toParamSelect();
      return undefined;
    }

    // No ctx: cannot validate id or detect paramless. React layer uses this
    // when driving confirm → paramSelect without firing submit; we leave a
    // warn breadcrumb so a later submit-time failure on a bogus id is
    // correlatable with this confirm.
    console.warn("[palette] confirmTool without ctx; id not validated", {
      id,
      scope: state.scope,
    });
    toParamSelect();
    return undefined;
  },

  setParam(key, value) {
    set((s) => ({ paramValues: { ...s.paramValues, [key]: value } }));
  },

  toggleWorktree() {
    set((s) => {
      const prev = s.paramValues.worktree;
      const next = prev !== true;
      return { paramValues: { ...s.paramValues, worktree: next } };
    });
  },

  toggleHost() {
    // ADR-0042: the in-UI "host" toggle is a boolean; readSandboxFromHostToggle
    // (in lib/tools) projects it onto the wire's `sandbox: "host"` field.
    // We store it as boolean here (true ↔ false), letting the wire mapping
    // stay in the ToolDef.submit layer so the store has zero wire-format
    // knowledge.
    set((s) => {
      const prev = s.paramValues.host;
      const next = prev !== true;
      return { paramValues: { ...s.paramValues, host: next } };
    });
  },

  setComposing(composing) {
    set({ composing });
  },

  async submit(ctx) {
    const state = get();
    // FR-019 IME guard: submit is the last gate before HTTP, so the cost of
    // a misclassified "submit" during composition is the loudest (a network
    // request fired for an IME commit). console.debug leaves a breadcrumb so
    // the drop is distinguishable from "nothing happened" in devtools.
    if (state.composing) {
      console.debug("[palette] submit() suppressed while composing=true (FR-019)", {
        selectedToolId: state.selectedToolId,
      });
      return;
    }
    // Closed-palette guard: state.error is invisible when open=false, so
    // the submit has no surface to land on. console.warn keeps the bogus
    // call traceable instead of silently no-oping.
    if (!state.open) {
      console.warn("[palette] submit() called while closed; ignoring", {
        selectedToolId: state.selectedToolId,
        scope: state.scope,
      });
      return;
    }
    // Re-entry guard: a second submit while one is in-flight would race the
    // close / error transitions. The UI should disable the submit affordance
    // during submitting; this also catches programmatic / test mis-sequencing.
    if (state.submitting) {
      console.debug("[palette] submit() re-entry while submitting=true; dropped", {
        selectedToolId: state.selectedToolId,
      });
      return;
    }

    // The resolution pipeline (findToolForSubmit + disabledReason) runs
    // inside the try so a synchronous throw from either (e.g. a push-tool
    // predicate that dereferences sessions[0] without guard) is routed
    // through the unified non-HTTP error branch instead of escaping as an
    // unhandledrejection without notify / state.error / submitting reset.
    set({ submitting: true, error: null });
    try {
      const tool = findToolForSubmit(ctx, state.selectedToolId);
      if (tool === null) {
        // selectedToolId is null or the id no longer resolves (push tool
        // list changed between confirm and submit). Notify + console.error
        // + full close so a follow-up Cmd+K starts clean.
        const idForUser = state.selectedToolId ?? "なし";
        ctx.notify.error(
          `内部エラー: 選択中ツール (${idForUser}) が見つかりません (scope=${state.scope})`,
        );
        console.error("[palette] submit() unresolved tool", {
          selectedToolId: state.selectedToolId,
          scope: state.scope,
        });
        set({ ...initialClosedState });
        return;
      }

      // FR-023: re-evaluate disabledReason at submit time. The user may
      // have opened the palette in a valid state but the daemon snapshot
      // moved before confirm — push tools whose active session changed
      // are the canonical case. ADR-0047 makes this the single source of
      // truth shared with ScopeSegment's disabled rendering.
      const reason = tool.disabledReason(ctx.daemon);
      if (reason !== null) {
        ctx.notify.error(reason);
        // close() resets the entire state including selectedToolId, so
        // the next open starts clean even if the disabledReason path
        // tripped after a long-lived palette session.
        set({ ...initialClosedState });
        return;
      }

      await tool.submit(ctx, state.paramValues);
      // tool.submit may have already called ctx.store.close()
      // (newSessionTool does) — close is idempotent because
      // set({ ...initialClosedState }) is the same shape twice. Calling
      // it here unconditionally makes the happy path independent of
      // whether the tool's own close() ran.
      set({ ...initialClosedState });
    } catch (e: unknown) {
      // classifySubmitError (palette_helpers) maps an unknown thrown value
      // onto a 3-arm discriminated union; each arm has its own side-effect
      // recipe per FR-024 + ADR-0046. Keeping the recipes inline here (vs.
      // pushing them into the helper) preserves the store-as-only-writer
      // invariant — set() never leaves this file.
      const branch = classifySubmitError(e);
      switch (branch.kind) {
        case "auth":
          // 401 → fixed Japanese toast and full close. Re-auth happens
          // out-of-band (URL #token=…); leaving the palette open would
          // suggest "try again here", which is wrong.
          ctx.notify.error("認証エラー (再ログインしてください)");
          set({ ...initialClosedState });
          return;
        case "http":
          // Server-validated 4xx (non-401) / 5xx → inline-error treatment
          // so the user can correct the input and retry without losing
          // the form. No notify.error: the server message is actionable
          // inline and doubling it as a toast is noisy.
          set({ error: branch.message, submitting: false });
          return;
        case "unknown":
          // Synchronous bugs inside ToolDef.submit / findToolForSubmit /
          // tool.disabledReason, network failures, wire-format errors.
          // These are NOT user-actionable, so we route through
          // notify.error AND console.error to capture both user-visible
          // signal and developer stack context, plus inline error for
          // anyone still looking at the open palette.
          console.error("[palette] submit() non-HTTP error", branch.cause);
          ctx.notify.error(`予期しないエラー: ${branch.message}`);
          set({ error: branch.message, submitting: false });
          return;
      }
    }
  },

  refocusInput() {
    // FR-029: incrementing the counter is the entire signal. CommandPalette's
    // useEffect([refocusSeq]) handles the actual input.focus() call.
    set((s) => ({ refocusSeq: s.refocusSeq + 1 }));
  },

  clearActiveIf(sessionId) {
    // We read daemon state directly (instead of having the caller pass it)
    // because the only legitimate caller is ToolDef.submit which already
    // proved it knows the right sessionId. The daemon store's selectSession
    // is the single-writer for activeSessionID; going through it preserves
    // any future invariants daemon.ts grows.
    const daemon = useDaemonStore.getState();
    if (daemon.activeSessionID === sessionId) {
      daemon.selectSession(null);
    }
  },
}));
