// usePaletteStore — pure state + actions for the command palette.
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// ADR-0036: HTTP / DOM stay outside; no document/window/HTMLElement refs.
// ADR-0040: `composing` is the single IME-suppression source of truth.
// ADR-0047: disabledReason re-evaluated at submit time.
// ADR-0055: frozenSnapshot lives in CommandPalette useRef; submitting boolean
//           is the sole freeze-epoch signal in the store.

import { create } from "zustand";
import { type DaemonSnapshot, type ToolCtx, listTools } from "../lib/tools";
import { type ActiveContextSlice, createActiveContextSlice } from "./palette_active_context";
import { classifySubmitError, findToolForSubmit, isParamless } from "./palette_helpers";
import {
  type InlineStatusSlice,
  createInlineStatusSlice,
  initialInlineStatusState,
} from "./palette_inline_status";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type PalettePhase = "toolSelect" | "paramSelect";

export interface OpenPaletteOptions {
  opener?: HTMLElement | null;
  // daemonSnapshot is provided when preselectToolId is set so the store can
  // resolve the tool against the live push-command set without sampling the
  // daemon store itself (keeps the openPalette transition deterministic in
  // unit tests — the test passes whatever snapshot it wants).
  daemonSnapshot?: DaemonSnapshot;
  // preselectToolId pre-selects a tool by id and skips ToolSelectPhase
  // (ADR-0043 / FR-021 — the Header's "New Session" CTA needs to land on
  // 'new-session' regardless of the fuzzy query). When the id resolves we
  // advance to phase='paramSelect'; otherwise we fall back to the unfiltered
  // toolSelect open with a console.warn (the alternative — closing immediately
  // — would leave the CTA feeling broken).
  // Paramless tools are NOT auto-submitted because openPalette has no
  // ToolCtx; the user lands on a 0-field paramSelect whose Enter routes
  // through submit(ctx) normally.
  preselectToolId?: string;
}

export interface PaletteState {
  open: boolean;
  phase: PalettePhase;
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
// Pure helpers (findToolForSubmit, isParamless, isApiHttpError,
// messageOf) live in ./palette_helpers.ts so this file stays under the
// 500-line limit; their bodies + rationale comments are unchanged.

export const usePaletteStore = create<
  PaletteState & PaletteActions & ActiveContextSlice & InlineStatusSlice
>()((set, get, store) => ({
  ...initialClosedState,
  refocusSeq: 0,
  ...createActiveContextSlice(set, get, store),
  ...createInlineStatusSlice(set, get, store),

  openPalette(opts) {
    const state = get();
    // FR-029 idempotency: a second openPalette while already open is a no-op
    // for state. Callers that want "re-focus the input" should call
    // refocusInput() explicitly — separating those two responsibilities lets
    // the hotkey handler call openPalette() unconditionally on Cmd/Ctrl+K
    // without disturbing in-progress param-select work.
    if (state.open) return;
    // preselectToolId (ADR-0043 / FR-021 / FR-A2 / FR-Det): the Header's
    // "New Session" CTA must land on 'new-session'. Resolve against the full
    // ToolDef set (ADR-0050: scope removed — all tools visible at once). We do
    // NOT materialize options or hit HTTP here (ADR-0036 keeps the store
    // DOM/HTTP-free); ParamSelectPhase handles dynamic option materialization
    // at render time.
    if (opts?.preselectToolId !== undefined) {
      const snap: DaemonSnapshot = opts.daemonSnapshot ?? {
        sessions: [],
        activeSessionID: null,
        projects: [],
        pushCommands: [],
      };
      const tool = listTools(snap, snap.pushCommands).find((t) => t.id === opts.preselectToolId);
      if (tool) {
        set({
          ...initialClosedState,
          open: true,
          opener: opts?.opener ?? null,
          phase: "paramSelect",
          selectedToolId: tool.id,
        });
        return;
      }
      // Unknown preselect id is a contract miss but not a user-facing
      // error: fall through to the unfiltered toolSelect open and leave a
      // console.warn so the regression is traceable.
      console.warn("[palette] openPalette preselectToolId did not resolve", {
        preselectToolId: opts.preselectToolId,
      });
    }
    set({
      ...initialClosedState,
      open: true,
      opener: opts?.opener ?? null,
    });
  },

  close() {
    // Reset all fields except refocusSeq (which is a counter — resetting it
    // would let a stale "please focus" signal fire on the next open). We do
    // NOT increment refocusSeq here: close means "stop interacting with the
    // input"; the CommandPalette unmount effect handles opener.focus().
    // activeContextSnapshot is reset to { kind: 'none' }; flashSeq /
    // announceSeq are preserved (monotonic counters, not reset on close).
    // Defensive cleanup: cancel any pending inline-status timer so it does
    // not fire against a closed (possibly re-opened) palette.
    const cur = get().inlineStatus;
    if (cur.timerId !== null) clearTimeout(cur.timerId);
    set({
      ...initialClosedState,
      activeContextSnapshot: { kind: "none" },
      ...initialInlineStatusState,
    });
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
    // activeContextSnapshot reset; flashSeq / announceSeq preserved (monotonic).
    // Defensive cleanup: cancel any pending inline-status timer on close path.
    const cur = get().inlineStatus;
    if (cur.timerId !== null) clearTimeout(cur.timerId);
    set({
      ...initialClosedState,
      activeContextSnapshot: { kind: "none" },
      ...initialInlineStatusState,
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
        ctx.notify.error(`Unknown tool: ${id}`);
        console.error("[palette] confirmTool unknown id", { id });
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
    console.warn("[palette] confirmTool without ctx; id not validated", { id });
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
    // ADR-0055: submitting=true is the freeze-epoch signal; frozenSnapshot
    // lives in CommandPalette useRef and is captured by useFrozenSnapshot hook.
    try {
      const tool = findToolForSubmit(ctx, state.selectedToolId);
      if (tool === null) {
        // selectedToolId is null or stale (push-tool list churned). "none"
        // substitutes for null in the user-facing toast (ADR-0047).
        const idForUser = state.selectedToolId ?? "none";
        ctx.notify.error(`Internal error: selected tool (${idForUser}) not found`);
        console.error("[palette] submit() unresolved tool", {
          selectedToolId: state.selectedToolId,
        });
        set({ ...initialClosedState, activeContextSnapshot: { kind: "none" } });
        return;
      }

      // FR-023 / ADR-0047: re-evaluate disabledReason at submit time.
      const reason = tool.disabledReason(ctx.daemon);
      if (reason !== null) {
        ctx.notify.error(reason);
        set({ ...initialClosedState, activeContextSnapshot: { kind: "none" } });
        return;
      }

      await tool.submit(ctx, state.paramValues);
      // close is idempotent; tools that call ctx.store.close() themselves
      // (e.g. newSessionTool) are safe — the second reset is a no-op.
      set({ ...initialClosedState, activeContextSnapshot: { kind: "none" } });
    } catch (e: unknown) {
      // classifySubmitError maps the throw onto auth / http / unknown.
      // set() stays in this file (store-as-only-writer, ADR-0036).
      const branch = classifySubmitError(e);
      switch (branch.kind) {
        case "auth":
          // 401: full close; re-auth is out-of-band (URL #token=…).
          ctx.notify.error("Authentication required");
          set({ ...initialClosedState, activeContextSnapshot: { kind: "none" } });
          return;
        case "http":
          // 4xx/5xx: inline error only; palette stays open for retry.
          set({ error: branch.message, submitting: false });
          return;
        case "unknown":
          // Sync bugs / network failures: notify + console.error + inline.
          console.error("[palette] submit() non-HTTP error", branch.cause);
          ctx.notify.error(`Unexpected error: ${branch.message}`);
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
}));
