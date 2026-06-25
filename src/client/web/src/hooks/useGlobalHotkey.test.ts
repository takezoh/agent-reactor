import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../store/palette";
import { useGlobalHotkey } from "./useGlobalHotkey";

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// setPlatform overrides navigator.platform for a single test. The descriptor
// dance lets us restore the original between tests because some happy-dom
// versions make platform read-only by default.
function setPlatform(value: string): void {
  Object.defineProperty(navigator, "platform", {
    value,
    configurable: true,
    writable: true,
  });
}

// dispatchKey constructs a KeyboardEvent on the *capture* target we install
// our listener on (document) so that bubble-phase only listeners (e.g. xterm
// textarea) are guaranteed to not have intercepted it. We dispatch on
// document.body so the event bubbles up through document; capture-phase
// listeners on document fire first.
function dispatchKey(init: KeyboardEventInit): KeyboardEvent {
  const ev = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  document.body.dispatchEvent(ev);
  return ev;
}

// installSpies replaces the store actions with vi.fn() spies via setState so
// useGlobalHotkey's call sites become observable without touching the real
// reducer paths (which have side effects we don't want under test here —
// e.g. reading daemon snapshot).
function installSpies(): {
  openPalette: ReturnType<typeof vi.fn>;
  refocusInput: ReturnType<typeof vi.fn>;
} {
  const openPalette = vi.fn();
  const refocusInput = vi.fn();
  usePaletteStore.setState({
    openPalette,
    refocusInput,
  });
  return { openPalette, refocusInput };
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

describe("useGlobalHotkey", () => {
  const originalPlatform = navigator.platform;
  // Capture the real action references once at module load so we can restore
  // them between tests after installSpies() swaps in vi.fn() replacements.
  const realOpenPalette = usePaletteStore.getState().openPalette;
  const realRefocusInput = usePaletteStore.getState().refocusInput;

  beforeEach(() => {
    // Always start each test with the palette closed, a fresh refocusSeq, and
    // the real action slots restored. Tests that want to observe call sites
    // call installSpies() explicitly to swap them for vi.fn()s.
    usePaletteStore.setState({
      open: false,
      refocusSeq: 0,
      openPalette: realOpenPalette,
      refocusInput: realRefocusInput,
    });
  });

  afterEach(() => {
    setPlatform(originalPlatform);
    vi.restoreAllMocks();
  });

  it("on mac (metaKey+K) opens the palette via store.openPalette", () => {
    setPlatform("MacIntel");
    const { openPalette, refocusInput } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    const ev = dispatchKey({ key: "k", metaKey: true });

    expect(openPalette).toHaveBeenCalledTimes(1);
    expect(refocusInput).not.toHaveBeenCalled();
    // FR-001: preventDefault is mandatory so Firefox's Ctrl+K search bar
    // (and Safari/Edge equivalents) don't steal focus.
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  it("on linux (ctrlKey+K) opens the palette via store.openPalette", () => {
    setPlatform("Linux x86_64");
    const { openPalette, refocusInput } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    const ev = dispatchKey({ key: "k", ctrlKey: true });

    expect(openPalette).toHaveBeenCalledTimes(1);
    expect(refocusInput).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  it("on windows (ctrlKey+K) opens the palette via store.openPalette", () => {
    setPlatform("Win32");
    const { openPalette } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    dispatchKey({ key: "k", ctrlKey: true });

    expect(openPalette).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("on mac, ctrlKey+K (no metaKey) is ignored — mac requires Cmd, not Ctrl", () => {
    setPlatform("MacIntel");
    const { openPalette, refocusInput } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    const ev = dispatchKey({ key: "k", ctrlKey: true });

    expect(openPalette).not.toHaveBeenCalled();
    expect(refocusInput).not.toHaveBeenCalled();
    // The handler must NOT call preventDefault for chords it does not own —
    // otherwise terminal/browser Ctrl+K bindings would silently break on mac.
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  it("on linux, metaKey+K (no ctrlKey) is ignored — non-mac requires Ctrl, not Meta", () => {
    setPlatform("Linux x86_64");
    const { openPalette, refocusInput } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    const ev = dispatchKey({ key: "k", metaKey: true });

    expect(openPalette).not.toHaveBeenCalled();
    expect(refocusInput).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  it("non-K keys with the modifier are ignored (e.g. Cmd+J on mac)", () => {
    setPlatform("MacIntel");
    const { openPalette, refocusInput } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    dispatchKey({ key: "j", metaKey: true });
    dispatchKey({ key: "Enter", metaKey: true });

    expect(openPalette).not.toHaveBeenCalled();
    expect(refocusInput).not.toHaveBeenCalled();

    unmount();
  });

  it("uppercase K (Shift+Cmd+K) still matches — chord is case-insensitive on key", () => {
    setPlatform("MacIntel");
    const { openPalette } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());

    // Some browsers report "K" (uppercase) when Shift is held even though
    // Cmd+Shift+K is the same chord intent as Cmd+K from the user's POV.
    dispatchKey({ key: "K", metaKey: true, shiftKey: true });

    expect(openPalette).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("when already open, the hotkey calls refocusInput() — NOT openPalette() (FR-029)", () => {
    setPlatform("MacIntel");
    const { openPalette, refocusInput } = installSpies();
    // Simulate "palette is already open".
    usePaletteStore.setState({ open: true });

    const { unmount } = renderHook(() => useGlobalHotkey());

    dispatchKey({ key: "k", metaKey: true });

    expect(openPalette).not.toHaveBeenCalled();
    expect(refocusInput).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("three rapid presses while open preserve phase / paramValues / query (FR-029 idempotent)", () => {
    setPlatform("MacIntel");
    // Use the REAL refocusInput (default store action) here to assert the
    // refocusSeq counter increments — that proves the signal reached the
    // store and that the surrounding state was not reset on each press.
    usePaletteStore.setState({
      open: true,
      phase: "paramSelect",
      // biome-ignore lint/suspicious/noExplicitAny: test fixture for a half-filled form
      paramValues: { ref: "feat/x" } as any,
      query: "rebase",
      refocusSeq: 0,
    });

    const { unmount } = renderHook(() => useGlobalHotkey());

    dispatchKey({ key: "k", metaKey: true });
    dispatchKey({ key: "k", metaKey: true });
    dispatchKey({ key: "k", metaKey: true });

    const s = usePaletteStore.getState();
    expect(s.refocusSeq).toBe(3);
    // FR-029: in-flight state survives repeated hotkey presses.
    expect(s.open).toBe(true);
    expect(s.phase).toBe("paramSelect");
    expect(s.paramValues).toEqual({ ref: "feat/x" });
    expect(s.query).toBe("rebase");

    unmount();
  });

  it("registers the listener on document with capture: true", () => {
    setPlatform("MacIntel");
    const addSpy = vi.spyOn(document, "addEventListener");
    const removeSpy = vi.spyOn(document, "removeEventListener");

    const { unmount } = renderHook(() => useGlobalHotkey());

    // ADR-0037 / FR-001: capture phase is load-bearing because xterm.js'
    // textarea consumes keydown on bubble phase. Anything other than
    // { capture: true } here would silently regress the terminal-focus case.
    const calls = addSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(calls).toHaveLength(1);
    expect(calls[0]?.[2]).toEqual({ capture: true });

    unmount();

    // Cleanup symmetry: removeEventListener must also pass { capture: true }
    // (different capture flags register/remove distinct slots).
    const rcalls = removeSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(rcalls).toHaveLength(1);
    expect(rcalls[0]?.[2]).toEqual({ capture: true });
  });

  it("listener fires even when an element (xterm textarea proxy) has focus and would consume bubble", () => {
    setPlatform("MacIntel");
    const { openPalette } = installSpies();

    // Simulate xterm's bubble-phase consumer: an element that calls
    // stopPropagation/preventDefault on keydown during the bubble phase.
    // Because we register on capture, our handler must fire BEFORE this one
    // and still see the event.
    const textarea = document.createElement("textarea");
    document.body.appendChild(textarea);
    const bubbleConsumer = vi.fn((e: Event) => {
      e.stopPropagation();
      e.preventDefault();
    });
    textarea.addEventListener("keydown", bubbleConsumer);
    textarea.focus();

    const { unmount } = renderHook(() => useGlobalHotkey());

    const ev = new KeyboardEvent("keydown", {
      key: "k",
      metaKey: true,
      bubbles: true,
      cancelable: true,
    });
    textarea.dispatchEvent(ev);

    expect(openPalette).toHaveBeenCalledTimes(1);

    textarea.remove();
    unmount();
  });

  it("unmount removes the listener so re-mount does not stack handlers", () => {
    setPlatform("MacIntel");
    const { openPalette } = installSpies();
    const { unmount } = renderHook(() => useGlobalHotkey());
    unmount();

    dispatchKey({ key: "k", metaKey: true });
    expect(openPalette).not.toHaveBeenCalled();
  });
});
