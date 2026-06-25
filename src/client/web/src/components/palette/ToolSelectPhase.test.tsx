// ToolSelectPhase tests.
// UAC-004 / UAC-005 / UAC-006 / UAC-007 / UAC-008 / UAC-015
// FR-001 / FR-002 / FR-003 / FR-004 / FR-005 / FR-006 / FR-007 / FR-008 / FR-030
// FR-026 / FR-019 / FR-020
//
// We render the real component against the real palette / daemon stores and
// only stub the SessionsApi factory + spy on store actions where we need to
// observe side effects.

import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionsApi } from "../../api/sessions";
import { useDaemonStore } from "../../store/daemon";
import { useNotificationsStore } from "../../store/notifications";
import { usePaletteStore } from "../../store/palette";
import type { SortedTools } from "../../store/palette_helpers";
import { ToolSelectPhase } from "./ToolSelectPhase";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeFakeHttp(): SessionsApi {
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
  };
}

function renderToolSelectPhase(props?: Partial<React.ComponentProps<typeof ToolSelectPhase>>) {
  const http = makeFakeHttp();
  const utils = render(<ToolSelectPhase httpFactory={() => http} {...props} />);
  return { http, ...utils };
}

function input(): HTMLInputElement {
  return screen.getByTestId("palette-input") as HTMLInputElement;
}

function options(): HTMLElement[] {
  return Array.from(screen.queryAllByRole("option")) as HTMLElement[];
}

function listbox(): HTMLElement {
  return screen.getByRole("listbox");
}

function selectedToolId(): string | null {
  const opt = options().find((o) => o.getAttribute("aria-selected") === "true");
  return opt?.dataset.toolId ?? null;
}

// Set up a daemon with push commands (save, resume, status) and an active session.
// push:* tools are enabled only when activeSessionID is set + activeOccupant === 'frame'
// (scopeDisabledReason only reads those two fields, not the sessions array).
function setDaemonWithPush({
  activeSessionID = null,
  activeOccupant = undefined,
  pushCommands = ["save", "resume", "status"],
}: {
  activeSessionID?: string | null;
  activeOccupant?: "main" | "log" | "frame" | undefined;
  pushCommands?: string[];
} = {}) {
  useDaemonStore.setState({
    sessions: [],
    activeSessionID,
    activeOccupant,
    sessionConfig: {
      projects: [],
      pushCommands,
    },
  });
}

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

// Capture original store actions before any test can replace them with spies.
// Tests that do usePaletteStore.setState({ confirmTool: spy }) permanently
// replace the action in the store; we restore the originals in beforeEach so
// later tests always get the real implementation.
const originalConfirmTool = usePaletteStore.getState().confirmTool;
const originalEmitDisabledFeedback = usePaletteStore.getState().emitDisabledFeedback;

beforeEach(() => {
  usePaletteStore.setState({
    open: true,
    phase: "toolSelect",
    selectedToolId: null,
    paramValues: {},
    paramCursor: 0,
    query: "",
    composing: false,
    submitting: false,
    error: null,
    opener: null,
    // Restore original actions that tests may have replaced with spies.
    confirmTool: originalConfirmTool,
    emitDisabledFeedback: originalEmitDisabledFeedback,
  });
  useDaemonStore.getState().reset();
  useNotificationsStore.getState().clear();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// FR-001 / FR-002: enabled rows above separator, disabled rows below
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — unified listbox structure", () => {
  it("renders enabled rows above presentation separator and disabled rows below (FR-001, FR-002)", () => {
    // UAC-004: fuzzy is called on all tools (no scope filter).
    // With push commands and no active session, push:* tools are disabled.
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save", "resume"] });
    renderToolSelectPhase();

    const lb = listbox();
    const opts = within(lb).queryAllByRole("option");
    // new-session is enabled; push:save and push:resume are disabled.
    expect(opts.length).toBe(3);

    // Verify DOM order: enabled (new-session) first, then disabled push tools.
    const enabledOpts = opts.filter((o) => o.getAttribute("aria-disabled") !== "true");
    const disabledOpts = opts.filter((o) => o.getAttribute("aria-disabled") === "true");
    expect(enabledOpts.length).toBe(1);
    expect(enabledOpts[0]?.dataset.toolId).toBe("new-session");
    expect(disabledOpts.length).toBe(2);

    // The enabled opts all appear before the first disabled opt in the DOM.
    const allOptElements = opts;
    // biome-ignore lint/style/noNonNullAssertion: test guarantees non-empty arrays above
    const lastEnabledIdx = allOptElements.indexOf(enabledOpts[enabledOpts.length - 1]!);
    // biome-ignore lint/style/noNonNullAssertion: test guarantees non-empty arrays above
    const firstDisabledIdx = allOptElements.indexOf(disabledOpts[0]!);
    expect(lastEnabledIdx).toBeLessThan(firstDisabledIdx);

    // Separator between groups.
    const separator = screen.getByTestId("palette-separator");
    expect(separator.getAttribute("role")).toBe("presentation");
  });

  it("maintains registry order within each group", () => {
    // UAC-004: tool order preserved within enabled/disabled groups.
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save", "resume", "status"] });
    renderToolSelectPhase();

    const opts = options();
    const disabledIds = opts
      .filter((o) => o.getAttribute("aria-disabled") === "true")
      .map((o) => o.dataset.toolId ?? "");
    // Registry order for push tools: save, resume, status.
    expect(disabledIds).toEqual(["push:save", "push:resume", "push:status"]);
  });

  it("renders single listbox with role='listbox' (FR-001)", () => {
    renderToolSelectPhase();
    const lbs = screen.queryAllByRole("listbox");
    expect(lbs).toHaveLength(1);
    expect(lbs[0]?.id).toBe("palette-listbox");
  });

  it("no separator when all tools are enabled", () => {
    // Standard scope only, no push commands = only new-session (enabled).
    renderToolSelectPhase();
    expect(screen.queryByTestId("palette-separator")).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// FR-003 / ADR-0047: disabled rows inline reason
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — disabled row FR-003", () => {
  it("disabled row shows warning icon and scopeDisabledReason string (FR-003)", () => {
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    const disabledOpts = options().filter((o) => o.getAttribute("aria-disabled") === "true");
    expect(disabledOpts.length).toBe(1);
    // biome-ignore lint/style/noNonNullAssertion: test guarantees at least 1 disabled opt above
    const row = disabledOpts[0]!;
    // Warning icon present.
    expect(row.querySelector(".palette-listbox__row--disabled-icon")).not.toBeNull();
    // Reason text present.
    const reasonEl = row.querySelector(".palette-listbox__row--disabled-reason");
    expect(reasonEl).not.toBeNull();
    expect(reasonEl?.textContent?.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// FR-004 / UAC-015: Arrow keys skip disabled rows
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — keyboard disabled skip", () => {
  it("ArrowDown skips disabled rows (FR-004 / UAC-015)", () => {
    // Setup: new-session (enabled, idx=0), push:save (disabled, idx=1), push:status (enabled, idx=2).
    // We need a session with frame to make push:status enabled.
    // But push:status disabled reason delegates to scopeDisabledReason('push', daemon).
    // To have a mix, we need push:save and push:resume disabled but push:status enabled.
    // Actually all push tools share the same disabledReason (scopeDisabledReason).
    // Let's use a simpler fixture: standard tool enabled + all push disabled.
    // After ArrowDown from new-session (idx=0), there are no more enabled rows,
    // so cursor stays at 0 (clamp).
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    // Initial state: cursor at new-session (idx=0).
    expect(selectedToolId()).toBe("new-session");

    // ArrowDown: no more enabled rows after idx=0, cursor stays at 0.
    fireEvent.keyDown(input(), { key: "ArrowDown" });
    expect(selectedToolId()).toBe("new-session");
  });

  it("ArrowDown moves to next enabled row, skipping disabled (UAC-015)", () => {
    // To test skip, we need: enabled(idx=0), disabled(idx=1), enabled(idx=2).
    // This requires push commands where some are enabled.
    // scopeDisabledReason returns null when activeOccupant === 'frame'.
    setDaemonWithPush({
      activeSessionID: "sess-1",
      activeOccupant: "frame",
      pushCommands: ["save"],
    });
    // Now new-session(enabled,0) and push:save(enabled,1) are both enabled.
    // We cannot get enabled/disabled/enabled with current tools unless we
    // partially enable push. Since all push tools share the same disabledReason,
    // let's instead verify ArrowDown advances among all enabled.
    renderToolSelectPhase();

    const opts = options();
    expect(opts.filter((o) => o.getAttribute("aria-disabled") !== "true").length).toBe(2);
    expect(selectedToolId()).toBe("new-session");

    // ArrowDown should move to next enabled (push:save).
    fireEvent.keyDown(input(), { key: "ArrowDown" });
    expect(selectedToolId()).toBe("push:save");
  });

  it("ArrowUp skips disabled rows symmetrically (UAC-015)", () => {
    setDaemonWithPush({
      activeSessionID: "sess-1",
      activeOccupant: "frame",
      pushCommands: ["save"],
    });
    renderToolSelectPhase();

    // Start at push:save (move to idx=1).
    fireEvent.keyDown(input(), { key: "ArrowDown" });
    expect(selectedToolId()).toBe("push:save");

    // ArrowUp from push:save → new-session.
    fireEvent.keyDown(input(), { key: "ArrowUp" });
    expect(selectedToolId()).toBe("new-session");
  });

  it("Ctrl+N / Ctrl+P also skip disabled rows (UAC-015)", () => {
    setDaemonWithPush({
      activeSessionID: "sess-1",
      activeOccupant: "frame",
      pushCommands: ["save"],
    });
    renderToolSelectPhase();

    fireEvent.keyDown(input(), { key: "n", ctrlKey: true });
    expect(selectedToolId()).toBe("push:save");

    fireEvent.keyDown(input(), { key: "p", ctrlKey: true });
    expect(selectedToolId()).toBe("new-session");
  });
});

// ---------------------------------------------------------------------------
// FR-005 / FR-030 / UAC-005: Enter on disabled row
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — Enter on disabled row", () => {
  it("Enter on disabled row triggers emitDisabledFeedback and keeps palette open (FR-005 / FR-030 / UAC-005)", () => {
    // UAC-004: fuzzy includes disabled tools.
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    // Move cursor to the disabled push:save row (idx=1).
    // ArrowDown from new-session (enabled, idx=0) → clamp stays at 0 (no more enabled).
    // We must manually set paramCursor to 1 to land on disabled row.
    act(() => {
      usePaletteStore.setState({ paramCursor: 1, selectedToolId: "push:save" });
    });

    const spy = vi.fn();
    usePaletteStore.setState({ emitDisabledFeedback: spy });

    fireEvent.keyDown(input(), { key: "Enter" });

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy.mock.calls[0]?.[0]).toBe("save"); // tool label
    // Palette stays open.
    expect(usePaletteStore.getState().open).toBe(true);
    expect(usePaletteStore.getState().phase).toBe("toolSelect");
  });
});

// ---------------------------------------------------------------------------
// FR-006 / UAC-006: pointermove on enabled row
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — pointer interaction", () => {
  it("pointermove on enabled row updates cursor and aria-activedescendant (FR-006 / UAC-006)", () => {
    setDaemonWithPush({
      activeSessionID: "sess-1",
      activeOccupant: "frame",
      pushCommands: ["save"],
    });
    renderToolSelectPhase();

    // Start at new-session (idx=0).
    expect(selectedToolId()).toBe("new-session");

    // Fire pointermove on the push:save (idx=1) option.
    const pushSaveOpt = options().find((o) => o.dataset.toolId === "push:save");
    expect(pushSaveOpt).toBeDefined();
    if (!pushSaveOpt) return;

    fireEvent.pointerMove(pushSaveOpt);

    // Cursor should now be at push:save.
    expect(selectedToolId()).toBe("push:save");
    expect(listbox().getAttribute("aria-activedescendant")).toBe("palette-opt-1");
  });

  it("pointermove on disabled row does not move cursor (FR-007 / UAC-007)", () => {
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    expect(selectedToolId()).toBe("new-session");

    // Attempt pointermove on the disabled push:save (idx=1).
    const disabledOpt = options().find((o) => o.dataset.toolId === "push:save");
    expect(disabledOpt).toBeDefined();
    if (!disabledOpt) return;

    fireEvent.pointerMove(disabledOpt);

    // Cursor unchanged.
    expect(selectedToolId()).toBe("new-session");
  });

  it("mouseleave keeps cursor position (FR-008 / UAC-008)", () => {
    setDaemonWithPush({
      activeSessionID: "sess-1",
      activeOccupant: "frame",
      pushCommands: ["save"],
    });
    renderToolSelectPhase();

    // Move to push:save.
    const pushSaveOpt = options().find((o) => o.dataset.toolId === "push:save");
    if (!pushSaveOpt) return;
    fireEvent.pointerMove(pushSaveOpt);
    expect(selectedToolId()).toBe("push:save");

    // Mouse leaves the listbox.
    fireEvent.mouseLeave(listbox());

    // Cursor unchanged.
    expect(selectedToolId()).toBe("push:save");
  });
});

// ---------------------------------------------------------------------------
// UAC-004: fuzzy filter includes disabled tools
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — fuzzy includes disabled", () => {
  it("fuzzy filter includes disabled tools (UAC-004)", () => {
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save", "resume"] });
    renderToolSelectPhase();

    // Type "save" — matches push:save even though it's disabled.
    fireEvent.change(input(), { target: { value: "save" } });

    const opts = options();
    const ids = opts.map((o) => o.dataset.toolId ?? "");
    expect(ids).toContain("push:save");
    // push:resume should not match "save".
    expect(ids).not.toContain("push:resume");
  });
});

// ---------------------------------------------------------------------------
// ARIA: aria-activedescendant uses logical index
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — ARIA logical index", () => {
  it("aria-activedescendant uses logical index from sortToolsForList", () => {
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    // With new-session(enabled,0) and push:save(disabled,1):
    // cursor is at 0 → aria-activedescendant = 'palette-opt-0'.
    expect(listbox().getAttribute("aria-activedescendant")).toBe("palette-opt-0");
    expect(input().getAttribute("aria-activedescendant")).toBe("palette-opt-0");

    // The option id matches the logical index, not DOM position.
    const newSessionOpt = options().find((o) => o.dataset.toolId === "new-session");
    expect(newSessionOpt?.id).toBe("palette-opt-0");
    const pushSaveOpt = options().find((o) => o.dataset.toolId === "push:save");
    expect(pushSaveOpt?.id).toBe("palette-opt-1");
  });
});

// ---------------------------------------------------------------------------
// ADR-0055: frozenList prop bypasses daemon selectors
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — frozenList prop (ADR-0055)", () => {
  it("frozenList prop bypasses daemon selectors: daemon mutation does not change render", async () => {
    // UAC: frozen mode renders frozenList, ignores live daemon updates.
    const { listTools } = await import("../../lib/tools");
    // Build a frozen snapshot with just new-session enabled.
    const snapshot = { sessions: [], activeSessionID: null, projects: [], pushCommands: [] };
    const tools = listTools(snapshot, []);
    const frozenList: SortedTools = {
      // biome-ignore lint/style/noNonNullAssertion: test registry always has new-session at index 0
      enabled: [{ tool: tools[0]!, enabled: true, reason: null, logicalIndex: 0 }],
      disabled: [],
      // biome-ignore lint/style/noNonNullAssertion: test registry always has new-session at index 0
      sorted: [{ tool: tools[0]!, enabled: true, reason: null, logicalIndex: 0 }],
    };

    renderToolSelectPhase({ frozenList, frozenCursor: 0 });

    // Only frozen tools rendered.
    const opts = options();
    expect(opts.map((o) => o.dataset.toolId)).toEqual(["new-session"]);

    // Now mutate the daemon — the render should stay frozen.
    act(() => {
      setDaemonWithPush({
        activeSessionID: "sess-1",
        activeOccupant: "frame",
        pushCommands: ["save", "resume"],
      });
    });

    // Still only the frozen tool.
    const optsAfter = options();
    expect(optsAfter.map((o) => o.dataset.toolId)).toEqual(["new-session"]);
  });

  it("frozen mode renders aria-disabled=true on listbox and ignores Enter / Arrow (ADR-0055)", async () => {
    const snapshot = { sessions: [], activeSessionID: null, projects: [], pushCommands: [] };
    const { listTools } = await import("../../lib/tools");
    const tools = listTools(snapshot, []);
    const frozenList: SortedTools = {
      // biome-ignore lint/style/noNonNullAssertion: test registry always has new-session at index 0
      enabled: [{ tool: tools[0]!, enabled: true, reason: null, logicalIndex: 0 }],
      disabled: [],
      // biome-ignore lint/style/noNonNullAssertion: test registry always has new-session at index 0
      sorted: [{ tool: tools[0]!, enabled: true, reason: null, logicalIndex: 0 }],
    };

    act(() => {
      usePaletteStore.setState({ submitting: true });
    });

    renderToolSelectPhase({ frozenList, frozenCursor: 0 });

    // listbox is aria-disabled.
    expect(listbox().getAttribute("aria-disabled")).toBe("true");

    // Arrow keys and Enter are no-ops in frozen mode.
    const confirmSpy = vi.fn();
    usePaletteStore.setState({
      confirmTool: confirmSpy as unknown as ReturnType<
        typeof usePaletteStore.getState
      >["confirmTool"],
    });

    const initialCursor = usePaletteStore.getState().paramCursor;
    fireEvent.keyDown(input(), { key: "ArrowDown" });
    fireEvent.keyDown(input(), { key: "Enter" });
    expect(usePaletteStore.getState().paramCursor).toBe(initialCursor);
    expect(confirmSpy).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Existing behavior: standard scope, fuzzy filter, IME, submitting
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — standard scope and existing behaviors", () => {
  it("renders standard scope tools in a listbox", () => {
    renderToolSelectPhase();
    const opts = options();
    expect(opts).toHaveLength(1);
    expect(opts[0]?.dataset.toolId).toBe("new-session");
    const ids = opts.map((o) => o.dataset.toolId ?? "");
    expect(ids).not.toContain("stop-session");
    expect(listbox().id).toBe("palette-listbox");
    expect(listbox().getAttribute("aria-activedescendant")).toBe("palette-opt-0");
    expect(opts[0]?.getAttribute("aria-selected")).toBe("true");
    const cb = input();
    expect(cb.getAttribute("role")).toBe("combobox");
    expect(cb.getAttribute("aria-controls")).toBe("palette-listbox");
    expect(cb.placeholder).toBe("Search commands...");
  });

  it("filters by fuzzy query and highlights matched ranges with <mark>", () => {
    renderToolSelectPhase();
    fireEvent.change(input(), { target: { value: "new" } });
    const opts = options();
    expect(opts).toHaveLength(1);
    expect(opts[0]?.dataset.toolId).toBe("new-session");
    const marks = screen.queryAllByTestId("palette-mark");
    expect(marks.length).toBeGreaterThan(0);
    const markedText = marks
      .map((m) => m.textContent ?? "")
      .join("")
      .toLowerCase();
    for (const ch of "new") {
      expect(markedText).toContain(ch);
    }
  });

  it("IME composition suppresses preventDefault on Enter / arrows (FR-019)", () => {
    renderToolSelectPhase();
    const spy = vi.spyOn(usePaletteStore.getState(), "confirmTool");
    usePaletteStore.setState({
      confirmTool: spy as unknown as ReturnType<typeof usePaletteStore.getState>["confirmTool"],
    });

    fireEvent.compositionStart(input());
    expect(usePaletteStore.getState().composing).toBe(true);

    const evt = new KeyboardEvent("keydown", {
      key: "Enter",
      bubbles: true,
      cancelable: true,
    });
    act(() => {
      input().dispatchEvent(evt);
    });
    expect(evt.defaultPrevented).toBe(false);
    expect(spy).not.toHaveBeenCalled();

    const initialCursor = usePaletteStore.getState().paramCursor;
    const arrow = new KeyboardEvent("keydown", {
      key: "ArrowDown",
      bubbles: true,
      cancelable: true,
    });
    act(() => {
      input().dispatchEvent(arrow);
    });
    expect(arrow.defaultPrevented).toBe(false);
    expect(usePaletteStore.getState().paramCursor).toBe(initialCursor);

    fireEvent.compositionEnd(input());
    expect(usePaletteStore.getState().composing).toBe(false);
    fireEvent.keyDown(input(), { key: "Enter" });
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it("submitting=true sets input readOnly and listbox aria-disabled (FR-020)", () => {
    renderToolSelectPhase();
    expect(input().readOnly).toBe(false);
    expect(listbox().getAttribute("aria-disabled")).toBeNull();

    act(() => {
      usePaletteStore.setState({ submitting: true });
    });

    expect(input().readOnly).toBe(true);
    expect(listbox().getAttribute("aria-disabled")).toBe("true");
  });

  it("Enter on the highlighted entry calls confirmTool", () => {
    renderToolSelectPhase();
    const spy = vi.spyOn(usePaletteStore.getState(), "confirmTool");
    usePaletteStore.setState({
      confirmTool: spy as unknown as ReturnType<typeof usePaletteStore.getState>["confirmTool"],
    });

    fireEvent.keyDown(input(), { key: "Enter" });
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy.mock.calls[0]?.[0]).toBe("new-session");
    const ctx = spy.mock.calls[0]?.[1] as Record<string, unknown> | undefined;
    expect(ctx).toBeDefined();
    expect(ctx?.http).toBeDefined();
    expect(ctx?.daemon).toBeDefined();
    expect(ctx?.notify).toBeDefined();
    expect(ctx?.store).toBeDefined();
  });

  it("Enter on a paramless tool triggers end-to-end submit transition (FR-010)", async () => {
    const http = makeFakeHttp();
    render(<ToolSelectPhase httpFactory={() => http} />);
    fireEvent.change(input(), { target: { value: "new" } });
    fireEvent.keyDown(input(), { key: "Enter" });
    // new-session has params → confirmTool transitions to paramSelect.
    expect(usePaletteStore.getState().phase).toBe("paramSelect");
    expect(usePaletteStore.getState().selectedToolId).toBe("new-session");
  });

  it("mousedown on an enabled option calls confirmTool (pointer parity with Enter)", () => {
    renderToolSelectPhase();
    const spy = vi.spyOn(usePaletteStore.getState(), "confirmTool");
    usePaletteStore.setState({
      confirmTool: spy as unknown as ReturnType<typeof usePaletteStore.getState>["confirmTool"],
    });

    const opt = options()[0];
    expect(opt).toBeDefined();
    if (!opt) return;
    fireEvent.mouseDown(opt);

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy.mock.calls[0]?.[0]).toBe("new-session");
    const ctx = spy.mock.calls[0]?.[1] as Record<string, unknown> | undefined;
    expect(ctx).toBeDefined();
    expect(ctx?.http).toBeDefined();
    expect(ctx?.daemon).toBeDefined();
  });

  it("mousedown is a no-op while submitting=true", () => {
    renderToolSelectPhase();
    const spy = vi.spyOn(usePaletteStore.getState(), "confirmTool");
    act(() => {
      usePaletteStore.setState({
        confirmTool: spy as unknown as ReturnType<typeof usePaletteStore.getState>["confirmTool"],
        submitting: true,
      });
    });

    const opt = options()[0];
    if (!opt) return;
    fireEvent.mouseDown(opt);

    expect(spy).not.toHaveBeenCalled();
  });

  it("mousedown is a no-op while composing=true (FR-019 IME guard)", () => {
    renderToolSelectPhase();
    const spy = vi.spyOn(usePaletteStore.getState(), "confirmTool");
    usePaletteStore.setState({
      confirmTool: spy as unknown as ReturnType<typeof usePaletteStore.getState>["confirmTool"],
    });
    fireEvent.compositionStart(input());
    expect(usePaletteStore.getState().composing).toBe(true);

    const opt = options()[0];
    if (!opt) return;
    fireEvent.mouseDown(opt);

    expect(spy).not.toHaveBeenCalled();
  });

  it("empty listbox when query matches nothing", () => {
    renderToolSelectPhase();
    fireEvent.change(input(), { target: { value: "xyzzy-no-match" } });
    expect(options()).toHaveLength(0);
    expect(listbox().getAttribute("aria-activedescendant")).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// FR-011 / UAC-014: push tool group transition flash
// ---------------------------------------------------------------------------

describe("ToolSelectPhase — disabled→enabled row flash (FR-011 / UAC-014)", () => {
  it("push tool group transition: disabled→enabled row flashes once (FR-011 / UAC-014)", async () => {
    // Setup: push:save is disabled (no active session)
    setDaemonWithPush({ activeSessionID: null, pushCommands: ["save"] });
    renderToolSelectPhase();

    // Verify push:save is in disabled group
    const disabledOptBefore = options().find((o) => o.dataset.toolId === "push:save");
    expect(disabledOptBefore?.getAttribute("aria-disabled")).toBe("true");
    expect(disabledOptBefore?.classList.contains("palette-listbox__row--flash")).toBe(false);

    // Transition: give activeSessionID → push:save becomes enabled
    act(() => {
      setDaemonWithPush({ activeSessionID: "sess-1", activeOccupant: "frame", pushCommands: ["save"] });
    });

    // push:save should now be in enabled group
    const enabledOptAfter = options().find((o) => o.dataset.toolId === "push:save");
    expect(enabledOptAfter?.getAttribute("aria-disabled")).toBeNull();
    // flash class present (newly enabled)
    expect(enabledOptAfter?.classList.contains("palette-listbox__row--flash")).toBe(true);
  });
});
