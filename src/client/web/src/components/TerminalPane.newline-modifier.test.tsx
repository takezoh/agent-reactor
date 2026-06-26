// TerminalPane.newline-modifier.test.tsx — Shift+Enter / Alt+Enter modifier
// path that injects a literal newline (`\<CR>`) so the user does not have to
// type the backslash by hand. Covers:
//   1. handleNewlineModifier as a pure function (every modifier combo).
//   2. TerminalPane wires the handler into term.attachCustomKeyEventHandler
//      and forwards `\\\r` through conn.send when the modifier fires.

import { render } from "@testing-library/react";
import { Terminal } from "@xterm/xterm";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Connection } from "../socket/connection";
import { TerminalPane, handleNewlineModifier } from "./TerminalPane";

function makeFakeConn(): Connection {
  let _onOutput: ((frame: [number, string, string, string]) => void) | undefined;
  return {
    subscribe: vi.fn(async () => {}),
    unsubscribe: vi.fn(async () => {}),
    send: vi.fn(),
    get onOutput() {
      return _onOutput;
    },
    set onOutput(cb) {
      _onOutput = cb;
    },
  } as unknown as Connection;
}

function makeKeyEvent(
  init: Partial<KeyboardEventInit & { type: string; isComposing: boolean }>,
): KeyboardEvent {
  // happy-dom's KeyboardEvent supports modifier flags via the standard init.
  // We default to keydown because handleNewlineModifier only fires on keydown.
  const ev = new KeyboardEvent(init.type ?? "keydown", {
    key: init.key,
    shiftKey: init.shiftKey,
    altKey: init.altKey,
    ctrlKey: init.ctrlKey,
    metaKey: init.metaKey,
    isComposing: init.isComposing,
    bubbles: true,
    cancelable: true,
  });
  // happy-dom does not always reflect the isComposing init field onto the
  // event property; pin it directly so the handler sees the intended value.
  if (init.isComposing !== undefined) {
    Object.defineProperty(ev, "isComposing", { value: init.isComposing, configurable: true });
  }
  return ev;
}

describe("handleNewlineModifier (pure)", () => {
  it("Shift+Enter sends `\\\\\\r` and tells xterm to skip default handling", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "Enter", shiftKey: true }), send);
    expect(send).toHaveBeenCalledTimes(1);
    expect(send).toHaveBeenCalledWith("\\\r");
    expect(ret).toBe(false);
  });

  it("Alt+Enter sends `\\\\\\r` and tells xterm to skip default handling", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "Enter", altKey: true }), send);
    expect(send).toHaveBeenCalledTimes(1);
    expect(send).toHaveBeenCalledWith("\\\r");
    expect(ret).toBe(false);
  });

  it("bare Enter passes through (returns true, nothing sent)", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "Enter" }), send);
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("Ctrl+Enter passes through (only Shift / Alt are reserved for newline)", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "Enter", ctrlKey: true }), send);
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("Meta+Enter passes through (only Shift / Alt are reserved for newline)", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "Enter", metaKey: true }), send);
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("Shift+letter (non-Enter) passes through", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(makeKeyEvent({ key: "a", shiftKey: true }), send);
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("keyup events pass through even when Shift+Enter (only keydown triggers)", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(
      makeKeyEvent({ type: "keyup", key: "Enter", shiftKey: true }),
      send,
    );
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("Shift+Enter during IME composition passes through (no `\\\\\\r` flush)", () => {
    const send = vi.fn();
    const ret = handleNewlineModifier(
      makeKeyEvent({ key: "Enter", shiftKey: true, isComposing: true }),
      send,
    );
    // IME composition confirm must not leak `\<CR>` into the prompt — the
    // user is still mid-conversion, the Enter belongs to the IME.
    expect(send).not.toHaveBeenCalled();
    expect(ret).toBe(true);
  });

  it("Shift+Enter calls preventDefault on the underlying KeyboardEvent", () => {
    const send = vi.fn();
    const ev = makeKeyEvent({ key: "Enter", shiftKey: true });
    handleNewlineModifier(ev, send);
    // Without preventDefault the browser default would still write `\n` into
    // the .xterm-helper-textarea once xterm's keymap is skipped.
    expect(ev.defaultPrevented).toBe(true);
  });

  it("bare Enter does NOT call preventDefault (xterm owns the suppression)", () => {
    const send = vi.fn();
    const ev = makeKeyEvent({ key: "Enter" });
    handleNewlineModifier(ev, send);
    expect(ev.defaultPrevented).toBe(false);
  });
});

describe("TerminalPane custom key handler wiring", () => {
  let capturedHandler: ((e: KeyboardEvent) => boolean) | undefined;

  beforeEach(() => {
    capturedHandler = undefined;
    // Capture the handler TerminalPane registers so we can fire keys at it
    // without depending on the real xterm DOM (happy-dom + the FakeTerminal
    // mock would not deliver a real KeyboardEvent through the helper textarea).
    vi.spyOn(
      Terminal.prototype as unknown as {
        attachCustomKeyEventHandler: (cb: (e: KeyboardEvent) => boolean) => void;
      },
      "attachCustomKeyEventHandler",
    ).mockImplementation((cb: (e: KeyboardEvent) => boolean) => {
      capturedHandler = cb;
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("registers a custom key handler on mount", () => {
    const conn = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="sess-A" />);
    expect(capturedHandler).toBeTypeOf("function");
    unmount();
  });

  it("Shift+Enter routes `\\\\\\r` through conn.send with the active sessionId", () => {
    const conn = makeFakeConn();
    const sendMock = conn.send as ReturnType<typeof vi.fn>;
    const { unmount } = render(<TerminalPane conn={conn} sessionId="sess-A" />);

    expect(capturedHandler).toBeTypeOf("function");
    const ret = capturedHandler?.(makeKeyEvent({ key: "Enter", shiftKey: true }));

    expect(ret).toBe(false);
    expect(sendMock).toHaveBeenCalledWith({ k: "i", d: "\\\r", sessionId: "sess-A" });
    unmount();
  });

  it("Alt+Enter routes `\\\\\\r` through conn.send with the active sessionId", () => {
    const conn = makeFakeConn();
    const sendMock = conn.send as ReturnType<typeof vi.fn>;
    const { unmount } = render(<TerminalPane conn={conn} sessionId="sess-B" />);

    capturedHandler?.(makeKeyEvent({ key: "Enter", altKey: true }));
    expect(sendMock).toHaveBeenCalledWith({ k: "i", d: "\\\r", sessionId: "sess-B" });
    unmount();
  });

  it("bare Enter is left to xterm's onData path (handler returns true, conn.send is not called by the handler)", () => {
    const conn = makeFakeConn();
    const sendMock = conn.send as ReturnType<typeof vi.fn>;
    const { unmount } = render(<TerminalPane conn={conn} sessionId="sess-C" />);

    const ret = capturedHandler?.(makeKeyEvent({ key: "Enter" }));
    expect(ret).toBe(true);
    // The handler must not synthesize a send for bare Enter — that is xterm's
    // job via term.onData (covered by TerminalPane.pc-baseline.test.tsx).
    expect(sendMock).not.toHaveBeenCalledWith({ k: "i", d: "\\\r", sessionId: "sess-C" });
    unmount();
  });

  it("Shift+Enter with no active session drops the input (parity with onData guard)", () => {
    const conn = makeFakeConn();
    const sendMock = conn.send as ReturnType<typeof vi.fn>;
    const { unmount } = render(<TerminalPane conn={conn} sessionId={null} />);

    const ret = capturedHandler?.(makeKeyEvent({ key: "Enter", shiftKey: true }));
    // Still false — we don't want xterm's default CR to leak when the user
    // pressed Shift+Enter even though we have no session to send it to.
    expect(ret).toBe(false);
    expect(sendMock).not.toHaveBeenCalled();
    unmount();
  });
});
