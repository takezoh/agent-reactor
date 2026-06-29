import { describe, expect, it, vi } from "vitest";
import type { SessionsApi } from "../api/sessions";
import type { ActiveContextSnapshot } from "../store/palette_active_context";
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
  addCalls: Array<{
    level: "info" | "warn" | "error";
    message: string;
    title?: string;
    body?: string;
  }>;
} {
  const successCalls: string[] = [];
  const errorCalls: string[] = [];
  const addCalls: Array<{
    level: "info" | "warn" | "error";
    message: string;
    title?: string;
    body?: string;
  }> = [];
  return {
    success(m) {
      successCalls.push(m);
    },
    error(m) {
      errorCalls.push(m);
    },
    add(input) {
      addCalls.push(input);
    },
    successCalls,
    errorCalls,
    addCalls,
  };
}

function makeFakeStore(): ToolStoreCtx & {
  closeCalls: number;
} {
  const state = {
    closeCalls: 0,
  };
  return {
    close() {
      state.closeCalls += 1;
    },
    get closeCalls() {
      return state.closeCalls;
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

function sessionFixture(id: string, title?: string, project?: string): SessionInfo {
  return {
    id,
    project: project ?? "/p",
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
    commands: [],
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

describe("listTools deterministic order", () => {
  it("returns new-session as the sole standard tool when pushCommands is empty", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    expect(tools.map((t) => t.id)).toEqual(["new-session"]);
  });

  it("never includes a stop-session entry (FR-B2 removal)", () => {
    // stop-session was retired in tools-registry-rewrite: deletion no longer
    // belongs to the palette surface (server-side DELETE /api/sessions/:id
    // remains for future re-introduction). listTools must NOT surface it for
    // any combination of daemon state or pushCommands.
    const tools = listTools(
      makeDaemonSnapshot({ sessions: [sessionFixture("s1")], activeSessionID: "s1" }),
      ["save", "commit"],
    );
    const ids = tools.map((t) => t.id);
    expect(ids).not.toContain("stop-session");
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
    expect(tools.map((t) => t.id)).toEqual(["new-session", "push:save", "push:commit"]);
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
// ParamDef discriminated union
// ---------------------------------------------------------------------------

describe("ParamDef discriminated union shape", () => {
  it("newSessionTool.params[0] is a dynamic-options ParamDef keyed to 'projects'", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    const newSession = findTool(tools, "new-session");
    const params = newSession.params;
    if (!params) throw new Error("expected params on new-session");
    const project = params[0];
    if (!project) throw new Error("expected project param");
    expect(project.id).toBe("project");
    expect(project.kind).toBe("dynamic-options");
    if (project.kind !== "dynamic-options") throw new Error("kind narrowing failed");
    expect(project.materializeKey).toBe("projects");
    expect(project.required).toBe(true);
    // The dynamic variant MUST NOT carry an in-place `options` array: that
    // would conflict with the materialize-at-param-select-time contract.
    expect("options" in project).toBe(false);
  });

  it("newSessionTool.params[1] is a dynamic-options ParamDef with materializeKey 'commands'", () => {
    // web-ui-fixes 2026-06-24: the command field is now a curated picker
    // sourced from /api/session-config's [session].commands list, not
    // free-form text.
    const tools = listTools(makeDaemonSnapshot(), []);
    const newSession = findTool(tools, "new-session");
    const params = newSession.params;
    if (!params) throw new Error("expected params on new-session");
    const command = params[1];
    if (!command) throw new Error("expected command param");
    expect(command.id).toBe("command");
    expect(command.kind).toBe("dynamic-options");
    if (command.kind !== "dynamic-options") throw new Error("kind narrowing failed");
    expect(command.materializeKey).toBe("commands");
    expect(command.required).toBe(true);
    // The dynamic variant MUST NOT carry an in-place options array.
    expect("options" in command).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// newSessionTool labels (English)
// ---------------------------------------------------------------------------

describe("newSessionTool labels", () => {
  it("uses English-only labels and success message", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    const newSession = findTool(tools, "new-session");
    expect(newSession.label).toBe("New Session");
    const params = newSession.params;
    if (!params) throw new Error("expected params");
    const project = params[0];
    const command = params[1];
    if (!project || !command) throw new Error("expected both params");
    expect(project.label).toBe("Project");
    expect(command.label).toBe("Command");
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
    expect(notify.successCalls).toEqual(["Session created"]);
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
    expect(notify.successCalls).toEqual(["Session created"]);
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
// option-source helpers
// ---------------------------------------------------------------------------

describe("projectOptions", () => {
  it("projects daemon.projects to value + label ParamOption entries", () => {
    const daemon = makeDaemonSnapshot({
      projects: [
        { path: "/repo/a", isGit: true, isSandboxed: false },
        { path: "/repo/b", isGit: false, isSandboxed: true },
      ],
    });
    const opts = projectOptions(daemon);
    expect(opts.map((o) => o.value)).toEqual(["/repo/a", "/repo/b"]);
    expect(opts.map((o) => o.label)).toEqual(["/repo/a", "/repo/b"]);
  });

  it("is the sole materialize for materializeKey === 'projects'", () => {
    // newSessionTool.params[0] declares materializeKey === 'projects', so
    // projectOptions must remain the unique implementation. This test fails
    // loudly if the contract drifts (e.g. someone changes the materializeKey
    // value or adds a second helper without removing this one).
    const tools = listTools(makeDaemonSnapshot(), []);
    const newSession = findTool(tools, "new-session");
    const params = newSession.params;
    if (!params) throw new Error("expected params");
    const project = params[0];
    if (!project || project.kind !== "dynamic-options") {
      throw new Error("expected dynamic-options project param");
    }
    expect(project.materializeKey).toBe("projects");
    // projectOptions accepts a DaemonSnapshot and returns ParamOption[]; the
    // mere call below is the type-level proof that this is the right shape.
    const opts = projectOptions(makeDaemonSnapshot());
    expect(Array.isArray(opts)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// scopeDisabledReason (ADR-0047: single source of truth)
// ---------------------------------------------------------------------------

describe("scopeDisabledReason English", () => {
  it("returns 'No active session' for push scope when active session is missing", () => {
    expect(scopeDisabledReason("push", makeDaemonSnapshot({ activeSessionID: null }))).toBe(
      "No active session",
    );
  });

  it("returns 'No push-capable driver' for push scope when occupant is not 'frame'", () => {
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" }),
      ),
    ).toBe("No push-capable driver");
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "log" }),
      ),
    ).toBe("No push-capable driver");
    // Absence of occupant fails closed — same English copy.
    expect(
      scopeDisabledReason(
        "push",
        makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: undefined }),
      ),
    ).toBe("No push-capable driver");
  });
});

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
    // FR-010: paramless = Enter immediate send. The store skips
    // ParamSelectPhase entirely when params === null.
    expect(save.params).toBeNull();
    const commit = findTool(tools, "push:commit");
    expect(commit.label).toBe("commit");
    expect(commit.params).toBeNull();
  });

  it("FR-028: returns standard tools only when pushCommands is empty (no shutdown)", () => {
    const tools = listTools(makeDaemonSnapshot(), []);
    expect(tools).toHaveLength(1);
    expect(tools.find((t) => t.scope === "push")).toBeUndefined();
  });
});

describe("push ToolDef.disabledReason (shares scopeDisabledReason per ADR-0047)", () => {
  it("FR-005: returns 'No active session' when no active session", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    expect(t.disabledReason(makeDaemonSnapshot({ activeSessionID: null }))).toBe(
      "No active session",
    );
  });

  it("FR-006: returns 'No push-capable driver' when occupant is not 'frame'", () => {
    const tools = listTools(makeDaemonSnapshot(), ["save"]);
    const t = findTool(tools, "push:save");
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "main" })),
    ).toBe("No push-capable driver");
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: "log" })),
    ).toBe("No push-capable driver");
    // Absence of occupant fails closed — same contract as scopeDisabledReason.
    expect(
      t.disabledReason(makeDaemonSnapshot({ activeSessionID: "s1", activeOccupant: undefined })),
    ).toBe("No push-capable driver");
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
    // ToolDef.disabledReason while the listbox renders via scopeDisabledReason
    // — drift between the two would silently let a submit through when the
    // row shows disabled, or vice versa.
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
  // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015

  // Resolved-snapshot fixture: session 'sess_abcd1234efgh' on project '/home/foo/bar'.
  function resolvedFrameDaemon(): DaemonSnapshot {
    return makeDaemonSnapshot({
      activeSessionID: "sess_abcd1234efgh",
      activeOccupant: "frame",
      sessions: [sessionFixture("sess_abcd1234efgh", undefined, "/home/foo/bar")],
      projects: [{ path: "/home/foo/bar", isGit: true, isSandboxed: true }],
    });
  }

  it("calls ctx.http.pushCommand(activeSessionID, command) on success", async () => {
    // UAC-011 / FR-014
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: resolvedFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await tool.submit(ctx, {});
    expect(http.pushCommand).toHaveBeenCalledTimes(1);
    expect(http.pushCommand).toHaveBeenCalledWith("sess_abcd1234efgh", "save");
  });

  it("FR-014 / UAC-011: emits info toast with projBase · sid8 message and fullPath/fullSessionId title", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: resolvedFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await tool.submit(ctx, {});
    expect(notify.addCalls).toHaveLength(1);
    expect(notify.addCalls[0]).toEqual({
      level: "info",
      title: "/home/foo/bar\nsess_abcd1234efgh",
      message: "Sent 'save' → bar · sess_abc",
    });
    // FR-015: no interactive elements in toast — the add call is a plain data
    // object without callbacks/actions. successCalls must remain empty.
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(1);
  });

  it("UAC-018: frozenActiveContext snapshot is used even when ctx.daemon active changes", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    // frozenActiveContext is the submit-time capture; ctx.daemon may drift
    // after a view-update. The toast MUST use the frozen snapshot, not re-derive.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const frozenActiveContext: ActiveContextSnapshot = {
      kind: "resolved",
      projBase: "frozen-proj",
      sid8: "frozen12",
      fullPath: "/home/frozen/proj",
      fullSessionId: "frozen_session_id",
    };
    // ctx.daemon points to a *different* active session — toast must ignore it
    const ctx = makeCtx({
      http,
      notify,
      store,
      daemon: resolvedFrameDaemon(), // has sess_abcd1234efgh as active
      frozenActiveContext,
    });
    const tool = findTool(listTools(ctx.daemon, ["commit"]), "push:commit");
    await tool.submit(ctx, {});
    expect(notify.addCalls).toHaveLength(1);
    expect(notify.addCalls[0]).toEqual({
      level: "info",
      title: "/home/frozen/proj\nfrozen_session_id",
      message: "Sent 'commit' → frozen-proj · frozen12",
    });
  });

  it("kind=unknown: emits info toast with ??? projBase and sid8 only in title", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    // Session found but project not in projects list → kind=unknown.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    // activeSessionID is present, session is in sessions[], but projects[] is empty
    const daemon = makeDaemonSnapshot({
      activeSessionID: "sess_unknown1234",
      activeOccupant: "frame",
      sessions: [sessionFixture("sess_unknown1234", undefined, "/home/foo/missing")],
      projects: [], // project not registered
    });
    const ctx = makeCtx({ http, notify, store, daemon });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await tool.submit(ctx, {});
    expect(notify.addCalls).toHaveLength(1);
    expect(notify.addCalls[0]).toEqual({
      level: "info",
      title: "sess_unknown1234",
      message: "Sent 'save' → ??? · sess_unk",
    });
  });

  it("kind=none (defensive): emits minimal info toast when snapshot is none", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    // Normally the activeSessionID guard throws before reaching here.
    // Test via frozenActiveContext to exercise the defensive branch.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const frozenActiveContext: ActiveContextSnapshot = { kind: "none" };
    const ctx = makeCtx({
      http,
      notify,
      store,
      daemon: resolvedFrameDaemon(),
      frozenActiveContext,
    });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await tool.submit(ctx, {});
    expect(notify.addCalls).toHaveLength(1);
    expect(notify.addCalls[0]).toEqual({ level: "info", message: "Sent 'save'" });
  });

  it("throws 'no active session' when activeSessionID is null", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
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
    expect(notify.addCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });

  it("propagates ctx.http.pushCommand failure without notifying or closing", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    const httpErr = new Error("HTTP 502");
    const http = makeFakeHttp({ pushCommand: vi.fn().mockRejectedValue(httpErr) });
    const notify = makeFakeNotify();
    const store = makeFakeStore();
    const ctx = makeCtx({ http, notify, store, daemon: resolvedFrameDaemon() });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    await expect(tool.submit(ctx, {})).rejects.toBe(httpErr);
    expect(notify.addCalls).toEqual([]);
    expect(notify.successCalls).toEqual([]);
    expect(store.closeCalls).toBe(0);
  });

  it("does NOT re-evaluate disabledReason inside submit (FR-023 lives in palette-store)", async () => {
    // UAC-011 / UAC-012 / UAC-018 / FR-014 / FR-015
    // Even when the daemon snapshot would make disabledReason return a
    // non-null reason, submit() proceeds — the palette-store owns the gate.
    // We assert this by giving the tool a daemon WITH activeSessionID (so
    // the inner throw doesn't fire) but with occupant=main (so
    // disabledReason would say 'No push-capable driver'). submit() must
    // still call http.pushCommand because the gate is the store's
    // responsibility.
    const http = makeFakeHttp({ pushCommand: vi.fn().mockResolvedValue(undefined) });
    const ctx = makeCtx({
      http,
      daemon: makeDaemonSnapshot({ activeSessionID: "sess-99", activeOccupant: "main" }),
    });
    const tool = findTool(listTools(ctx.daemon, ["save"]), "push:save");
    expect(tool.disabledReason(ctx.daemon)).toBe("No push-capable driver");
    await tool.submit(ctx, {});
    expect(http.pushCommand).toHaveBeenCalledWith("sess-99", "save");
  });
});
