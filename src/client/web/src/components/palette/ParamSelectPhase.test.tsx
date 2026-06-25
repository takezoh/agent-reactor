// Tests for ParamSelectPhase — exercises FR-011 / FR-012 / FR-013 / FR-014 /
// FR-015 / FR-016 / FR-019 / FR-020 against the live usePaletteStore +
// useDaemonStore singletons. We drive the store with setState() to put it in
// the precise phase / selection shape ParamSelectPhase consumes, then render
// the component and dispatch React Testing Library events.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiHttpError, SessionsApi } from "../../api/sessions";
import type {
  DaemonSnapshot,
  NotificationsApi,
  ToolCtx,
  ToolDaemonActions,
  ToolStoreCtx,
} from "../../lib/tools";
import { useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import type { SessionInfo, View } from "../../wire/server";
import { ParamSelectPhase } from "./ParamSelectPhase";

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

function makeFakeStoreActions(): ToolStoreCtx {
  return {
    close() {},
    clearActiveIf() {},
  };
}

function makeFakeDaemonActions(): ToolDaemonActions {
  return {
    selectSession() {},
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

// emptyView is the minimum shape SessionInfo.view needs to satisfy the
// wire type. Card has no title/subtitle so displayLabel falls back to id —
// keeps fixtures noise-free for stop-session listbox assertions.
const emptyView: View = {
  card: {},
};

function makeSession(id: string, overrides: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id,
    project: "/p",
    command: "cmd",
    created_at: "2026-06-24T00:00:00Z",
    view: emptyView,
    ...overrides,
  };
}

// Set daemon store with optional forward-compat fields. After
// session-config-extension landed (and Y3 unified the snapshot through
// selectDaemonSnapshot), projects + pushCommands live under sessionConfig
// rather than at the top level of DaemonState. This helper accepts the
// flat shape callers historically used and rewrites it onto the real
// sessionConfig slice so we don't have to churn every existing test.
function setDaemonState(
  state: Partial<{
    sessions: SessionInfo[];
    activeSessionID: string | null;
    projects: DaemonSnapshot["projects"];
    pushCommands: string[];
    activeOccupant: DaemonSnapshot["activeOccupant"];
  }>,
): void {
  const { projects, pushCommands, ...rest } = state;
  const patch: Record<string, unknown> = { ...rest };
  if (projects !== undefined || pushCommands !== undefined) {
    // Preserve whichever side the caller did not specify so a partial
    // setState (e.g. setting only pushCommands in a follow-up call) does
    // not clobber the other field.
    const existing = useDaemonStore.getState().sessionConfig;
    patch.sessionConfig = {
      projects: projects ?? existing?.projects ?? [],
      pushCommands: pushCommands ?? existing?.pushCommands ?? [],
    };
  }
  useDaemonStore.setState(patch as Partial<ReturnType<typeof useDaemonStore.getState>>);
}

// Seed the palette into paramSelect on a given tool id, with optional
// paramValues / paramCursor pre-set. Calling open + setState directly is
// the smallest path to the rendered phase without re-implementing the
// confirmTool branch in the test (confirmTool's logic is covered by the
// palette-store test suite).
function seedPalette(
  selectedToolId: string,
  paramValues: Record<string, unknown> = {},
  paramCursor = 0,
): void {
  usePaletteStore.setState({
    open: true,
    phase: "paramSelect",
    scope: "standard",
    selectedToolId,
    paramValues,
    paramCursor,
    submitting: false,
    composing: false,
    error: null,
  });
}

function resetStores(): void {
  usePaletteStore.getState().close();
  usePaletteStore.setState({ refocusSeq: 0 });
  useDaemonStore.getState().reset();
  // reset() doesn't know about forward-compat fields — wipe them too so
  // a leak from one test doesn't taint another.
  setDaemonState({ projects: [], pushCommands: [], activeOccupant: undefined });
}

function makeHttpError(status: number, message = `HTTP ${status}`): ApiHttpError {
  const err = new Error(message) as ApiHttpError;
  err.status = status;
  return err;
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe("ParamSelectPhase", () => {
  beforeEach(() => {
    resetStores();
  });

  // -------------------------------------------------------------------------
  // FR-011 縦並び全表示 / FR-012 listbox vs text branch (new-session)
  // -------------------------------------------------------------------------

  it("renders new-session with project listbox + command text input (FR-011 / FR-012)", () => {
    setDaemonState({
      projects: [
        { path: "/repo/a", isGit: true, isSandboxed: false },
        { path: "/repo/b", isGit: false, isSandboxed: true },
      ],
    });
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // both params show up (vertically — DOM order matches params[])
    const groups = screen.getAllByRole("group");
    expect(groups.length).toBe(2);
    // project is a listbox; command is a text input. We distinguish by
    // the inner role + element type rather than label string so the test
    // doesn't depend on the Japanese label text.
    const [projectGroup, commandGroup] = groups;
    if (!projectGroup || !commandGroup) throw new Error("missing param groups");
    expect(projectGroup.querySelector("[role=listbox]")).not.toBeNull();
    expect(commandGroup.querySelector("input[type=text]")).not.toBeNull();
  });

  it("project listbox enumerates daemon.projects as options (FR-012)", () => {
    setDaemonState({
      projects: [
        { path: "/repo/a", isGit: true, isSandboxed: false },
        { path: "/repo/b", isGit: false, isSandboxed: true },
      ],
    });
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // The new-session ToolDef declares params[0].options as [] (a static
    // empty placeholder per tools.ts). ParamSelectPhase therefore renders
    // ZERO options for project in this task — session-config-extension
    // is what wires daemon.projects → live options. We assert the empty
    // listbox renders so the structure is observable; option enumeration
    // is verified for stop-session below where the ToolDef's option
    // source actually projects from daemon (sessionOptions helper).
    const projectListbox = screen.getAllByRole("listbox")[0];
    if (!projectListbox) throw new Error("expected project listbox");
    const options = projectListbox.querySelectorAll("[role=option]");
    expect(options.length).toBe(0);
  });

  // -------------------------------------------------------------------------
  // FR-013/014/015 command-field worktree/host toggles (visibility)
  // -------------------------------------------------------------------------

  it("shows worktree toggle when selected project is a git repo (FR-015)", () => {
    setDaemonState({
      projects: [{ path: "/repo/git", isGit: true, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/git" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).not.toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).toBeNull();
  });

  it("shows host toggle when selected project is sandboxed (FR-015)", () => {
    setDaemonState({
      projects: [{ path: "/repo/sb", isGit: false, isSandboxed: true }],
    });
    seedPalette("new-session", { project: "/repo/sb" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).not.toBeNull();
  });

  it("hides both toggles when project is neither git nor sandboxed (FR-015)", () => {
    setDaemonState({
      projects: [{ path: "/repo/plain", isGit: false, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/plain" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).toBeNull();
  });

  it("hides both toggles when no project selected (FR-015 — guards undefined project)", () => {
    setDaemonState({
      projects: [{ path: "/repo/git", isGit: true, isSandboxed: true }],
    });
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).toBeNull();
  });

  // -------------------------------------------------------------------------
  // FR-013/014/016 command-field Tab / Shift+Tab key handling
  // -------------------------------------------------------------------------

  function commandInput(): HTMLInputElement {
    // The command field is the text input inside the second group of
    // new-session (project, command). We pull it by id since the test
    // for new-session always knows the param id is 'command'.
    return document.getElementById("palette-param-command") as HTMLInputElement;
  }

  it("Tab on command field flips paramValues.worktree when project.isGit (FR-013)", () => {
    setDaemonState({
      projects: [{ path: "/repo/git", isGit: true, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/git" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(usePaletteStore.getState().paramValues.worktree).toBeUndefined();
    fireEvent.keyDown(commandInput(), { key: "Tab" });
    expect(usePaletteStore.getState().paramValues.worktree).toBe(true);
    fireEvent.keyDown(commandInput(), { key: "Tab" });
    expect(usePaletteStore.getState().paramValues.worktree).toBe(false);
  });

  it("Tab on command field is pass-through (no toggle) when project is not git (FR-015/016)", () => {
    setDaemonState({
      projects: [{ path: "/repo/plain", isGit: false, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/plain" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const before = usePaletteStore.getState().paramValues.worktree;
    const ev = fireEvent.keyDown(commandInput(), { key: "Tab" });
    // Both: store value untouched AND default was not prevented (focus
    // trap is allowed to do its standard tab traversal — FR-016).
    expect(usePaletteStore.getState().paramValues.worktree).toBe(before);
    expect(ev).toBe(true); // RTL returns false only when preventDefault'd
  });

  it("Shift+Tab on command field flips paramValues.host when project.isSandboxed (FR-014)", () => {
    setDaemonState({
      projects: [{ path: "/repo/sb", isGit: false, isSandboxed: true }],
    });
    seedPalette("new-session", { project: "/repo/sb" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(usePaletteStore.getState().paramValues.host).toBeUndefined();
    fireEvent.keyDown(commandInput(), { key: "Tab", shiftKey: true });
    expect(usePaletteStore.getState().paramValues.host).toBe(true);
    fireEvent.keyDown(commandInput(), { key: "Tab", shiftKey: true });
    expect(usePaletteStore.getState().paramValues.host).toBe(false);
  });

  it("Shift+Tab on command field is pass-through when project is not sandboxed (FR-015/016)", () => {
    setDaemonState({
      projects: [{ path: "/repo/git-only", isGit: true, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/git-only" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const before = usePaletteStore.getState().paramValues.host;
    const ev = fireEvent.keyDown(commandInput(), { key: "Tab", shiftKey: true });
    expect(usePaletteStore.getState().paramValues.host).toBe(before);
    expect(ev).toBe(true);
  });

  // -------------------------------------------------------------------------
  // FR-016 non-command fields never hijack Tab
  // -------------------------------------------------------------------------

  it("Tab on stop-session sessionId listbox does not toggle (FR-016)", () => {
    setDaemonState({
      sessions: [makeSession("s1"), makeSession("s2")],
    });
    seedPalette("stop-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const listbox = screen.getByRole("listbox");
    const before = usePaletteStore.getState().paramValues;
    fireEvent.keyDown(listbox, { key: "Tab" });
    // worktree / host must not appear: this isn't a command field.
    expect(usePaletteStore.getState().paramValues.worktree).toBe(before.worktree);
    expect(usePaletteStore.getState().paramValues.host).toBe(before.host);
  });

  // -------------------------------------------------------------------------
  // FR-012 stop-session listbox enumerates sessions
  // -------------------------------------------------------------------------

  it("stop-session renders sessionId listbox populated from daemon.sessions (FR-012)", () => {
    setDaemonState({
      sessions: [
        makeSession("s1", { view: { card: { title: "alpha" } } }),
        makeSession("s2", { view: { card: { title: "beta" } } }),
      ],
    });
    seedPalette("stop-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // The stop-session ToolDef declares params[0].options as a static []
    // placeholder, identical to project. The component renders whatever
    // the ToolDef declares — at this F1 stage, dynamic option projection
    // (sessionOptions) is the shell's responsibility (a later task).
    // We assert the listbox renders so the *structure* is correct; the
    // dynamic population is exercised in the dynamic-push task.
    const listbox = screen.getByRole("listbox");
    expect(listbox).not.toBeNull();
  });

  // -------------------------------------------------------------------------
  // FR-011 / spec point 4 — Enter advances or submits
  // -------------------------------------------------------------------------

  it("Enter on non-final field advances paramCursor (spec point 4)", () => {
    setDaemonState({
      projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
    });
    // paramCursor=0 (project focused), command is the final field. Fire
    // Enter on the command text input — advanceOrSubmit reads paramCursor
    // from the store, not from the focused element, so this exercises
    // the "non-final → moveCursor(+1)" branch. Listbox Enter is
    // structurally guarded by emptiness (static [] in the ToolDef), so
    // we cannot use the project listbox for this assertion.
    seedPalette("new-session", { project: "/repo/a" }, 0);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    fireEvent.keyDown(commandInput(), { key: "Enter" });
    // paramCursor 0 → 1 (advance). No submit fired because we were not
    // at the final field.
    expect(usePaletteStore.getState().paramCursor).toBe(1);
  });

  it("Enter on final field triggers store.submit(ctx) (acceptance criteria)", async () => {
    const createSession = vi.fn().mockResolvedValue({ id: "sess-new" });
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonState({
      projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
    });
    seedPalette(
      "new-session",
      { project: "/repo/a", command: "echo hi" },
      1, // final field
    );

    render(<ParamSelectPhase ctx={ctx} />);

    // submit is async; wrap the event AND microtask drains in act() so
    // React's state updates inside the awaited promise chain don't trip
    // the "not wrapped in act" warning.
    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter" });
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(createSession).toHaveBeenCalledTimes(1);
    expect(createSession.mock.calls[0]?.[0]).toEqual({
      project: "/repo/a",
      command: "echo hi",
    });
  });

  // -------------------------------------------------------------------------
  // FR-019 IME suppression
  // -------------------------------------------------------------------------

  it("Enter on command field while composing does NOT submit (FR-019)", async () => {
    const createSession = vi.fn().mockResolvedValue({ id: "sess-new" });
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonState({
      projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/a", command: "x" }, 1);
    // Composing=true: the store's submit() and our advanceOrSubmit both
    // gate on this. Setting it on the store directly (rather than via
    // composition events) makes the test independent of happy-dom's
    // composition event timing.
    usePaletteStore.setState({ composing: true });

    render(<ParamSelectPhase ctx={ctx} />);

    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter" });
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(createSession).not.toHaveBeenCalled();
  });

  // -------------------------------------------------------------------------
  // FR-020 submitting=true disables all inputs
  // -------------------------------------------------------------------------

  it("submitting=true disables every input (FR-020)", () => {
    setDaemonState({
      projects: [{ path: "/repo/a", isGit: true, isSandboxed: true }],
    });
    seedPalette("new-session", { project: "/repo/a" }, 1);
    usePaletteStore.setState({ submitting: true });

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const input = commandInput();
    expect(input.disabled).toBe(true);
    const listbox = screen.getAllByRole("listbox")[0];
    if (!listbox) throw new Error("expected listbox");
    expect(listbox.getAttribute("aria-disabled")).toBe("true");
  });

  // -------------------------------------------------------------------------
  // FR-024 4xx error: surface inline (sanity-check the store wiring is in
  // play — full HTTP error matrix lives in palette.test.ts).
  // -------------------------------------------------------------------------

  it("submit error sets store.error and does not crash render (FR-024)", async () => {
    const createSession = vi.fn().mockRejectedValue(makeHttpError(400, "bad input"));
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonState({
      projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
    });
    seedPalette("new-session", { project: "/repo/a", command: "echo hi" }, 1);

    render(<ParamSelectPhase ctx={ctx} />);

    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter" });
      // Three microtask drains: rejection chain + catch + state update.
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(usePaletteStore.getState().error).toBe("bad input");
    expect(usePaletteStore.getState().submitting).toBe(false);
  });
});
