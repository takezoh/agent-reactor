import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiHttpError, SessionsApi } from "../api/sessions";
import type {
  DaemonSnapshot,
  NotificationsApi,
  ToolCtx,
  ToolDaemonActions,
  ToolDef,
  ToolStoreCtx,
} from "../lib/tools";
import * as toolsModule from "../lib/tools";
import { useDaemonStore } from "./daemon";
import { usePaletteStore } from "./palette";

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

function makeFakeHttp(overrides: Partial<SessionsApi> = {}): SessionsApi {
  return {
    createSession: vi.fn().mockResolvedValue({ id: "sess-new" }),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    pushCommand: vi.fn().mockResolvedValue(undefined),
    getSessionConfig: vi.fn().mockResolvedValue({
      projectRoots: [],
      projectPaths: [],
      projects: [],
      commands: [],
      pushCommands: [],
    }),
    ...overrides,
  };
}

function makeFakeNotify(): NotificationsApi & {
  successCalls: string[];
  errorCalls: string[];
} {
  const successCalls: string[] = [];
  const errorCalls: string[] = [];
  return {
    success(m) {
      successCalls.push(m);
    },
    error(m) {
      errorCalls.push(m);
    },
    successCalls,
    errorCalls,
  };
}

function makeFakeStoreActions(): ToolStoreCtx & {
  closeCalls: number;
  clearActiveIfCalls: string[];
} {
  const state = { closeCalls: 0, clearActiveIfCalls: [] as string[] };
  return {
    close() {
      state.closeCalls += 1;
    },
    clearActiveIf(id) {
      state.clearActiveIfCalls.push(id);
    },
    get closeCalls() {
      return state.closeCalls;
    },
    get clearActiveIfCalls() {
      return state.clearActiveIfCalls;
    },
  };
}

// makeFakeDaemonActions records selectSession calls so the FR-021 wiring
// (palette submit -> new-session.submit -> daemonActions.selectSession) is
// observable from the store tests without touching the real daemon store.
function makeFakeDaemonActions(): ToolDaemonActions & {
  selectSessionCalls: Array<string | null>;
} {
  const calls: Array<string | null> = [];
  return {
    selectSession(id) {
      calls.push(id);
    },
    get selectSessionCalls() {
      return calls;
    },
  };
}

function makeDaemonSnapshot(overrides: Partial<DaemonSnapshot> = {}): DaemonSnapshot {
  return {
    sessions: [],
    activeSessionID: null,
    projects: [],
    pushCommands: [],
    ...overrides,
  };
}

function makeCtx(overrides: Partial<ToolCtx> = {}): ToolCtx {
  return {
    http: makeFakeHttp(),
    daemon: makeDaemonSnapshot(),
    daemonActions: makeFakeDaemonActions(),
    notify: makeFakeNotify(),
    store: makeFakeStoreActions(),
    ...overrides,
  };
}

function makeHttpError(status: number, message = `HTTP ${status}`): ApiHttpError {
  const err = new Error(message) as ApiHttpError;
  err.status = status;
  return err;
}

// resetStore clears the palette singleton between tests. We do this by
// calling close() (which resets every field via initialClosedState) — going
// through the action keeps "what `close()` does" testable here too rather
// than rebuilding the reset shape in a fixture.
function resetPalette() {
  usePaletteStore.getState().close();
  // close() preserves refocusSeq; reset it to 0 explicitly so the increment
  // assertions start from a known baseline.
  usePaletteStore.setState({ refocusSeq: 0 });
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe("usePaletteStore", () => {
  beforeEach(() => {
    resetPalette();
    useDaemonStore.getState().reset();
  });

  // -------------------------------------------------------------------------
  // initial state
  // -------------------------------------------------------------------------

  it("has the expected initial state", () => {
    const s = usePaletteStore.getState();
    expect(s.open).toBe(false);
    expect(s.phase).toBe("toolSelect");
    expect(s.scope).toBe("standard");
    expect(s.selectedToolId).toBeNull();
    expect(s.paramValues).toEqual({});
    expect(s.paramCursor).toBe(0);
    expect(s.query).toBe("");
    expect(s.composing).toBe(false);
    expect(s.submitting).toBe(false);
    expect(s.error).toBeNull();
    expect(s.opener).toBeNull();
    expect(s.refocusSeq).toBe(0);
  });

  // -------------------------------------------------------------------------
  // openPalette: scope selection (FR-004) + idempotency (FR-029)
  // -------------------------------------------------------------------------

  it("openPalette picks 'push' when active session has frame occupant (FR-004)", () => {
    usePaletteStore.getState().openPalette({
      daemonSnapshot: makeDaemonSnapshot({
        activeSessionID: "s1",
        activeOccupant: "frame",
      }),
    });
    const s = usePaletteStore.getState();
    expect(s.open).toBe(true);
    expect(s.scope).toBe("push");
  });

  it("openPalette picks 'standard' when active session occupant is not 'frame'", () => {
    usePaletteStore.getState().openPalette({
      daemonSnapshot: makeDaemonSnapshot({
        activeSessionID: "s1",
        activeOccupant: "main",
      }),
    });
    expect(usePaletteStore.getState().scope).toBe("standard");
  });

  it("openPalette picks 'standard' when no active session is present", () => {
    usePaletteStore.getState().openPalette({
      daemonSnapshot: makeDaemonSnapshot({ activeSessionID: null }),
    });
    expect(usePaletteStore.getState().scope).toBe("standard");
  });

  it("openPalette picks 'standard' when no daemonSnapshot is provided", () => {
    usePaletteStore.getState().openPalette();
    expect(usePaletteStore.getState().scope).toBe("standard");
    expect(usePaletteStore.getState().open).toBe(true);
  });

  it("openPalette is a no-op when already open (FR-029)", () => {
    // First open: push scope.
    usePaletteStore.getState().openPalette({
      daemonSnapshot: makeDaemonSnapshot({
        activeSessionID: "s1",
        activeOccupant: "frame",
      }),
    });
    // User navigates into paramSelect.
    usePaletteStore.setState({
      phase: "paramSelect",
      selectedToolId: "new-session",
      paramValues: { project: "/p" },
    });
    // Second open with a snapshot that WOULD pick 'standard' must not
    // overwrite the in-progress paramSelect state.
    usePaletteStore.getState().openPalette({
      daemonSnapshot: makeDaemonSnapshot({ activeSessionID: null }),
    });
    const s = usePaletteStore.getState();
    expect(s.scope).toBe("push");
    expect(s.phase).toBe("paramSelect");
    expect(s.selectedToolId).toBe("new-session");
    expect(s.paramValues).toEqual({ project: "/p" });
  });

  it("openPalette stores the opener element verbatim", () => {
    // We don't need a real DOM; the store only stashes the reference.
    const opener = { id: "stub-opener" } as unknown as HTMLElement;
    usePaletteStore.getState().openPalette({ opener });
    expect(usePaletteStore.getState().opener).toBe(opener);
  });

  // -------------------------------------------------------------------------
  // close / back (FR-017)
  // -------------------------------------------------------------------------

  it("close resets every field except refocusSeq", () => {
    usePaletteStore.getState().refocusInput(); // bumps refocusSeq to 1
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ phase: "paramSelect", selectedToolId: "x" });
    usePaletteStore.getState().close();
    const s = usePaletteStore.getState();
    expect(s.open).toBe(false);
    expect(s.phase).toBe("toolSelect");
    expect(s.selectedToolId).toBeNull();
    expect(s.opener).toBeNull();
    expect(s.refocusSeq).toBe(1); // preserved across close
  });

  it("back from paramSelect returns to toolSelect and clears paramValues / selectedToolId", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({
      phase: "paramSelect",
      selectedToolId: "new-session",
      paramValues: { project: "/p", command: "claude" },
      paramCursor: 3,
      query: "ses",
      error: "stale error",
    });
    usePaletteStore.getState().back();
    const s = usePaletteStore.getState();
    expect(s.phase).toBe("toolSelect");
    expect(s.selectedToolId).toBeNull();
    expect(s.paramValues).toEqual({});
    expect(s.paramCursor).toBe(0);
    // query is preserved on back (UX: user lands back on filtered list)
    expect(s.query).toBe("ses");
    expect(s.error).toBeNull();
    expect(s.open).toBe(true);
  });

  it("back from toolSelect closes the palette (FR-017)", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ query: "ses" });
    usePaletteStore.getState().back();
    expect(usePaletteStore.getState().open).toBe(false);
    expect(usePaletteStore.getState().query).toBe("");
  });

  // -------------------------------------------------------------------------
  // setScope / setQuery / moveCursor + IME guard (FR-019)
  // -------------------------------------------------------------------------

  it("setScope resets phase, paramValues, cursor, query, error", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({
      phase: "paramSelect",
      selectedToolId: "new-session",
      paramValues: { project: "/p" },
      paramCursor: 2,
      query: "ses",
      error: "x",
    });
    usePaletteStore.getState().setScope("push");
    const s = usePaletteStore.getState();
    expect(s.scope).toBe("push");
    expect(s.phase).toBe("toolSelect");
    expect(s.selectedToolId).toBeNull();
    expect(s.paramValues).toEqual({});
    expect(s.paramCursor).toBe(0);
    expect(s.query).toBe("");
    expect(s.error).toBeNull();
  });

  it("setQuery is a no-op while composing=true (FR-019)", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ query: "abc", paramCursor: 5, composing: true });
    usePaletteStore.getState().setQuery("xyz");
    expect(usePaletteStore.getState().query).toBe("abc");
    expect(usePaletteStore.getState().paramCursor).toBe(5);
  });

  it("setQuery updates query and resets cursor when composing=false", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ paramCursor: 5 });
    usePaletteStore.getState().setQuery("ses");
    expect(usePaletteStore.getState().query).toBe("ses");
    expect(usePaletteStore.getState().paramCursor).toBe(0);
  });

  it("moveCursor is a no-op while composing=true (FR-019)", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ paramCursor: 2, composing: true });
    usePaletteStore.getState().moveCursor(1);
    expect(usePaletteStore.getState().paramCursor).toBe(2);
  });

  it("moveCursor adds the delta (clamp is the phase component's job)", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.getState().moveCursor(3);
    expect(usePaletteStore.getState().paramCursor).toBe(3);
    usePaletteStore.getState().moveCursor(-1);
    expect(usePaletteStore.getState().paramCursor).toBe(2);
  });

  // -------------------------------------------------------------------------
  // confirmTool (FR-010 paramless fast path, FR-019 IME guard)
  // -------------------------------------------------------------------------

  it("confirmTool is a no-op while composing=true (FR-019)", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.setState({ composing: true });
    usePaletteStore.getState().confirmTool("new-session", makeCtx());
    const s = usePaletteStore.getState();
    expect(s.phase).toBe("toolSelect");
    expect(s.selectedToolId).toBeNull();
  });

  it("confirmTool transitions to paramSelect for a params-bearing tool", () => {
    usePaletteStore.getState().openPalette();
    usePaletteStore.getState().confirmTool("new-session", makeCtx());
    const s = usePaletteStore.getState();
    expect(s.phase).toBe("paramSelect");
    expect(s.selectedToolId).toBe("new-session");
    expect(s.paramValues).toEqual({});
    expect(s.paramCursor).toBe(0);
  });

  it("confirmTool with ctx but unknown id fails fast (notify.error + console.error, no transition)", async () => {
    // Reviewer fix (major): previously confirmTool(id, ctx) would transition
    // to paramSelect for an unknown id and the failure would only surface
    // one frame later inside submit() as a generic "ツールが選択されていません"
    // — losing the attribution of who passed the bogus id. Now we fail-fast
    // with notify.error + console.error and leave state unchanged.
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([]);
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      const notify = makeFakeNotify();
      usePaletteStore.getState().confirmTool("ghost-tool", makeCtx({ notify }));
      const s = usePaletteStore.getState();
      // State NOT advanced to paramSelect — caller's contract break is
      // visible at the call site, not laundered into a later submit().
      expect(s.phase).toBe("toolSelect");
      expect(s.selectedToolId).toBeNull();
      // Toast surfaced with id + scope context.
      expect(notify.errorCalls).toHaveLength(1);
      expect(notify.errorCalls[0]).toContain("ghost-tool");
      expect(notify.errorCalls[0]).toContain("standard");
      // Devtools breadcrumb for attribution.
      expect(errSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      errSpy.mockRestore();
    }
  });

  it("confirmTool without ctx accepts the id optimistically with console.warn breadcrumb", async () => {
    // Reviewer fix (major, secondary): the no-ctx path is reached when the
    // React layer drives confirm → paramSelect transition without firing
    // submit. We cannot validate the id without ctx, but we leave a warn
    // breadcrumb so the eventual submit-time failure (if the id is bogus)
    // is correlatable with this confirm.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.getState().confirmTool("new-session");
      const s = usePaletteStore.getState();
      expect(s.phase).toBe("paramSelect");
      expect(s.selectedToolId).toBe("new-session");
      expect(warnSpy).toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("confirmTool fires submit immediately for a paramless tool (FR-010)", async () => {
    // Mock listTools to return a single paramless tool. This keeps the test
    // independent of the real registry's shape — only the "params=null →
    // immediate submit" wiring is under test.
    const paramless: ToolDef = {
      id: "shutdown-stub",
      label: "stub",
      scope: "standard",
      params: null,
      disabledReason: () => null,
      submit: vi.fn().mockResolvedValue(undefined),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([paramless]);
    try {
      usePaletteStore.getState().openPalette();
      const ctx = makeCtx();
      await usePaletteStore.getState().confirmTool("shutdown-stub", ctx);
      // submit ran (paramless fast path) and palette closed on success.
      expect(paramless.submit).toHaveBeenCalledTimes(1);
      expect(usePaletteStore.getState().open).toBe(false);
    } finally {
      spy.mockRestore();
    }
  });

  // -------------------------------------------------------------------------
  // setParam / toggleWorktree / toggleHost
  // -------------------------------------------------------------------------

  it("setParam writes the value into paramValues", () => {
    usePaletteStore.getState().setParam("project", "/p");
    expect(usePaletteStore.getState().paramValues.project).toBe("/p");
  });

  it("toggleWorktree flips the boolean (undefined → true → false)", () => {
    usePaletteStore.getState().toggleWorktree();
    expect(usePaletteStore.getState().paramValues.worktree).toBe(true);
    usePaletteStore.getState().toggleWorktree();
    expect(usePaletteStore.getState().paramValues.worktree).toBe(false);
  });

  it("toggleHost flips the boolean (undefined → true → false)", () => {
    usePaletteStore.getState().toggleHost();
    expect(usePaletteStore.getState().paramValues.host).toBe(true);
    usePaletteStore.getState().toggleHost();
    expect(usePaletteStore.getState().paramValues.host).toBe(false);
  });

  // -------------------------------------------------------------------------
  // setComposing
  // -------------------------------------------------------------------------

  it("setComposing toggles the IME guard flag", () => {
    usePaletteStore.getState().setComposing(true);
    expect(usePaletteStore.getState().composing).toBe(true);
    usePaletteStore.getState().setComposing(false);
    expect(usePaletteStore.getState().composing).toBe(false);
  });

  // -------------------------------------------------------------------------
  // submit (FR-020, FR-023, FR-024)
  // -------------------------------------------------------------------------

  it("submit succeeds, closes palette, clears error", async () => {
    const submit = vi.fn().mockResolvedValue(undefined);
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit,
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      await usePaletteStore.getState().submit(makeCtx());
      expect(submit).toHaveBeenCalledTimes(1);
      const s = usePaletteStore.getState();
      expect(s.open).toBe(false);
      expect(s.error).toBeNull();
      expect(s.submitting).toBe(false);
    } finally {
      spy.mockRestore();
    }
  });

  it("submit on 4xx (non-401) keeps palette open and surfaces inline error (FR-024)", async () => {
    const httpErr = makeHttpError(400, "bad request");
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit: vi.fn().mockRejectedValue(httpErr),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      expect(s.open).toBe(true);
      expect(s.error).toBe("bad request");
      expect(s.submitting).toBe(false);
      // 4xx does NOT route through notify.error (inline error is the SSoT)
      expect(notify.errorCalls).toEqual([]);
    } finally {
      spy.mockRestore();
    }
  });

  it("submit on 413 (push body too large) keeps palette open with inline error and no toast (T4)", async () => {
    // Reviewer fix (major T4): the push route returns 413 when the body
    // exceeds MaxBytesReader (1 MiB). The store must treat 413 like the
    // 4xx-non-401 branch — inline error stays visible, palette stays
    // open so the user can shorten the command, and we do NOT emit a
    // generic toast (the inline error IS the actionable signal). This
    // is exactly the contract palette already promises for 4xx; the
    // test pins 413 specifically so a future "413 needs special handling"
    // refactor cannot silently regress to closing the palette.
    const httpErr = makeHttpError(413, "request body too large");
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "push",
      params: [],
      disabledReason: () => null,
      submit: vi.fn().mockRejectedValue(httpErr),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      // Palette stays open (user can correct + retry).
      expect(s.open).toBe(true);
      // Inline error carries the server message verbatim.
      expect(s.error).toBe("request body too large");
      expect(s.submitting).toBe(false);
      // No toast — inline error is the SSoT for HTTP failures (FR-024).
      expect(notify.errorCalls).toEqual([]);
    } finally {
      spy.mockRestore();
    }
  });

  it("submit on 401 closes palette and fires auth toast (FR-024)", async () => {
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit: vi.fn().mockRejectedValue(makeHttpError(401, "unauthorized")),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      expect(s.open).toBe(false);
      expect(notify.errorCalls).toEqual(["認証エラー (再ログインしてください)"]);
    } finally {
      spy.mockRestore();
    }
  });

  it("submit with disabledReason non-null skips HTTP, fires toast, closes (FR-023)", async () => {
    const submitFn = vi.fn().mockResolvedValue(undefined);
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "push",
      params: [],
      disabledReason: () => "push 対象 driver なし",
      submit: submitFn,
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      expect(submitFn).not.toHaveBeenCalled();
      expect(notify.errorCalls).toEqual(["push 対象 driver なし"]);
      expect(usePaletteStore.getState().open).toBe(false);
    } finally {
      spy.mockRestore();
    }
  });

  it("submit with no selectedToolId closes palette and routes notify.error (silent-failure fix)", async () => {
    // Reviewer fix: previously the no-selectedToolId branch wrote a generic
    // inline error and left the palette open. That branch is reached only
    // via a contract break (UI let user submit before confirming a tool, or
    // the tool list churned between confirm and submit), and inline-only
    // surfacing was invisible if the palette was already closed. Now we:
    //   - fire notify.error with id + scope context,
    //   - log to console.error for devtools / Sentry,
    //   - reset state fully (close palette).
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      // selectedToolId stays null
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      // Palette closed and state fully reset.
      expect(s.open).toBe(false);
      expect(s.error).toBeNull();
      // Toast surfaced with context. We surface "なし" (not the literal
      // token "null") for the user-facing missing-id case so the toast
      // reads as a Japanese error message rather than a developer
      // artifact; the precise null is still in the console.error
      // breadcrumb for attribution.
      expect(notify.errorCalls).toHaveLength(1);
      expect(notify.errorCalls[0]).toContain("ツール");
      expect(notify.errorCalls[0]).toContain("なし");
      expect(notify.errorCalls[0]).not.toContain("null");
      expect(notify.errorCalls[0]).toContain("standard");
      // Devtools breadcrumb for attribution.
      expect(errSpy).toHaveBeenCalled();
    } finally {
      errSpy.mockRestore();
    }
  });

  it("submit when palette is closed is a no-op with console.warn breadcrumb", async () => {
    // Reviewer fix: a programmatic submit() while open=false used to write
    // an inline error nobody could see. We now drop with a warn breadcrumb
    // so the caller mistake is traceable, and we do NOT mutate state.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      // palette starts closed (resetPalette in beforeEach)
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      expect(s.open).toBe(false);
      expect(s.error).toBeNull();
      expect(notify.errorCalls).toEqual([]);
      expect(warnSpy).toHaveBeenCalled();
    } finally {
      warnSpy.mockRestore();
    }
  });

  it("submit non-HTTP error (synchronous bug / network failure) fires notify.error + console.error", async () => {
    // Reviewer fix: previously every non-401 throw landed in the same
    // inline-error branch as HTTP 4xx. A synchronous bug inside
    // ToolDef.submit (TypeError / ReferenceError) or a network failure
    // (TypeError "Failed to fetch") would leave only an inline string with
    // no toast and no Sentry-bound log. Now non-HTTP failures get
    // notify.error + console.error in addition to inline error, while HTTP
    // failures still stay inline-only.
    const bug = new TypeError("Failed to fetch");
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit: vi.fn().mockRejectedValue(bug),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await usePaletteStore.getState().submit(makeCtx({ notify }));
      const s = usePaletteStore.getState();
      // Inline error preserved for visible palettes.
      expect(s.error).toBe("Failed to fetch");
      expect(s.submitting).toBe(false);
      // Palette stays open so the user can see the error inline.
      expect(s.open).toBe(true);
      // Toast surfaced (non-HTTP must not be silent).
      expect(notify.errorCalls).toHaveLength(1);
      expect(notify.errorCalls[0]).toContain("予期しないエラー");
      expect(notify.errorCalls[0]).toContain("Failed to fetch");
      // Devtools breadcrumb for stack trace.
      expect(errSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      errSpy.mockRestore();
    }
  });

  it("submit is suppressed while composing=true (FR-019 + debug breadcrumb)", async () => {
    // Reviewer fix (major): the FR-019 IME guard previously covered
    // setQuery / moveCursor / confirmTool but NOT submit. An Enter that
    // commits an IME composition could bypass confirmTool (e.g. Enter on
    // the paramSelect form's submit button while still composing) and
    // fire tool.submit + a network request. Submit now mirrors the
    // other actions: composing=true → drop with a console.debug
    // breadcrumb. Keeps the contract that NOTHING in the input pipeline
    // mutates state or fires HTTP while the IME is mid-composition.
    const submit = vi.fn().mockResolvedValue(undefined);
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit,
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    const debugSpy = vi.spyOn(console, "debug").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({
        selectedToolId: "t",
        phase: "paramSelect",
        composing: true,
      });
      await usePaletteStore.getState().submit(makeCtx());
      // tool.submit must NOT fire while composing.
      expect(submit).not.toHaveBeenCalled();
      // State unchanged (still open, no error written, not submitting).
      const s = usePaletteStore.getState();
      expect(s.open).toBe(true);
      expect(s.error).toBeNull();
      expect(s.submitting).toBe(false);
      // Suppression is breadcrumbed so devtools can distinguish "guard
      // fired" from "nothing happened".
      expect(debugSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      debugSpy.mockRestore();
    }
  });

  it("submit routes a synchronous throw in tool.disabledReason through the non-HTTP error branch", async () => {
    // Reviewer fix (minor): tool.disabledReason is implementer-supplied
    // and may have bugs (e.g. a push tool reading sessions[0].name
    // without guarding for empty sessions). Previously this throw was
    // OUTSIDE submit()'s try block and propagated as an
    // unhandledrejection, bypassing the carefully designed user-feedback
    // path. The resolution pipeline (findToolForSubmit +
    // tool.disabledReason) is now inside the try, so a synchronous
    // throw lands in the non-HTTP error branch: notify.error +
    // console.error + inline state.error + submitting=false.
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "push",
      params: [],
      disabledReason: () => {
        throw new TypeError("Cannot read properties of undefined (reading 'name')");
      },
      submit: vi.fn().mockResolvedValue(undefined),
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      // submit() must NOT throw out — the contract is that all failure
      // modes surface visibly through notify.error / state.error.
      await expect(usePaletteStore.getState().submit(makeCtx({ notify }))).resolves.toBeUndefined();
      const s = usePaletteStore.getState();
      // tool.submit was NOT called (we threw during disabledReason).
      expect(tool.submit).not.toHaveBeenCalled();
      // Inline error preserved for visible palettes.
      expect(s.error).toContain("Cannot read properties of undefined");
      expect(s.submitting).toBe(false);
      // Palette stays open so the user sees the inline error.
      expect(s.open).toBe(true);
      // Toast surfaced through the non-HTTP path (loudest, not silent).
      expect(notify.errorCalls).toHaveLength(1);
      expect(notify.errorCalls[0]).toContain("予期しないエラー");
      // Devtools breadcrumb for stack attribution.
      expect(errSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      errSpy.mockRestore();
    }
  });

  it("submit routes a synchronous throw in listTools through the non-HTTP error branch", async () => {
    // Reviewer fix (minor, sibling of the disabledReason case): if
    // listTools (called inside findToolForSubmit) throws synchronously
    // (e.g. a registry bug on a malformed daemon snapshot), the throw
    // must NOT escape submit() as an unhandledrejection. The try now
    // wraps findToolForSubmit so the throw lands in the unified
    // non-HTTP error branch.
    const spy = vi.spyOn(toolsModule, "listTools").mockImplementation(() => {
      throw new Error("registry boom");
    });
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({ selectedToolId: "t", phase: "paramSelect" });
      const notify = makeFakeNotify();
      await expect(usePaletteStore.getState().submit(makeCtx({ notify }))).resolves.toBeUndefined();
      const s = usePaletteStore.getState();
      expect(s.error).toBe("registry boom");
      expect(s.submitting).toBe(false);
      expect(s.open).toBe(true);
      expect(notify.errorCalls).toHaveLength(1);
      expect(notify.errorCalls[0]).toContain("予期しないエラー");
      expect(notify.errorCalls[0]).toContain("registry boom");
      expect(errSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      errSpy.mockRestore();
    }
  });

  it("submit while submitting=true short-circuits (silent drop by design + debug breadcrumb)", async () => {
    // Reviewer fix (minor): the silent drop is intentional (UI disables the
    // submit affordance during submitting=true), but we now leave a
    // console.debug breadcrumb so the dropped re-entry is distinguishable
    // from a no-op during debugging.
    const submit = vi.fn().mockResolvedValue(undefined);
    const tool: ToolDef = {
      id: "t",
      label: "t",
      scope: "standard",
      params: [],
      disabledReason: () => null,
      submit,
    };
    const spy = vi.spyOn(toolsModule, "listTools").mockReturnValue([tool]);
    const debugSpy = vi.spyOn(console, "debug").mockImplementation(() => {});
    try {
      usePaletteStore.getState().openPalette();
      usePaletteStore.setState({
        selectedToolId: "t",
        phase: "paramSelect",
        submitting: true,
      });
      await usePaletteStore.getState().submit(makeCtx());
      expect(submit).not.toHaveBeenCalled();
      expect(debugSpy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
      debugSpy.mockRestore();
    }
  });

  // -------------------------------------------------------------------------
  // refocusInput (FR-029)
  // -------------------------------------------------------------------------

  it("refocusInput increments refocusSeq exactly once per call", () => {
    expect(usePaletteStore.getState().refocusSeq).toBe(0);
    usePaletteStore.getState().refocusInput();
    expect(usePaletteStore.getState().refocusSeq).toBe(1);
    usePaletteStore.getState().refocusInput();
    expect(usePaletteStore.getState().refocusSeq).toBe(2);
  });

  // -------------------------------------------------------------------------
  // clearActiveIf
  // -------------------------------------------------------------------------

  it("clearActiveIf clears daemon activeSessionID when ids match", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    usePaletteStore.getState().clearActiveIf("s1");
    expect(useDaemonStore.getState().activeSessionID).toBeNull();
  });

  it("clearActiveIf is a no-op when ids differ", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    usePaletteStore.getState().clearActiveIf("s2");
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");
  });
});
