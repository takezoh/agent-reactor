// UAC-009 / FR-018 / FR-023 / NFR-006
import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../store/palette";
import { type ChipHotkeyOptions, useChipHotkey } from "./useChipHotkey";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dispatchKey fires a capture-visible keydown event on document.body so the
// capture-phase listener on document sees it before any bubble-phase consumer.
function dispatchKey(init: KeyboardEventInit): KeyboardEvent {
  const ev = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  document.body.dispatchEvent(ev);
  return ev;
}

// defaultOpts returns options that put the hook in the "fully active" state.
function defaultOpts(): ChipHotkeyOptions {
  return {
    worktreeChipVisible: true,
    hostChipVisible: true,
    commandFieldVisible: true,
  };
}

// installSpies replaces toggleWorktree / toggleHost with vi.fn() spies via
// setState so call sites are observable without triggering real reducer paths.
function installSpies(): {
  toggleWorktree: ReturnType<typeof vi.fn>;
  toggleHost: ReturnType<typeof vi.fn>;
} {
  const toggleWorktree = vi.fn();
  const toggleHost = vi.fn();
  usePaletteStore.setState({ toggleWorktree, toggleHost });
  return { toggleWorktree, toggleHost };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useChipHotkey", () => {
  const realToggleWorktree = usePaletteStore.getState().toggleWorktree;
  const realToggleHost = usePaletteStore.getState().toggleHost;

  beforeEach(() => {
    // Reset to a clean open paramSelect state with real actions restored.
    // Individual tests call installSpies() to swap in vi.fn() replacements.
    usePaletteStore.setState({
      open: true,
      phase: "paramSelect",
      composing: false,
      toggleWorktree: realToggleWorktree,
      toggleHost: realToggleHost,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // Happy-path: Alt+W toggles worktree chip
  // -------------------------------------------------------------------------

  it("Alt+W (KeyW + altKey) calls toggleWorktree and calls preventDefault (FR-018 / UAC-009)", () => {
    // UAC-009 / FR-018
    const { toggleWorktree, toggleHost } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).toHaveBeenCalledTimes(1);
    expect(toggleHost).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  // -------------------------------------------------------------------------
  // Happy-path: Alt+H toggles host chip
  // -------------------------------------------------------------------------

  it("Alt+H (KeyH + altKey) calls toggleHost and calls preventDefault (FR-018 / UAC-009)", () => {
    // UAC-009 / FR-018
    const { toggleWorktree, toggleHost } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyH", altKey: true });

    expect(toggleHost).toHaveBeenCalledTimes(1);
    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  // -------------------------------------------------------------------------
  // composing=true is no-op (FR-023)
  // -------------------------------------------------------------------------

  it("composing=true suppresses Alt+W (FR-023)", () => {
    // UAC-009 / FR-023
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ composing: true });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  it("composing=true suppresses Alt+H (FR-023)", () => {
    // UAC-009 / FR-023
    const { toggleHost } = installSpies();
    usePaletteStore.setState({ composing: true });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyH", altKey: true });

    expect(toggleHost).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // phase !== 'paramSelect' is no-op
  // -------------------------------------------------------------------------

  it("phase='toolSelect' suppresses Alt+W", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ phase: "toolSelect" });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // open=false is no-op
  // -------------------------------------------------------------------------

  it("open=false suppresses Alt+W", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ open: false });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // commandFieldVisible=false is no-op
  // -------------------------------------------------------------------------

  it("commandFieldVisible=false suppresses Alt+W", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const opts: ChipHotkeyOptions = { ...defaultOpts(), commandFieldVisible: false };
    const { unmount } = renderHook(() => useChipHotkey(opts));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // worktreeChipVisible=false suppresses only Alt+W (not Alt+H)
  // -------------------------------------------------------------------------

  it("worktreeChipVisible=false suppresses Alt+W but Alt+H still fires toggleHost", () => {
    // UAC-009 / FR-018
    const { toggleWorktree, toggleHost } = installSpies();
    const opts: ChipHotkeyOptions = { ...defaultOpts(), worktreeChipVisible: false };
    const { unmount } = renderHook(() => useChipHotkey(opts));

    const evW = dispatchKey({ code: "KeyW", altKey: true });
    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(evW.defaultPrevented).toBe(false);

    const evH = dispatchKey({ code: "KeyH", altKey: true });
    expect(toggleHost).toHaveBeenCalledTimes(1);
    expect(evH.defaultPrevented).toBe(true);

    unmount();
  });

  // -------------------------------------------------------------------------
  // altKey=false is no-op
  // -------------------------------------------------------------------------

  it("altKey=false does not fire toggleWorktree for KeyW", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: false });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  it("altKey=false does not fire toggleHost for KeyH", () => {
    // UAC-009 / FR-018
    const { toggleHost } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyH", altKey: false });

    expect(toggleHost).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // Unrelated key codes with altKey are no-op
  // -------------------------------------------------------------------------

  it("event.code='KeyA' + altKey=true does not fire any action", () => {
    // UAC-009 / FR-018
    const { toggleWorktree, toggleHost } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyA", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(toggleHost).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // Capture-phase registration
  // -------------------------------------------------------------------------

  it("registers listener on document with capture=true and removes on unmount (NFR-006)", () => {
    // NFR-006 / FR-018
    const addSpy = vi.spyOn(document, "addEventListener");
    const removeSpy = vi.spyOn(document, "removeEventListener");

    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const addCalls = addSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(addCalls).toHaveLength(1);
    // The third argument must be the boolean `true` (capture phase).
    expect(addCalls[0]?.[2]).toBe(true);

    unmount();

    const removeCalls = removeSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(removeCalls).toHaveLength(1);
    expect(removeCalls[0]?.[2]).toBe(true);
  });

  // -------------------------------------------------------------------------
  // Unmount removes the listener — subsequent keydown is silently ignored
  // -------------------------------------------------------------------------

  it("after unmount, Alt+W no longer calls toggleWorktree", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));
    unmount();

    dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
  });

  it("after unmount, Alt+H no longer calls toggleHost", () => {
    // UAC-009 / FR-018
    const { toggleHost } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));
    unmount();

    dispatchKey({ code: "KeyH", altKey: true });

    expect(toggleHost).not.toHaveBeenCalled();
  });
});
