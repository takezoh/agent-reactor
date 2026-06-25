// Tests for ParamSelectPhase — exercises FR-011 / FR-012 / FR-013 /
// FR-014 / FR-015 / FR-016 / FR-019 / FR-020 plus the palette-bugfix
// extensions FR-A1 (leading-option preset on dynamic-options) / FR-A4
// (empty dynamic-options renders ParamEmptyState + suppresses later
// params) / FR-IME (composing gate on Enter/ArrowUp/ArrowDown) /
// FR-Det (preselect-direct and toolSelect-then-confirm land on
// identical DOM).
//
// We drive the store with setState() / openPalette() / confirmTool() to
// put the palette in the precise phase ParamSelectPhase consumes, then
// render and dispatch React Testing Library events.

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiHttpError, SessionsApi } from "../../api/sessions";
import type { NotificationsApi, ToolCtx, ToolDaemonActions, ToolStoreCtx } from "../../lib/tools";
import { useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import { type DaemonSnapshot, mkSnapshot } from "../../test/fixtures/daemon";
import { ParamSelectPhase, materializeOptions } from "./ParamSelectPhase";

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
  };
}

function makeFakeDaemonActions(): ToolDaemonActions {
  return {
    selectSession() {},
  };
}

function makeCtx(overrides: Partial<ToolCtx> = {}): ToolCtx {
  return {
    http: makeFakeHttp(),
    daemon: mkSnapshot(),
    daemonActions: makeFakeDaemonActions(),
    notify: makeFakeNotify(),
    store: makeFakeStoreActions(),
    ...overrides,
  };
}

// setDaemonSnapshot pushes a mkSnapshot()-shaped DaemonSnapshot into the
// useDaemonStore so selectDaemonSnapshot() inside ParamSelectPhase sees
// the test's intent. projects/pushCommands live under sessionConfig in
// the store, so we rewrite them onto that slice instead of the flat
// top-level fields the snapshot itself exposes.
function setDaemonSnapshot(snap: DaemonSnapshot): void {
  useDaemonStore.setState({
    sessions: snap.sessions,
    activeSessionID: snap.activeSessionID,
    activeOccupant: snap.activeOccupant,
    sessionConfig: {
      projects: snap.projects,
      pushCommands: snap.pushCommands,
    },
  });
}

// seedPalette puts the store into paramSelect on a given tool id, with
// optional pre-filled paramValues / paramCursor. The two FR-Det test
// scenarios use openPalette + confirmTool / preselectToolId variants
// directly (NOT this helper) so each entry path is exercised end-to-end.
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
  setDaemonSnapshot(mkSnapshot());
}

function makeHttpError(status: number, message = `HTTP ${status}`): ApiHttpError {
  const err = new Error(message) as ApiHttpError;
  err.status = status;
  return err;
}

// commandInput grabs the new-session command field by id. We pull by id
// rather than label so the test stays decoupled from the (eventually
// English-only) label string.
function commandInput(): HTMLInputElement {
  return document.getElementById("palette-param-command") as HTMLInputElement;
}

function projectListbox(): HTMLElement {
  return document.getElementById("palette-param-project") as HTMLElement;
}

// ---------------------------------------------------------------------------
// materializeOptions unit tests (pure helper)
// ---------------------------------------------------------------------------

describe("materializeOptions", () => {
  it("dynamic-options/projects projects daemon.projects to {value,label}", () => {
    const snap = mkSnapshot({
      projects: [{ path: "/a" }, { path: "/b" }],
    });
    const opts = materializeOptions(
      { id: "project", kind: "dynamic-options", materializeKey: "projects", label: "Project" },
      snap,
    );
    expect(opts).toEqual([
      { value: "/a", label: "/a" },
      { value: "/b", label: "/b" },
    ]);
  });

  it("dynamic-options with no projects returns []", () => {
    const opts = materializeOptions(
      { id: "project", kind: "dynamic-options", materializeKey: "projects", label: "Project" },
      mkSnapshot(),
    );
    expect(opts).toEqual([]);
  });

  it("static-options returns the baked-in options array", () => {
    const baked = [
      { value: "a", label: "Alpha" },
      { value: "b", label: "Beta" },
    ];
    const opts = materializeOptions(
      { id: "x", kind: "static-options", options: baked, label: "X" },
      mkSnapshot(),
    );
    expect(opts).toBe(baked);
  });

  it("text returns null (caller renders a text input)", () => {
    const opts = materializeOptions(
      { id: "command", kind: "text", label: "Command" },
      mkSnapshot(),
    );
    expect(opts).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe("ParamSelectPhase", () => {
  beforeEach(() => {
    resetStores();
  });

  // -------------------------------------------------------------------------
  // FR-011 / FR-012 layout
  // -------------------------------------------------------------------------

  it("renders new-session with project listbox + command text input", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [
          { path: "/repo/a", isGit: true },
          { path: "/repo/b", isSandboxed: true },
        ],
      }),
    );
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const groups = screen.getAllByRole("group");
    expect(groups.length).toBe(2);
    const [projectGroup, commandGroup] = groups;
    if (!projectGroup || !commandGroup) throw new Error("missing param groups");
    expect(projectGroup.querySelector("[role=listbox]")).not.toBeNull();
    expect(commandGroup.querySelector("input[type=text]")).not.toBeNull();
  });

  // -------------------------------------------------------------------------
  // FR-A1 — dynamic-options listbox + leading-option preset
  // -------------------------------------------------------------------------

  it("renders one option per daemon project and presets the first (FR-A1)", async () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }, { path: "/repo/b" }, { path: "/repo/c" }],
      }),
    );
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const lb = projectListbox();
    expect(lb).not.toBeNull();
    const options = lb.querySelectorAll("[role=option]");
    expect(options.length).toBe(3);

    // FR-A1 leading-option preset: the useEffect lands in a microtask
    // after the first render. waitFor lets React flush the setParam.
    await waitFor(() => {
      expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
    });
    await waitFor(() => {
      const first = lb.querySelector("[role=option]:first-child");
      expect(first?.getAttribute("aria-selected")).toBe("true");
      expect(lb.getAttribute("aria-activedescendant")).toBe("palette-param-project-opt-0");
    });
  });

  it("leading-option preset does NOT clobber a user-set value (FR-A1)", async () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }, { path: "/repo/b" }],
      }),
    );
    // User had already chosen /repo/b before reopening the palette.
    seedPalette("new-session", { project: "/repo/b" });

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // Microtask flush — give any rogue effect a chance to fire and
    // (incorrectly) reset the value to /repo/a. The assertion below
    // pins that it must NOT.
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(usePaletteStore.getState().paramValues.project).toBe("/repo/b");
  });

  it("preset effect does not fire for text / static-options params (FR-A1)", async () => {
    // new-session has [dynamic-options 'project', text 'command']. We
    // assert that after the preset settles, paramValues.command is
    // still undefined — the text-kind slot was NOT touched.
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    await waitFor(() => {
      expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
    });
    expect(usePaletteStore.getState().paramValues.command).toBeUndefined();
  });

  // -------------------------------------------------------------------------
  // FR-A4 — empty dynamic-options → ParamEmptyState + suppression
  // -------------------------------------------------------------------------

  it("renders ParamEmptyState and suppresses later params + submit when projects=[] (FR-A4)", async () => {
    setDaemonSnapshot(mkSnapshot({ projects: [] }));
    seedPalette("new-session");

    const ctx = makeCtx({
      http: makeFakeHttp({
        createSession: vi.fn().mockResolvedValue({ id: "sess-new" }),
      }),
    });

    render(<ParamSelectPhase ctx={ctx} />);

    // role=status with the exact English copy.
    const status = screen.getByRole("status");
    expect(status.textContent).toBe("No projects available - add a project first");

    // The command field is suppressed entirely — no input, no submit
    // affordance (the form has no <button type=submit>; Enter on a
    // non-existent input cannot fire advanceOrSubmit either).
    expect(document.getElementById("palette-param-command")).toBeNull();
    expect(screen.queryByRole("button", { name: /submit/i })).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("paramCursor does not advance past the empty-state slot (FR-A4)", () => {
    setDaemonSnapshot(mkSnapshot({ projects: [] }));
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // There is no input/listbox to receive Enter, so paramCursor stays
    // pinned at 0 across the empty-state render.
    expect(usePaletteStore.getState().paramCursor).toBe(0);
  });

  it("Esc back from empty state returns to toolSelect and clears paramValues (FR-A4)", () => {
    setDaemonSnapshot(mkSnapshot({ projects: [] }));
    seedPalette("new-session", { project: "stale" });

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // back() is the store-level reducer the palette shell wires to
    // Esc. Calling it directly mirrors what the shell does on Esc.
    act(() => {
      usePaletteStore.getState().back();
    });
    const state = usePaletteStore.getState();
    expect(state.phase).toBe("toolSelect");
    expect(state.selectedToolId).toBeNull();
    expect(state.paramValues).toEqual({});
  });

  // -------------------------------------------------------------------------
  // FR-Det — preselect vs toolSelect-then-confirm parity
  //
  // The two entry paths (preselect-direct and toolSelect-then-confirm) MUST
  // produce byte-equivalent DOM so a future change to one path cannot
  // silently diverge the other. We render both in succession, snapshot the
  // form's outerHTML after waiting for the project listbox to land on its
  // preset option, and assert strict equality.
  //
  // outerHTML (not isEqualNode) is the comparison primitive because:
  //   - it's stable across React reconciler internals (data-reactid etc. are
  //     gone in modern React)
  //   - any drift in attribute order / option count / aria-activedescendant /
  //     role / id surfaces as a unified diff
  //   - isEqualNode would also compare host attributes the test does not
  //     care about (e.g. data-react-action) and is harder to debug on
  //     failure (boolean result, no diff)
  // -------------------------------------------------------------------------

  it("FR-Det: preselect-direct and toolSelect-then-confirm yield byte-equal DOM", async () => {
    const snap = mkSnapshot({ projects: [{ path: "/p" }, { path: "/q" }] });

    // First render: preselect-direct.
    setDaemonSnapshot(snap);
    usePaletteStore.getState().openPalette({
      preselectToolId: "new-session",
      daemonSnapshot: snap,
    });
    const firstRender = render(<ParamSelectPhase ctx={makeCtx()} />);
    const firstForm = firstRender.container.querySelector(
      "form.palette-param-select",
    ) as HTMLFormElement;
    await waitFor(() => {
      expect(projectListbox().getAttribute("aria-activedescendant")).toBe(
        "palette-param-project-opt-0",
      );
    });
    const preselectHtml = firstForm.outerHTML;
    firstRender.unmount();

    // Second render: toolSelect-then-confirm. Reset the store BEFORE the
    // second openPalette so we observe the same transition pipeline (no
    // residual focused/cursor leak from the first run).
    resetStores();
    setDaemonSnapshot(snap);
    usePaletteStore.getState().openPalette({ daemonSnapshot: snap });
    usePaletteStore.getState().confirmTool("new-session");
    const secondRender = render(<ParamSelectPhase ctx={makeCtx()} />);
    const secondForm = secondRender.container.querySelector(
      "form.palette-param-select",
    ) as HTMLFormElement;
    await waitFor(() => {
      expect(projectListbox().getAttribute("aria-activedescendant")).toBe(
        "palette-param-project-opt-0",
      );
    });
    const confirmHtml = secondForm.outerHTML;

    // Strict byte-equality. Any drift (option ordering, attribute set,
    // aria-activedescendant target, focused class on the wrong field) is
    // a regression of FR-Det.
    expect(confirmHtml).toBe(preselectHtml);

    // Belt-and-braces structural assertions: option count + label order +
    // active descendant. If the byte-equality assertion above fires, these
    // pin the failure to a specific axis rather than leaving the developer
    // to diff a long HTML string.
    const opts = projectListbox().querySelectorAll("[role=option]");
    expect(opts).toHaveLength(2);
    expect(Array.from(opts).map((o) => o.textContent ?? "")).toEqual(["/p", "/q"]);
  });

  // -------------------------------------------------------------------------
  // FR-013/014/015 command-field worktree/host toggles (visibility)
  // -------------------------------------------------------------------------

  it("shows worktree toggle when selected project is a git repo (FR-015)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/git", isGit: true }],
      }),
    );
    seedPalette("new-session", { project: "/repo/git" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).not.toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).toBeNull();
  });

  it("shows host toggle when selected project is sandboxed (FR-015)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/sb", isSandboxed: true }],
      }),
    );
    seedPalette("new-session", { project: "/repo/sb" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).not.toBeNull();
  });

  it("hides both toggles when project is neither git nor sandboxed (FR-015)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/plain" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/plain" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(screen.queryByText(/worktree:/i)).toBeNull();
    expect(screen.queryByText(/sandbox=host:/i)).toBeNull();
  });

  // -------------------------------------------------------------------------
  // FR-013/014/016 command-field Tab / Shift+Tab key handling
  // -------------------------------------------------------------------------

  it("Tab on command field flips paramValues.worktree when project.isGit (FR-013)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/git", isGit: true }],
      }),
    );
    seedPalette("new-session", { project: "/repo/git" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(usePaletteStore.getState().paramValues.worktree).toBeUndefined();
    fireEvent.keyDown(commandInput(), { key: "Tab" });
    expect(usePaletteStore.getState().paramValues.worktree).toBe(true);
    fireEvent.keyDown(commandInput(), { key: "Tab" });
    expect(usePaletteStore.getState().paramValues.worktree).toBe(false);
  });

  it("Tab on command field is pass-through (no toggle) when project is not git (FR-015/016)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/plain" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/plain" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const before = usePaletteStore.getState().paramValues.worktree;
    const ev = fireEvent.keyDown(commandInput(), { key: "Tab" });
    expect(usePaletteStore.getState().paramValues.worktree).toBe(before);
    expect(ev).toBe(true); // RTL returns false only when preventDefault'd
  });

  it("Shift+Tab on command field flips paramValues.host when project.isSandboxed (FR-014)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/sb", isSandboxed: true }],
      }),
    );
    seedPalette("new-session", { project: "/repo/sb" }, 1);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    expect(usePaletteStore.getState().paramValues.host).toBeUndefined();
    fireEvent.keyDown(commandInput(), { key: "Tab", shiftKey: true });
    expect(usePaletteStore.getState().paramValues.host).toBe(true);
    fireEvent.keyDown(commandInput(), { key: "Tab", shiftKey: true });
    expect(usePaletteStore.getState().paramValues.host).toBe(false);
  });

  // -------------------------------------------------------------------------
  // FR-011 / spec point 4 — Enter advances or submits
  // -------------------------------------------------------------------------

  it("Enter on non-final field advances paramCursor (spec point 4)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a" }, 0);

    render(<ParamSelectPhase ctx={makeCtx()} />);

    fireEvent.keyDown(commandInput(), { key: "Enter" });
    expect(usePaletteStore.getState().paramCursor).toBe(1);
  });

  it("Enter on final field triggers store.submit(ctx)", async () => {
    const createSession = vi.fn().mockResolvedValue({ id: "sess-new" });
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a", command: "echo hi" }, 1);

    render(<ParamSelectPhase ctx={ctx} />);

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
  // FR-019 / FR-IME — composing pre-empts Enter / ArrowUp / ArrowDown
  // -------------------------------------------------------------------------

  it("Enter on command field while store.composing=true does NOT submit (FR-019)", async () => {
    const createSession = vi.fn().mockResolvedValue({ id: "sess-new" });
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a", command: "x" }, 1);
    usePaletteStore.setState({ composing: true });

    render(<ParamSelectPhase ctx={ctx} />);

    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter" });
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(createSession).not.toHaveBeenCalled();
  });

  it("Enter on command field while nativeEvent.isComposing=true does NOT submit (FR-IME)", async () => {
    const createSession = vi.fn().mockResolvedValue({ id: "sess-new" });
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a", command: "x" }, 1);

    render(<ParamSelectPhase ctx={ctx} />);

    // Construct a synthetic KeyboardEvent whose nativeEvent.isComposing
    // is true (RTL's fireEvent.keyDown forwards init.isComposing into
    // the underlying KeyboardEvent under happy-dom; the React layer
    // exposes it as e.nativeEvent.isComposing).
    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter", isComposing: true });
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(createSession).not.toHaveBeenCalled();
  });

  it("ArrowDown on project listbox while composing does NOT move selection (FR-IME)", async () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }, { path: "/repo/b" }],
      }),
    );
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);

    // Wait for preset effect.
    await waitFor(() => {
      expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
    });
    act(() => {
      usePaletteStore.setState({ composing: true });
    });
    fireEvent.keyDown(projectListbox(), { key: "ArrowDown" });
    // Selection remains on /repo/a — ArrowDown was dropped by the
    // composing gate.
    expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
  });

  it("ArrowUp on project listbox with nativeEvent.isComposing does NOT move (FR-IME)", async () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }, { path: "/repo/b" }],
      }),
    );
    seedPalette("new-session");

    render(<ParamSelectPhase ctx={makeCtx()} />);
    await waitFor(() => {
      expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
    });
    fireEvent.keyDown(projectListbox(), { key: "ArrowUp", isComposing: true });
    // ArrowUp with isComposing=true is dropped; selection stays at the
    // preset /repo/a (not wrapped to /repo/b).
    expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");
  });

  // -------------------------------------------------------------------------
  // FR-020 submitting=true disables all inputs
  // -------------------------------------------------------------------------

  it("submitting=true disables every input (FR-020)", () => {
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a", isGit: true, isSandboxed: true }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a" }, 1);
    usePaletteStore.setState({ submitting: true });

    render(<ParamSelectPhase ctx={makeCtx()} />);

    const input = commandInput();
    expect(input.disabled).toBe(true);
    const listbox = projectListbox();
    expect(listbox.getAttribute("aria-disabled")).toBe("true");
  });

  // -------------------------------------------------------------------------
  // FR-024 4xx error: surface inline
  // -------------------------------------------------------------------------

  it("submit error sets store.error and does not crash render (FR-024)", async () => {
    const createSession = vi.fn().mockRejectedValue(makeHttpError(400, "bad input"));
    const ctx = makeCtx({ http: makeFakeHttp({ createSession }) });
    setDaemonSnapshot(
      mkSnapshot({
        projects: [{ path: "/repo/a" }],
      }),
    );
    seedPalette("new-session", { project: "/repo/a", command: "echo hi" }, 1);

    render(<ParamSelectPhase ctx={ctx} />);

    await act(async () => {
      fireEvent.keyDown(commandInput(), { key: "Enter" });
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(usePaletteStore.getState().error).toBe("bad input");
    expect(usePaletteStore.getState().submitting).toBe(false);
  });
});
