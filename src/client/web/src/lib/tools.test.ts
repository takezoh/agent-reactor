import { describe, expect, it, vi } from "vitest";
import type { SessionsApi } from "../api/sessions";
import type { SessionInfo } from "../wire/server";
import {
  type DaemonSnapshot,
  type NotificationsApi,
  type ToolCtx,
  type ToolDaemonActions,
  type ToolDef,
  type ToolStoreCtx,
  listTools,
  projectOptions,
  scopeDisabledReason,
  sessionOptions,
} from "./tools";

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

function makeFakeHttp(overrides: Partial<SessionsApi> = {}): SessionsApi {
  return {
    createSession: vi.fn().mockResolvedValue({ id: "sess-new" }),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    pushCommand: vi.fn().mockRejectedValue(new Error("not implemented")),
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

function makeFakeStore(): ToolStoreCtx & {
  closeCalls: number;
  clearActiveIfCalls: string[];
} {
  const state = {
    closeCalls: 0,
    clearActiveIfCalls: [] as string[],
  };
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

// makeFakeDaemonActions records selectSession calls so new-session.submit
// (FR-021) can be asserted to invoke the write-side daemon API.
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

function sessionFixture(id: string, title?: string): SessionInfo {
  return {
    id,
    project: "/p",
    command: "claude",
    created_at: "2026-06-24T00:00:00Z",
    view: {
      card: title === undefined ? {} : { title },
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
    store: makeFakeStore(),
    ...overrides,
  };
}

function findTool(tools: ToolDef[], id: string): ToolDef {
  const t = tools.find((x) => x.id === id);
  if (!t) throw new Error(`tool not found: ${id}`);
  return t;
}

// ---------------------------------------------------------------------------
// listTools
// ---------------------------------------------------------------------------

describe("listTools", () => {
  it("returns new-session and stop-session in deterministic order", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    expect(tools.map((t) => t.id)).toEqual(["new-session", "stop-session"]);
  });

  it("never includes a tool with id 'shutdown' as a standard scope entry (FR-028)", () => {
    // FR-028 keeps shutdown out of the curated tool surface: it gets a
    // dedicated confirm modal in palette-store, not a row in the listbox.
    // Even when "shutdown" appears in pushCommands the push expansion may
    // create a ToolDef with id 'push:shutdown' — that's the curated push
    // surface, distinct from the FR-028 standard "shutdown" entry. The
    // assertion targets the standard-scope id specifically.
    const tools = listTools(makeDaemonSnapshot(), ["shutdown", "/quit"]);
    expect(tools.find((t) => t.id === "shutdown")).toBeUndefined();
  });

  it("appends one push ToolDef per pushCommands entry in given order", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save", "commit"]);
    expect(tools.map((t) => t.id)).toEqual([
      "new-session",
      "stop-session",
      "push:save",
      "push:commit",
    ]);
  });

  it("returns only standard tools when pushCommands is empty", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    expect(tools.map((t) => t.id)).toEqual(["new-session", "stop-session"]);
  });

  it("each tool exposes the ToolDef shape", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    for (const t of tools) {
      expect(typeof t.label).toBe("string");
      expect(t.label.length).toBeGreaterThan(0);
      expect(typeof t.disabledReason).toBe("function");
      expect(typeof t.submit).toBe("function");
    }
    // Standard tools have a params array; push tools are paramless (null,
    // FR-010): hitting Enter on the listbox row fires submit() immediately
    // without entering the ParamSelectPhase.
    const newSession = findTool(tools, "new-session");
    expect(Array.isArray(newSession.params)).toBe(true);
    const pushSave = findTool(tools, "push:save");
    expect(pushSave.params).toBeNull();
    expect(pushSave.scope).toBe("push");
  });
});

// ---------------------------------------------------------------------------
// disabledReason
// ---------------------------------------------------------------------------

describe("standard ToolDef.disabledReason", () => {
  it("returns null for new-session regardless of daemon state", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    const t = findTool(tools, "new-session");
    expect(t.disabledReason(makeDaemonSnapshot())).toBeNull();
    expect(
      t.disabledReason(
        makeDaemonSnapshot({
          sessions: [sessionFixture("s1")],
          activeSessionID: "s1",
        }),
      ),
    ).toBeNull();
  });

  it("returns null for stop-session regardless of daemon state", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    const t = findTool(tools, "stop-session");
    expect(t.disabledReason(makeDaemonSnapshot())).toBeNull();
    expect(
      t.disabledReason(
        makeDaemonSnapshot({
          sessions: [sessionFixture("s1")],
        }),
      ),
    ).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// new-session.submit
// ---------------------------------------------------------------------------

describe("new-session.submit", () => {
  it("calls http.createSession with project + command only when toggles absent", async () => {
    const http = makeFakeHttp();
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await tool.submit(ctx, { project: "/repo/a", command: "claude" });
    expect(http.createSession).toHaveBeenCalledTimes(1);
    expect(http.createSession).toHaveBeenCalledWith({
      project: "/repo/a",
      command: "claude",
    });
    expect(notify.successCalls).toEqual(["セッションを作成しました"]);
    expect(store.closeCalls).toBe(1);
  });

  it("FR-021: selects the newly-created session via daemonActions.selectSession(rc.id)", async () => {
    // Reviewer fix (major T3): newSessionTool.submit previously discarded
    // the returned {id} from createSession and never selected the new
    // session. SessionList would stay on whatever was active before the
    // CTA, defeating "New Session" as a one-click flow. After the fix the
    // tool MUST call ctx.daemonActions.selectSession(rc.id) before notify
    // + close so the activeSessionID write lands even if notify throws.
    const http = makeFakeHttp({
      createSession: vi.fn().mockResolvedValue({ id: "sess-just-made" }),
    });
    const daemonActions = makeFakeDaemonActions();
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, daemonActions, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await tool.submit(ctx, { project: "/repo/a", command: "claude" });
    expect(daemonActions.selectSessionCalls).toEqual(["sess-just-made"]);
    expect(notify.successCalls).toEqual(["セッションを作成しました"]);
    expect(store.closeCalls).toBe(1);
  });

  it("does NOT select a new session when createSession rejects (FR-021 guard)", async () => {
    // The select MUST be downstream of a successful create — otherwise a
    // failed POST would still mutate activeSessionID, leaving the UI
    // pointing at a session that does not exist.
    const http = makeFakeHttp({
      createSession: vi.fn().mockRejectedValue(new Error("HTTP 500")),
    });
    const daemonActions = makeFakeDaemonActions();
    const ctx = makeCtx({ http, daemonActions });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await expect(tool.submit(ctx, { project: "/repo/a", command: "claude" })).rejects.toThrow();
    expect(daemonActions.selectSessionCalls).toEqual([]);
  });

  it("forwards worktree=true on the wire", async () => {
    const http = makeFakeHttp();
    const ctx = makeCtx({ http });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await tool.submit(ctx, {
      project: "/repo/a",
      command: "claude",
      worktree: true,
    });
    expect(http.createSession).toHaveBeenCalledWith({
      project: "/repo/a",
      command: "claude",
      worktree: true,
    });
  });

  it("maps host=true to sandbox: 'host' (ADR-0042)", async () => {
    const http = makeFakeHttp();
    const ctx = makeCtx({ http });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await tool.submit(ctx, {
      project: "/repo/a",
      command: "claude",
      host: true,
    });
    expect(http.createSession).toHaveBeenCalledWith({
      project: "/repo/a",
      command: "claude",
      sandbox: "host",
    });
  });

  it("omits sandbox when host=false (= 'auto' on the wire)", async () => {
    const http = makeFakeHttp();
    const ctx = makeCtx({ http });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await tool.submit(ctx, {
      project: "/repo/a",
      command: "claude",
      host: false,
    });
    expect(http.createSession).toHaveBeenCalledWith({
      project: "/repo/a",
      command: "claude",
    });
  });

  it("throws when project is missing and does not notify / close", async () => {
    const http = makeFakeHttp();
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await expect(tool.submit(ctx, { command: "claude" })).rejects.toThrow(/project/);
    expect(http.createSession).not.toHaveBeenCalled();
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });

  it("propagates http.createSession failure without notifying success", async () => {
    const httpErr = new Error("HTTP 500");
    const http = makeFakeHttp({ createSession: vi.fn().mockRejectedValue(httpErr) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "new-session");
    await expect(tool.submit(ctx, { project: "/repo/a", command: "claude" })).rejects.toBe(httpErr);
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// stop-session.submit
// ---------------------------------------------------------------------------

describe("stop-session.submit", () => {
  it("calls http.deleteSession with the sessionId", async () => {
    const http = makeFakeHttp();
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "stop-session");
    await tool.submit(ctx, { sessionId: "sess-42" });
    expect(http.deleteSession).toHaveBeenCalledTimes(1);
    expect(http.deleteSession).toHaveBeenCalledWith("sess-42");
    expect(notify.successCalls).toEqual(["セッションを停止しました"]);
    expect(store.closeCalls).toBe(1);
    expect(store.clearActiveIfCalls).toEqual(["sess-42"]);
  });

  it("propagates http.deleteSession failure", async () => {
    const httpErr = new Error("HTTP 404");
    const http = makeFakeHttp({ deleteSession: vi.fn().mockRejectedValue(httpErr) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store });
    const tool = findTool(listTools(ctx.daemon, []), "stop-session");
    await expect(tool.submit(ctx, { sessionId: "sess-42" })).rejects.toBe(httpErr);
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
    // clearActiveIf is only called on success — the active pointer stays
    // intact if the kill failed.
    expect(store.clearActiveIfCalls).toEqual([]);
  });

  it("throws when sessionId is missing", async () => {
    const ctx = makeCtx();
    const tool = findTool(listTools(ctx.daemon, []), "stop-session");
    await expect(tool.submit(ctx, {})).rejects.toThrow(/sessionId/);
  });
});

// ---------------------------------------------------------------------------
// option-source helpers
// ---------------------------------------------------------------------------

describe("projectOptions", () => {
  it("projects daemon.projects to value + getText", () => {
    const daemon = makeDaemonSnapshot({
      projects: [
        { path: "/repo/a", isGit: true, isSandboxed: false },
        { path: "/repo/b", isGit: false, isSandboxed: true },
      ],
    });
    const opts = projectOptions(daemon);
    expect(opts.map((o) => o.value)).toEqual(["/repo/a", "/repo/b"]);
    const firstOpt = opts[0];
    if (!firstOpt) throw new Error("missing opt");
    expect(firstOpt.getText(firstOpt.value)).toBe("/repo/a");
  });
});

describe("sessionOptions", () => {
  it("uses ADR-0033 display label (title → subtitle → id fallback)", () => {
    const daemon = makeDaemonSnapshot({
      sessions: [
        sessionFixture("s1", "My session"),
        sessionFixture("s2"), // no title → falls back to id
      ],
    });
    const opts = sessionOptions(daemon);
    expect(opts.map((o) => o.value)).toEqual(["s1", "s2"]);
    const o0 = opts[0];
    const o1 = opts[1];
    if (!o0 || !o1) throw new Error("missing opt");
    expect(o0.getText(o0.value)).toBe("My session");
    expect(o1.getText(o1.value)).toBe("s2");
  });
});

// ---------------------------------------------------------------------------
// scopeDisabledReason (ADR-0047: single source of truth)
// ---------------------------------------------------------------------------

describe("scopeDisabledReason", () => {
  it("standard scope is always enabled regardless of daemon state", () => {
    expect(scopeDisabledReason("standard", makeDaemonSnapshot())).toBeNull();
    expect(
      scopeDisabledReason("standard", makeDaemonSnapshot({ activeSessionID: null })),
    ).toBeNull();
    expect(
      scopeDisabledReason(
        "standard",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" }),
      ),
    ).toBeNull();
  });

  it("FR-005: push is disabled with 'アクティブセッションなし' when no active session", () => {
    expect(scopeDisabledReason("push", makeDaemonSnapshot({ activeSessionID: null }))).toBe(
      "アクティブセッションなし",
    );
  });

  it("FR-006: push is disabled with 'push 対象 driver なし' when occupant is not 'frame'", () => {
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" }),
      ),
    ).toBe("push 対象 driver なし");
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "log" }),
      ),
    ).toBe("push 対象 driver なし");
  });

  it("FR-006: push fails closed (disabled) when occupant is undefined", () => {
    // The wire may not yet carry occupant; absence must read as "no frame"
    // so a stale push cannot fire against a pane that may have already
    // shifted off the frame driver.
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: undefined }),
      ),
    ).toBe("push 対象 driver なし");
  });

  it("FR-005 + FR-006: push is enabled when both active session and frame occupant are present", () => {
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "frame" }),
      ),
    ).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// push ToolDef (dynamic expansion via listTools / toolsForPush)
// ---------------------------------------------------------------------------

describe("push ToolDef expansion", () => {
  it("generates a paramless push ToolDef with id 'push:<command>' and label = command", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save", "commit"]);
    const save = findTool(tools, "push:save");
    expect(save.scope).toBe("push");
    expect(save.label).toBe("save");
    // FR-010: paramless = Enter 即送信. The store skips ParamSelectPhase
    // entirely when params === null.
    expect(save.params).toBeNull();
    const commit = findTool(tools, "push:commit");
    expect(commit.label).toBe("commit");
    expect(commit.params).toBeNull();
  });

  it("FR-028: returns standard tools only when pushCommands is empty (no shutdown)", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    expect(tools).toHaveLength(2);
    expect(tools.find((t) => t.scope === "push")).toBeUndefined();
  });
});

describe("push ToolDef.disabledReason (shares scopeDisabledReason per ADR-0047)", () => {
  it("FR-005: returns 'アクティブセッションなし' when no active session", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    expect(t.disabledReason(makeDaemonSnapshot({ activeSessionID: null }))).toBe(
      "アクティブセッションなし",
    );
  });

  it("FR-006: returns 'push 対象 driver なし' when occupant is not 'frame'", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" })),
    ).toBe("push 対象 driver なし");
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "log" })),
    ).toBe("push 対象 driver なし");
    // Absence of occupant fails closed — same contract as scopeDisabledReason.
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: undefined })),
    ).toBe("push 対象 driver なし");
  });

  it("returns null when active session + frame occupant are both present", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "frame" })),
    ).toBeNull();
  });

  it("ADR-0047: push ToolDef.disabledReason agrees with scopeDisabledReason('push', daemon) byte-for-byte", () => {
    // The same daemon snapshot fed to both code paths MUST produce the same
    // string, because palette-store's submit-time re-check (FR-023) calls
    // ToolDef.disabledReason while ScopeSegment renders scopeDisabledReason
    // — drift between the two would silently let a submit through when the
    // segment looks disabled, or vice versa.
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    const cases: DaemonSnapshot[] = [
      makeDaemonSnapshot({ activeSessionID: null }),
      makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" }),
      makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "log" }),
      makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: undefined }),
      makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "frame" }),
    ];
    for (const d of cases) {
      expect(t.disabledReason(d)).toBe(scopeDisabledReason("push", d));
    }
  });
});

describe("push ToolDef.submit", () => {
  function activeFrameDaemon(): DaemonSnapshot {
    return makeDaemonSnapshot({ activeSessionID: "sess-42", activeOccupant: "frame" });
  }

  it("calls ctx.http.pushCommand(activeSessionID, command) on success", async () => {
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: activeFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await tool.submit(ctx, {});
    expect(http.pushCommand).toHaveBeenCalledTimes(1);
    expect(http.pushCommand).toHaveBeenCalledWith("sess-42", "save");
  });

  it("notifies success with 'push: <command>' and closes the palette", async () => {
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: activeFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["commit"]), "push:commit");
    await tool.submit(ctx, {});
    expect(notify.successCalls).toEqual(["push: commit"]);
    expect(store.closeCalls).toBe(1);
  });

  it("throws 'no active session' when activeSessionID is null", async () => {
    // palette-store gates this via disabledReason re-check (FR-023); the
    // defensive throw protects against direct submit() calls (test code,
    // future entry points) that bypass the store.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({
      http,
      notify,
      store,
      daemon: makeDaemonSnapshot({ activeSessionID: null }),
    });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await expect(tool.submit(ctx, {})).rejects.toThrow(/no active session/);
    expect(http.pushCommand).not.toHaveBeenCalled();
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });

  it("propagates ctx.http.pushCommand failure without notifying success or closing", async () => {
    const httpErr = new Error("HTTP 502");
    const http = makeFakeHttp({ pushCommand: vi.fn().mockRejectedValue(httpErr) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: activeFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await expect(tool.submit(ctx, {})).rejects.toBe(httpErr);
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });

  it("does NOT re-evaluate disabledReason inside submit (FR-023 lives in palette-store)", async () => {
    // Even when the daemon snapshot would make disabledReason return a
    // non-null reason, submit() proceeds — the palette-store owns the gate.
    // We assert this by giving the tool a daemon WITH activeSessionID (so
    // the inner throw doesn't fire) but with occupant=main (so
    // disabledReason would say 'push 対象 driver なし'). submit() must still
    // call http.pushCommand because the gate is the store's responsibility.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const ctx = makeCtx({
      http,
      daemon: makeDaemonSnapshot({ activeSessionID: "sess-99", activeOccupant: "main" }),
    });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    expect(tool.disabledReason(ctx.daemon)).toBe("push 対象 driver なし");
    await tool.submit(ctx, {});
    expect(http.pushCommand).toHaveBeenCalledWith("sess-99", "save");
  });
});
