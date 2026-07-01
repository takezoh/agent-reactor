import { act, fireEvent, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HINT_SEEN_KEY } from "../hooks/useCoachmarkOnce";
import { MOBILE_GATE_QUERY } from "../hooks/useMobileGate";
import { FAB_OFFSET_PROP } from "../hooks/useVisualViewportLift";
import type { Connection } from "../socket/connection";
import {
  type MatchMediaHandle,
  mockMatchMedia,
  mockVisualViewport,
  pinchByRatio,
  swipeFromTo,
  tapAt,
} from "../test/touch-harness";
import { TerminalPane } from "./TerminalPane";

// Spec aria-labels, \u-escaped (ADR-0049 english-only source), decoded values:
//   ARIA_KEYBOARD_OPEN  → KeyboardFAB idle label (open keyboard)
//   ARIA_KEYBOARD_CLOSE → KeyboardFAB input-mode label (close keyboard)
//   ARIA_FONT_SIZE      → FontSizeControl trigger label (font size)
const ARIA_KEYBOARD_OPEN = "\u30AD\u30FC\u30DC\u30FC\u30C9\u3092\u958B\u304F";
const ARIA_KEYBOARD_CLOSE = "\u30AD\u30FC\u30DC\u30FC\u30C9\u3092\u9589\u3058\u308B";
const ARIA_FONT_SIZE = "\u6587\u5B57\u30B5\u30A4\u30BA";
// ARIA_JUMP_LATEST -> JumpToLatestFAB label ('jump to latest', cross-task UAC-014/015)
const ARIA_JUMP_LATEST = "\u6700\u65B0\u3078\u30B9\u30AF\u30ED\u30FC\u30EB";
// VIEW_MODE_TEXT   -> AriaLive announcement on blur/Esc exit ('returned to view mode', UAC-006)
const VIEW_MODE_TEXT = "\u95B2\u89A7\u30E2\u30FC\u30C9\u306B\u623B\u308A\u307E\u3057\u305F";

// ---------------------------------------------------------------------------
// Helpers to grab the mocked FitAddon instance from vi.mock("@xterm/addon-fit")
// ---------------------------------------------------------------------------

// We need a spy on fit.fit() — re-open the mock to capture instance calls.
// The mock is defined in test-setup.ts; we extend it here per-test with vi.spyOn
// by reaching into the module mock after importing.

import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import type { SessionConfig } from "../api/sessions";
import { useDaemonStore } from "../store/daemon";

// fontSessionConfig builds a minimal GET /api/session-config client view that
// only carries the [terminal] font_family / font_size fields under test.
function fontSessionConfig(fontFamily: string, fontSize: number): SessionConfig {
  return {
    projectRoots: [],
    projectPaths: [],
    projects: [],
    commands: [],
    pushCommands: [],
    fontFamily,
    fontSize,
  };
}

// ---------------------------------------------------------------------------
// Minimal fakeConn factory (fresh per test to avoid state bleed)
// ---------------------------------------------------------------------------
function makeFakeConn(): {
  conn: Connection;
  capturedOnOutput: () => ((frame: [number, string, string, string]) => void) | undefined;
} {
  let _onOutput: ((frame: [number, string, string, string]) => void) | undefined;
  const conn = {
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
  return {
    conn,
    capturedOnOutput: () => _onOutput,
  };
}

describe("TerminalPane", () => {
  // Spy on FitAddon.prototype.fit across all tests
  let fitSpy: ReturnType<typeof vi.spyOn>;
  let writeSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    writeSpy = vi.spyOn(Terminal.prototype, "write");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // Basic smoke test
  // -------------------------------------------------------------------------
  it("mounts and unmounts without throwing", () => {
    const { conn } = makeFakeConn();
    const { unmount, container } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(container.querySelector(".terminal-host")).not.toBeNull();
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-008: initial fit is called via scheduleFit (rAF) on mount
  // The synchronous rAF mock in test-setup flushes immediately, so fit.fit()
  // should have been called once right after render.
  // -------------------------------------------------------------------------
  it("FR-008: calls fit.fit() on initial mount via scheduleFit (rAF)", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    // rAF mock runs synchronously → fit.fit() should already be called
    expect(fitSpy).toHaveBeenCalledTimes(1);
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-006: __triggerResize fires ResizeObserver callback → fit.fit() called
  // -------------------------------------------------------------------------
  it("FR-006: __triggerResize on host element causes fit.fit() to be called", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;
    expect(host).not.toBeNull();

    const callsBefore = fitSpy.mock.calls.length;
    // Simulate ResizeObserver firing on the host element
    globalThis.__triggerResize(host, []);

    // rAF mock is synchronous so fit.fit() fires immediately
    expect(fitSpy.mock.calls.length).toBeGreaterThan(callsBefore);
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-007: sibling panel size change via ResizeObserver → refit
  // (same mechanic as FR-006; verifying at least one additional call)
  // -------------------------------------------------------------------------
  it("FR-007: ResizeObserver host resize triggers scheduleFit and calls fit.fit()", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;

    const callsBefore = fitSpy.mock.calls.length;
    globalThis.__triggerResize(host, [{ contentRect: { width: 400, height: 300 } }]);

    expect(fitSpy.mock.calls.length).toBeGreaterThan(callsBefore);
    unmount();
  });

  // -------------------------------------------------------------------------
  // NFR-005: rapid consecutive ResizeObserver firings are coalesced into a
  // single rAF tick. With synchronous rAF mock each call resolves immediately,
  // so the pending-flag logic prevents duplicate calls within the same tick.
  // We test by counting fit.fit() calls after N rapid triggers — should be N
  // (one per trigger) but never more, and each rAF resolves synchronously so
  // we can count exactly.
  // Actually with synchronous rAF, each scheduleFit call runs immediately,
  // meaning rafPending flips back to false between calls, so each call to
  // scheduleFit will invoke fit. The NFR is about coalescing within ONE frame.
  // We simulate TWO back-to-back __triggerResize calls without any frame
  // boundary between them by temporarily making rAF queue (not fire immediately).
  // -------------------------------------------------------------------------
  it("NFR-005: rapid ResizeObserver firings in same frame are coalesced to 1 fit.fit() call", () => {
    const { conn } = makeFakeConn();

    // Temporarily override rAF to queue (not fire synchronously) so we can
    // test the pending-flag coalescing behavior.
    const rafQueue: FrameRequestCallback[] = [];
    const origRAF = globalThis.requestAnimationFrame;
    globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => {
      rafQueue.push(cb);
      return rafQueue.length;
    };

    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;

    // Clear calls from mount (initial scheduleFit was queued, not yet run)
    fitSpy.mockClear();

    // Fire resize 5 times rapidly (no frame flush between them)
    const trigger = globalThis.__triggerResize;
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);

    // Before flushing, fit.fit() should not yet have been called
    expect(fitSpy).not.toHaveBeenCalled();

    // Flush all queued rAF callbacks
    const queued = rafQueue.splice(0);
    for (const cb of queued) cb(performance.now());

    // Only 1 fit.fit() should have been called (coalesced)
    expect(fitSpy).toHaveBeenCalledTimes(1);

    // Restore
    globalThis.requestAnimationFrame = origRAF;
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-005: keyed remount — stale output for old sessionId is NOT written
  // to the new TerminalPane instance.
  // -------------------------------------------------------------------------
  it("FR-005: stale output for old sessionId is not written after key-based remount", () => {
    const { conn, capturedOnOutput } = makeFakeConn();

    // Mount with sessionId "s1"
    const { unmount: unmount1 } = render(<TerminalPane key="s1" conn={conn} sessionId="s1" />);
    const onOutputS1 = capturedOnOutput();
    expect(onOutputS1).toBeDefined();

    // Unmount s1 (simulating key change / remount)
    unmount1();
    // After unmount, conn.onOutput should be cleared
    expect(capturedOnOutput()).toBeUndefined();

    // Mount new instance with sessionId "s2"
    const { unmount: unmount2 } = render(<TerminalPane key="s2" conn={conn} sessionId="s2" />);

    writeSpy.mockClear();

    // Deliver stale output tagged with old session "s1" to the s1 handler
    // (simulate the stale callback from before remount)
    if (onOutputS1) {
      // The old handler is detached; calling it would use a disposed terminal.
      // The important assertion is that the NEW instance's onOutput drops
      // frames whose sessionId !== "s2".
      const newHandler = capturedOnOutput();
      expect(newHandler).toBeDefined();
      if (newHandler) {
        // Deliver stale s1 frame to new handler — should be dropped
        newHandler([0, "o", btoa("stale data"), "s1"]);
        expect(writeSpy).not.toHaveBeenCalled();

        // Deliver correct s2 frame — should be written
        newHandler([0, "o", btoa("good data"), "s2"]);
        expect(writeSpy).toHaveBeenCalledTimes(1);
      }
    }

    unmount2();
  });

  // -------------------------------------------------------------------------
  // ADR 0030 lifecycle: a real key remount must hand off conn.onOutput
  // cleanly. Driving the swap through @testing-library's rerender with a
  // changing `key` forces React to unmount the old instance (cleanup runs,
  // conn.onOutput cleared) BEFORE the new instance mounts (new onOutput
  // installed). Without ADR 0030's keyed remount the old effect would keep
  // running and writes for the stale sessionId would land in the new term.
  // -------------------------------------------------------------------------
  it("FR-005/ADR-0030: keyed rerender unmounts old onOutput before new install; new term receives new-session frames only", () => {
    const { conn, capturedOnOutput } = makeFakeConn();

    // Mount with key=s1 → installs onOutput-A bound to s1
    const { rerender, unmount } = render(<TerminalPane key="s1" conn={conn} sessionId="s1" />);
    const onOutputBefore = capturedOnOutput();
    expect(onOutputBefore).toBeDefined();

    // Track the term.write call count seen by ANY Terminal instance — both
    // old and new share Terminal.prototype, so writeSpy spans them.
    writeSpy.mockClear();

    // Force a real key remount. React unmounts the s1 instance (cleanup runs
    // → conn.onOutput cleared, term disposed) and mounts a fresh instance
    // under key=s2 which installs a new onOutput bound to s2.
    rerender(<TerminalPane key="s2" conn={conn} sessionId="s2" />);

    const onOutputAfter = capturedOnOutput();
    expect(onOutputAfter).toBeDefined();
    // The new effect installed a brand-new handler — not the s1 closure.
    expect(onOutputAfter).not.toBe(onOutputBefore);

    // A stale s1-tagged frame arriving on the now-live (s2) handler is
    // dropped by the frame[3] !== sessionRef.current guard.
    if (onOutputAfter) {
      onOutputAfter([0, "o", btoa("stale s1 frame"), "s1"]);
      expect(writeSpy).not.toHaveBeenCalled();

      // The matching s2 frame lands on the new term.
      onOutputAfter([0, "o", btoa("good s2 frame"), "s2"]);
      expect(writeSpy).toHaveBeenCalledTimes(1);
    }

    unmount();
    // Final cleanup leaves conn.onOutput cleared so a future Connection
    // consumer doesn't see a dangling closure into a disposed terminal.
    expect(capturedOnOutput()).toBeUndefined();
  });

  // -------------------------------------------------------------------------
  // Regression: conn.onOutput filter — frame[3] !== sessionRef.current drops
  // -------------------------------------------------------------------------
  it("regression: conn.onOutput drops frames whose sessionId does not match current session", () => {
    const { conn, capturedOnOutput } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="session-A" />);

    writeSpy.mockClear();
    const handler = capturedOnOutput();
    expect(handler).toBeDefined();

    if (handler) {
      // Frame from a different session — must be dropped
      handler([0, "o", btoa("wrong session"), "session-B"]);
      expect(writeSpy).not.toHaveBeenCalled();

      // Frame from the correct session — must be written
      handler([0, "o", btoa("correct"), "session-A"]);
      expect(writeSpy).toHaveBeenCalledTimes(1);
    }

    unmount();
  });

  // -------------------------------------------------------------------------
  // Subscribe / unsubscribe lifecycle
  // -------------------------------------------------------------------------
  it("calls conn.subscribe on mount and conn.unsubscribe on unmount", () => {
    const { conn } = makeFakeConn();
    const subscribeMock = conn.subscribe as ReturnType<typeof vi.fn>;
    const unsubscribeMock = conn.unsubscribe as ReturnType<typeof vi.fn>;

    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(subscribeMock).toHaveBeenCalledWith("s1");
    expect(unsubscribeMock).not.toHaveBeenCalled();

    unmount();
    expect(unsubscribeMock).toHaveBeenCalledWith("s1");
  });

  it("cleans up conn.onOutput on unmount", () => {
    const { conn, capturedOnOutput } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(capturedOnOutput()).toBeDefined();
    unmount();
    expect(capturedOnOutput()).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// FR-THEME-003: terminal-host element uses CSS var(--bg) for background
// (ADR-0059 hybrid bridge)
// ---------------------------------------------------------------------------

describe("FR-THEME-003 — terminal-host background is driven by CSS var(--bg)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject CSS rules so happy-dom can resolve custom properties.
    // The actual app.css rule: .terminal-host { background: var(--bg) !important }
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --bg: #1e1e1e; }
      [data-theme="light"] { --bg: #f5f5f5; }
      [data-theme="dark"]  { --bg: #1e1e1e; }
      .terminal-host { background: var(--bg) !important; }
    `;
    document.head.appendChild(styleEl);
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    styleEl.remove();
    delete document.documentElement.dataset.theme;
  });

  it("terminal-host background resolves to dark --bg when data-theme=dark", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host).not.toBeNull();

    const bg = getComputedStyle(host).backgroundColor;
    // In happy-dom, getComputedStyle resolves custom property values from
    // injected <style> tags. The injected dark rule sets --bg to #1e1e1e so
    // the resolved background should equal that value (not empty, not transparent,
    // not the light value). This verifies token-to-element wiring is active.
    expect(bg).toBe("#1e1e1e");
    unmount();
  });

  it("dark and light --bg resolve to different values", () => {
    const { conn: connDark } = makeFakeConn();
    document.documentElement.dataset.theme = "dark";
    const { container: containerDark, unmount: unmountDark } = render(
      <TerminalPane conn={connDark} sessionId="s1" />,
    );
    const hostDark = containerDark.querySelector(".terminal-host") as HTMLElement;
    const bgDark = getComputedStyle(hostDark).backgroundColor;
    unmountDark();

    const { conn: connLight } = makeFakeConn();
    document.documentElement.dataset.theme = "light";
    const { container: containerLight, unmount: unmountLight } = render(
      <TerminalPane conn={connLight} sessionId="s2" />,
    );
    const hostLight = containerLight.querySelector(".terminal-host") as HTMLElement;
    const bgLight = getComputedStyle(hostLight).backgroundColor;
    unmountLight();

    // Dark and light backgrounds must be distinct values.
    expect(bgDark).not.toBe(bgLight);
  });
});

// ---------------------------------------------------------------------------
// FR-TERMINAL-001: terminal-host height > 0 after resize (ADR-0060 / ADR-0034)
// FR-TERMINAL-002: no double-scroll after render
// ADR-0060 structural: terminal-host CSS uses var(--dvh) + flex:1 1 0 coexist
// ---------------------------------------------------------------------------

describe("FR-TERMINAL-001 — viewport height changes refit terminal-host (ADR-0060 / ADR-0034)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject CSS so getComputedStyle can resolve height token.
    // --dvh defaults to 100vh; in happy-dom 100vh resolves to a px value
    // based on window.innerHeight. We set it explicitly so the test is stable.
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 100vh; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
    vi.restoreAllMocks();
  });

  it("FR-TERMINAL-001: terminal-host getComputedStyle.height > 0 after shrink and expand resize", () => {
    const { conn } = makeFakeConn();
    const fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host).not.toBeNull();

    // Simulate viewport shrink: set innerHeight to 600 and fire resize.
    Object.defineProperty(window, "innerHeight", {
      value: 600,
      writable: true,
      configurable: true,
    });
    window.dispatchEvent(new Event("resize"));
    // rAF mock fires synchronously → fit.fit() called.
    expect(fitSpy.mock.calls.length).toBeGreaterThan(0);

    // terminal-host has height set via CSS var(--dvh) → getComputedStyle.height
    // is non-empty (not "0px") as long as CSS is injected.
    const heightShrink = getComputedStyle(host).height;
    expect(heightShrink).not.toBe("0px");
    expect(heightShrink).not.toBe("");

    // Simulate viewport expand: restore innerHeight to 667 and fire resize.
    fitSpy.mockClear();
    Object.defineProperty(window, "innerHeight", {
      value: 667,
      writable: true,
      configurable: true,
    });
    window.dispatchEvent(new Event("resize"));
    expect(fitSpy.mock.calls.length).toBeGreaterThan(0);

    const heightExpand = getComputedStyle(host).height;
    expect(heightExpand).not.toBe("0px");
    expect(heightExpand).not.toBe("");

    unmount();
  });

  it("FR-TERMINAL-001 rAF coalesce: rapid resize events produce at most 1 fit.fit() per frame", () => {
    const { conn } = makeFakeConn();

    // Use a queuing rAF so we can verify coalescing.
    const rafQueue: FrameRequestCallback[] = [];
    const origRAF = globalThis.requestAnimationFrame;
    globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => {
      rafQueue.push(cb);
      return rafQueue.length;
    };

    const fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    // Drain the initial mount rAF queue within act() to avoid cross-test
    // state contamination. On mount, scheduleFit() and useXtermTheme's
    // rebuild() both queue rAF callbacks. We flush them first so the
    // pending flag resets to false before we start the resize test.
    const mountQueue = rafQueue.splice(0);
    act(() => {
      for (const cb of mountQueue) cb(performance.now());
    });
    fitSpy.mockClear();

    // Fire 5 resize events without flushing the rAF queue.
    for (let i = 0; i < 5; i++) {
      window.dispatchEvent(new Event("resize"));
    }
    // Also trigger 3 ResizeObserver callbacks on the host element.
    for (let i = 0; i < 3; i++) {
      globalThis.__triggerResize(host, []);
    }

    // Before flush: coalesce means pending flag is set, fit.fit() not yet called.
    expect(fitSpy).not.toHaveBeenCalled();

    // Flush queued rAF callbacks: only 1 should have been queued for the
    // resize batch (pending flag prevents additional enqueues).
    const resizeQueue = rafQueue.splice(0);
    expect(resizeQueue.length).toBe(1);
    act(() => {
      for (const cb of resizeQueue) cb(performance.now());
    });
    expect(fitSpy).toHaveBeenCalledTimes(1);

    globalThis.requestAnimationFrame = origRAF;
    unmount();
  });
});

describe("FR-TERMINAL-002 — no double-scroll after render (ADR-0060 / m3 body overflow:hidden)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject body overflow:hidden (guaranteed by m3-app-shell-grid) plus
    // the terminal-host height token so the DOM layout matches production.
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 100vh; }
      html, body, #root { height: 100%; margin: 0; }
      body { overflow: hidden; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
  });

  it("FR-TERMINAL-002: scrollHeight <= innerHeight and scrollWidth <= innerWidth after render", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    // With body overflow:hidden, scrollHeight and scrollWidth must not exceed
    // the viewport dimensions — double-scroll is impossible.
    const scrollH = document.documentElement.scrollHeight;
    const scrollW = document.documentElement.scrollWidth;
    const innerH = window.innerHeight;
    const innerW = window.innerWidth;

    expect(scrollH).toBeLessThanOrEqual(innerH);
    expect(scrollW).toBeLessThanOrEqual(innerW);

    unmount();
  });
});

describe("ADR-0060 structural — terminal-host CSS: var(--dvh) + flex:1 1 0 coexist", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Use a concrete px value for --dvh so happy-dom's getComputedStyle can
    // resolve height: var(--dvh) to a non-empty, non-zero string without
    // needing to resolve viewport-relative units (which happy-dom cannot do
    // for custom properties transitively referencing 100dvh/100vh).
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 768px; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
  });

  it("terminal-host has height CSS property set via var(--dvh)", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    // getComputedStyle.height must resolve to the --dvh concrete value (768px).
    // This confirms the height: var(--dvh) declaration is wired and applied.
    const height = getComputedStyle(host).height;
    expect(height).toBe("768px");

    unmount();
  });

  it("terminal-host retains flex:1 1 0 alongside height:var(--dvh) (ADR-0029 + ADR-0060 coexist)", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    const style = getComputedStyle(host);
    // flex: 1 1 0 expands to flexGrow=1, flexShrink=1, flexBasis=0.
    expect(style.flexGrow).toBe("1");
    expect(style.flexShrink).toBe("1");
    // flexBasis and minHeight may be "0px" or "0" depending on happy-dom version.
    expect(Number.parseFloat(style.flexBasis)).toBe(0);
    expect(Number.parseFloat(style.minHeight)).toBe(0);

    unmount();
  });
});

// ---------------------------------------------------------------------------
// FR-THEME-002: xterm.options.theme is updated within 1 rAF after data-theme
// switches (ADR-0059 hybrid bridge — ITheme side)
// ---------------------------------------------------------------------------

describe("FR-THEME-002 — xterm.options.theme is updated on data-theme change", () => {
  // Capture created Terminal instances so we can inspect options.theme.
  let createdTerminals: Terminal[];
  let constructorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    createdTerminals = [];
    constructorSpy = vi.spyOn(Terminal.prototype, "open").mockImplementation(function (
      this: Terminal,
    ) {
      createdTerminals.push(this);
    });

    // Set dark token values on documentElement so useXtermTheme reads them.
    document.documentElement.style.setProperty("--xterm-fg", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-cursor", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-selection", "rgba(74, 158, 255, 0.3)");
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    constructorSpy.mockRestore();
    createdTerminals = [];
    document.documentElement.style.removeProperty("--xterm-fg");
    document.documentElement.style.removeProperty("--xterm-cursor");
    document.documentElement.style.removeProperty("--xterm-selection");
    delete document.documentElement.dataset.theme;
  });

  it("xterm.options.theme is set on mount with initial ITheme", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    // After mount the terminal should have been opened; theme effect runs after.
    // With synchronous rAF mock, theme is applied immediately.
    // The terminal instance captured via open() spy has options.theme set.
    expect(createdTerminals.length).toBeGreaterThan(0);
    const term = createdTerminals[0];
    const theme = (term as unknown as { options: Record<string, unknown> }).options.theme;
    // The ITheme should have been applied (not undefined).
    expect(theme).toBeDefined();
    expect(typeof theme).toBe("object");

    unmount();
  });

  it("xterm.options.theme updates within 1 rAF after data-theme changes dark→light", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    expect(createdTerminals.length).toBeGreaterThan(0);
    const term = createdTerminals[0] as unknown as { options: Record<string, unknown> };

    // Record initial dark theme.
    const darkTheme = term.options.theme as { foreground?: string } | undefined;
    expect(darkTheme).toBeDefined();

    // Switch to light: update CSS tokens + data-theme then flush MutationObserver.
    act(() => {
      document.documentElement.style.setProperty("--xterm-fg", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-cursor", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-selection", "rgba(0, 102, 204, 0.3)");
      document.documentElement.dataset.theme = "light";
      // Manually fire MutationObserver callbacks — happy-dom does not fire them
      // automatically on documentElement attribute mutations.
      globalThis.flushThemeObservers();
    });

    // rAF runs synchronously → ITheme is rebuilt with light token values.
    const lightTheme = term.options.theme as { foreground?: string } | undefined;
    expect(lightTheme).toBeDefined();
    // foreground token changed from dark (#e6e6e6) to light (#1a1a1a).
    expect((lightTheme as { foreground?: string }).foreground).toBe("#1a1a1a");
    // Dark and light IThemes are distinct objects with different foreground values.
    expect((darkTheme as { foreground?: string }).foreground).not.toBe(
      (lightTheme as { foreground?: string }).foreground,
    );

    unmount();
  });
});

// ===========================================================================
// chunk-07 mobile integration wiring (ADR 0069/0072/0074).
// These exercise TerminalPane under the mobile gate: the overlay set mounts,
// the PC path stays absent under gate false, terminal-host box is invariant
// (UAC-025), visualViewport-lift drives --terminal-fab-offset through the real
// wiring, the coachmark shows once, and pinch→fontSize→refit is end-to-end.
// ===========================================================================

function mobileConn(): Connection {
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

describe("TerminalPane — mobile gate true mounts the overlay set (ADR 0074)", () => {
  let mm: MatchMediaHandle;
  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("renders the fab-layer with KeyboardFAB / FontSizeControl / AriaLiveStatus and data-input-active=false", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    expect(container.querySelector(".terminal-fab-layer")).not.toBeNull();
    expect(container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`)).not.toBeNull();
    expect(container.querySelector(`[aria-label="${ARIA_FONT_SIZE}"]`)).not.toBeNull();
    expect(container.querySelector('[data-testid="terminal-aria-live"]')).not.toBeNull();

    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host.getAttribute("data-input-active")).toBe("false");
    unmount();
  });

  it("KeyboardFAB tap enters input mode: data-input-active=true + aria-pressed/label flip", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const fab = container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;

    act(() => fireEvent.click(fab));

    expect(host.getAttribute("data-input-active")).toBe("true");
    const closed = container.querySelector(`[aria-label="${ARIA_KEYBOARD_CLOSE}"]`) as HTMLElement;
    expect(closed).not.toBeNull();
    expect(closed.getAttribute("aria-pressed")).toBe("true");
    unmount();
  });
});

describe("TerminalPane — mobile gate false keeps the overlay absent (FR-PC-PRESERVE-*)", () => {
  let mm: MatchMediaHandle;
  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: false });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("no fab-layer / FABs / data-input-active when the gate is false", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(container.querySelector(".terminal-fab-layer")).toBeNull();
    expect(container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`)).toBeNull();
    expect(container.querySelector("[data-input-active]")).toBeNull();
    expect(container.querySelector("[data-coachmark]")).toBeNull();
    unmount();
  });
});

describe("TerminalPane — UAC-025 terminal-host box invariant (overlay placement)", () => {
  let mm: MatchMediaHandle;
  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("FABs live in the absolute fab-layer SIBLING, never as in-flow children of terminal-host", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const layer = container.querySelector(".terminal-fab-layer") as HTMLElement;

    // Discriminator vs the counterexample (FAB inserted as a flex child of
    // terminal-host): the FAB and the whole layer must NOT be inside terminal-host.
    expect(host.contains(layer)).toBe(false);
    expect(host.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`)).toBeNull();
    expect(host.querySelector(".terminal-fab-layer")).toBeNull();

    // Spec-named assertion: terminal-host rect unchanged across a FAB state change.
    const before = JSON.stringify(host.getBoundingClientRect());
    act(() =>
      fireEvent.click(
        container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement,
      ),
    );
    const after = JSON.stringify(host.getBoundingClientRect());
    expect(after).toBe(before);
    unmount();
  });
});

describe("TerminalPane — visualViewport-lift drives --terminal-fab-offset (FR-MOB-VVP-001/003)", () => {
  let mm: MatchMediaHandle;
  beforeEach(() => {
    window.localStorage.clear();
    Object.defineProperty(window, "innerHeight", { value: 800, configurable: true });
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("writes the offset only while in input mode and clears it on exit", () => {
    const vv = mockVisualViewport({ height: 500, offsetTop: 0 }); // 800-500+16 = 316
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const layer = container.querySelector(".terminal-fab-layer") as HTMLElement;
    const fab = () =>
      container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;
    const fabClose = () =>
      container.querySelector(`[aria-label="${ARIA_KEYBOARD_CLOSE}"]`) as HTMLElement;

    // view mode: no inline write (subscription is gated on input mode).
    expect(layer.style.getPropertyValue(FAB_OFFSET_PROP)).toBe("");

    act(() => fireEvent.click(fab())); // enter input mode → subscribe + stamp
    expect(getComputedStyle(layer).getPropertyValue(FAB_OFFSET_PROP).trim()).toBe("316px");

    act(() => fireEvent.click(fabClose())); // exit input mode → unsubscribe + clear
    expect(layer.style.getPropertyValue(FAB_OFFSET_PROP)).toBe("");

    vv.restore();
    unmount();
  });
});

describe("TerminalPane — Coachmark shows once (FR-MOB-COACH-001)", () => {
  let mm: MatchMediaHandle;
  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("renders the coachmark on first mount and writes hintSeen, absent on the next mount", () => {
    const conn1 = mobileConn();
    const first = render(<TerminalPane conn={conn1} sessionId="s1" />);
    expect(first.container.querySelector("[data-coachmark]")).not.toBeNull();
    expect(window.localStorage.getItem(HINT_SEEN_KEY)).toBe("1");
    first.unmount();

    // Second session/mount: hintSeen is set → no coachmark.
    const conn2 = mobileConn();
    const second = render(<TerminalPane conn={conn2} sessionId="s2" />);
    expect(second.container.querySelector("[data-coachmark]")).toBeNull();
    second.unmount();
  });
});

describe("TerminalPane — horizontal swipe → arrow wire frames (ADR 0077, FR-MOB-SWIPE-ARROW-*)", () => {
  let mm: MatchMediaHandle;
  let created: Array<{ options: Record<string, unknown> }>;

  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
    created = [];
    // Seed the xterm sub-DOM (mocked open is a no-op) so the gesture hook can
    // bind to .xterm-viewport and the fontSize effect can read the term options.
    vi.spyOn(Terminal.prototype, "open").mockImplementation(function (
      this: Terminal,
      el: HTMLElement,
    ) {
      const vp = document.createElement("div");
      vp.className = "xterm-viewport";
      const ta = document.createElement("textarea");
      ta.className = "xterm-helper-textarea";
      el.appendChild(vp);
      el.appendChild(ta);
      created.push(this as unknown as { options: Record<string, unknown> });
    });
  });
  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  /** Collect every `{k:"i"}` wire frame's `d` field from conn.send.mock.calls. */
  function collectInputBytes(conn: Connection): string[] {
    const send = (conn as unknown as { send: { mock: { calls: unknown[][] } } }).send;
    const out: string[] = [];
    for (const call of send.mock.calls) {
      const frame = call[0] as { k?: string; d?: string };
      if (frame?.k === "i" && typeof frame.d === "string") out.push(frame.d);
    }
    return out;
  }

  function enterInputMode(container: HTMLElement): void {
    const fab = container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;
    act(() => fireEvent.click(fab));
  }

  // VT100 cursor sequences. Stored as constants so the assertions can use
  // `split` / `includes` (biome's noControlCharactersInRegex forbids embedding
  // the ESC byte inside a regex literal, but plain string ops are fine).
  const ESC_RIGHT = "\x1b[C";
  const ESC_LEFT = "\x1b[D";

  function countOccurrences(haystack: string, needle: string): number {
    return haystack.split(needle).length - 1;
  }
  function hasAnyArrow(frames: string[]): boolean {
    return frames.some((d) => d.includes(ESC_RIGHT) || d.includes(ESC_LEFT));
  }

  it("FR-MOB-SWIPE-ARROW-001: input mode + 100px right swipe sends one or more \\x1b[C frames summing to 11 cells", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;
    expect(viewport).not.toBeNull();

    enterInputMode(container);

    act(() => {
      swipeFromTo(viewport, { clientX: 100, clientY: 200 }, { clientX: 200, clientY: 200 }, 100);
    });

    const frames = collectInputBytes(conn);
    const totalRight = frames.reduce((n, d) => n + countOccurrences(d, ESC_RIGHT), 0);
    const totalLeft = frames.reduce((n, d) => n + countOccurrences(d, ESC_LEFT), 0);
    // TerminalMobileOverlay omits cellSize so the hook falls back to DEFAULT_CELL.width=9.
    // 100px ÷ 9 = 11.11 → integer truncation per touchmove sums to 11.
    expect(totalRight).toBe(11);
    expect(totalLeft).toBe(0);
    unmount();
  });

  it("FR-MOB-SWIPE-ARROW-001: input mode + 100px left swipe sends \\x1b[D frames summing to 11 cells", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;

    enterInputMode(container);

    act(() => {
      swipeFromTo(viewport, { clientX: 200, clientY: 200 }, { clientX: 100, clientY: 200 }, 100);
    });

    const frames = collectInputBytes(conn);
    const totalLeft = frames.reduce((n, d) => n + countOccurrences(d, ESC_LEFT), 0);
    const totalRight = frames.reduce((n, d) => n + countOccurrences(d, ESC_RIGHT), 0);
    // DEFAULT_CELL.width=9 fallback: 100px ÷ 9 = 11 (see right-swipe note above).
    expect(totalLeft).toBe(11);
    expect(totalRight).toBe(0);
    unmount();
  });

  it("FR-MOB-SWIPE-ARROW-002: view mode + horizontal swipe sends zero arrow input frames", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host.getAttribute("data-input-active")).toBe("false");

    act(() => {
      swipeFromTo(viewport, { clientX: 100, clientY: 200 }, { clientX: 200, clientY: 200 }, 100);
    });

    const frames = collectInputBytes(conn);
    expect(hasAnyArrow(frames)).toBe(false);
    unmount();
  });

  it("FR-MOB-SWIPE-ARROW-002: vertical swipe in input mode never emits arrow frames (axis lock)", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;

    enterInputMode(container);

    act(() => {
      swipeFromTo(viewport, { clientX: 100, clientY: 400 }, { clientX: 100, clientY: 80 }, 200);
    });

    const frames = collectInputBytes(conn);
    expect(hasAnyArrow(frames)).toBe(false);
    unmount();
  });

  it("FR-MOB-SWIPE-ARROW-003: 2-finger pinch sends no arrow frames and does NOT mutate fontSize", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;
    const term = created[0];
    expect(term.options.fontSize).toBe(14);

    act(() => {
      pinchByRatio(viewport, 1.5, { cx: 100, cy: 100 }, 40);
    });

    expect(term.options.fontSize).toBe(14); // pinch path is gone; stepper is the only writer
    const frames = collectInputBytes(conn);
    expect(hasAnyArrow(frames)).toBe(false);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host.getAttribute("data-input-active")).toBe("false");
    unmount();
  });
});

// ===========================================================================
// Cross-task user-reachable integration scenarios (UAC-002/003/004/005/006/009
// + UAC-014/015 + FR-MOB-JUMP-005). These exercise the full TerminalPane wiring
// under mobile-gate=true with the same xterm DOM seed the pinch suite uses, so
// the chunk-03 / 04 / 06 / 07 hooks bind to a real .xterm-viewport and a real
// .xterm-helper-textarea and the assertions land on the production code path
// (mode + AriaLive + outside-tap + seed-gated jump FAB), not on a paper DOM.
// ===========================================================================

describe("TerminalPane — cross-task mobile UAC integration (gate=true)", () => {
  let mm: MatchMediaHandle;
  let created: Array<{ options: Record<string, unknown>; scrollToBottom?: () => void }>;

  beforeEach(() => {
    window.localStorage.clear();
    mm = mockMatchMedia({ [MOBILE_GATE_QUERY]: true });
    created = [];
    // Same seed strategy as the pinch suite (line 884-933): replace the mocked
    // Terminal.open with one that creates a real .xterm-viewport and real
    // .xterm-helper-textarea inside the host, so useInputMode / useHostPointerInterceptor
    // / useJumpToLatest / useTerminalTouchGestures bind to actual DOM nodes.
    vi.spyOn(Terminal.prototype, "open").mockImplementation(function (
      this: Terminal,
      el: HTMLElement,
    ) {
      const vp = document.createElement("div");
      vp.className = "xterm-viewport";
      const ta = document.createElement("textarea");
      ta.className = "xterm-helper-textarea";
      el.appendChild(vp);
      el.appendChild(ta);
      created.push(this as unknown as { options: Record<string, unknown> });
    });
  });

  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // UAC-005: outside-tap chain (ADR 0068 useHostPointerInterceptor).
  // After entering input mode via the KeyboardFAB, a capture-phase pointerdown
  // on the viewport (descendant of terminal-host, neither helper-textarea nor
  // inside a [data-overlay] FAB layer) must exit input mode and flip the FAB
  // aria-pressed back to false.
  //
  // Counterexample: outside-tap subscribe missing → data-input-active stays 'true'.
  // -------------------------------------------------------------------------
  it("UAC-005: capture-phase pointerdown on viewport exits input mode (FAB aria-pressed flips back)", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;
    const openFab = container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;

    act(() => fireEvent.click(openFab));
    expect(host.getAttribute("data-input-active")).toBe("true");

    // pointerdown on viewport → host capture-phase listener → outside-tap exit.
    act(() => fireEvent.pointerDown(viewport));

    expect(host.getAttribute("data-input-active")).toBe("false");
    const reOpenedFab = container.querySelector(
      `[aria-label="${ARIA_KEYBOARD_OPEN}"]`,
    ) as HTMLElement;
    expect(reOpenedFab).not.toBeNull();
    expect(reOpenedFab.getAttribute("aria-pressed")).toBe("false");
    unmount();
  });

  // -------------------------------------------------------------------------
  // UAC-006: blur -> AriaLive announcement (VIEW_MODE_TEXT, 'returned to view mode').
  // useInputMode's blur subscription flips state→view + reducer produces the
  // VIEW_MODE_ANNOUNCEMENT, which AnnouncerProvider feeds into AriaLiveStatus
  // (data-testid='terminal-aria-live').
  //
  // Counterexample: blur listener unsubscribed → data-input-active stays 'true'
  // (the phantom input-mode bug) and the live region stays empty.
  // -------------------------------------------------------------------------
  it("UAC-006: helper-textarea blur exits input mode and the AriaLive carries the view-mode announcement", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const helper = container.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement;
    const openFab = container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;

    act(() => fireEvent.click(openFab));
    expect(host.getAttribute("data-input-active")).toBe("true");

    act(() => fireEvent.blur(helper));

    expect(host.getAttribute("data-input-active")).toBe("false");
    const live = container.querySelector('[data-testid="terminal-aria-live"]') as HTMLElement;
    expect(live).not.toBeNull();
    expect(live.textContent).toBe(VIEW_MODE_TEXT);
    unmount();
  });

  // -------------------------------------------------------------------------
  // UAC-002 + UAC-009: in view mode, neither tap nor swipe on the viewport
  // focuses the helper textarea (ADR 0068 focus-block via capture-phase
  // pointerdown.preventDefault on the host). We assert both the production
  // contract (pointerdown.defaultPrevented === true on the host listener)
  // and the user-observable effect (focus spy never fires, document.activeElement
  // never lands on the helper).
  //
  // Counterexample: touchend → term.focus() or swipe touchstart → focus path
  // would push focus into the textarea and bump the spy.
  // -------------------------------------------------------------------------
  it("UAC-002 / UAC-009: tap and swipe on viewport in view mode never focus the helper textarea", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const helper = container.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement;
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;
    expect(host.getAttribute("data-input-active")).toBe("false");

    const focusSpy = vi.fn();
    helper.addEventListener("focus", focusSpy);

    // tap (touchstart→touchend) — chunk-01 harness; production never focuses.
    act(() => {
      tapAt(viewport, 100, 100);
    });
    expect(focusSpy).not.toHaveBeenCalled();
    expect(document.activeElement).not.toBe(helper);

    // swipe (touchstart→N touchmove→touchend) — same harness.
    act(() => {
      swipeFromTo(viewport, { clientX: 100, clientY: 100 }, { clientX: 200, clientY: 250 }, 64);
    });
    expect(focusSpy).not.toHaveBeenCalled();
    expect(document.activeElement).not.toBe(helper);

    // The production focus-block invariant: pointerdown on viewport in view mode
    // is preventDefault'd by useHostPointerInterceptor so the synthesized
    // mousedown/focus never reaches the helper textarea (real-device contract).
    const pd = new Event("pointerdown", { bubbles: true, cancelable: true });
    act(() => {
      viewport.dispatchEvent(pd);
    });
    expect(pd.defaultPrevented).toBe(true);
    expect(focusSpy).not.toHaveBeenCalled();
    unmount();
  });

  // -------------------------------------------------------------------------
  // UAC-014 + UAC-015 + FR-MOB-JUMP-005: seed-gate flow. The jump FAB is
  // forced-absent until the ADR-0066 first-output seed lands. Once seedReady
  // is true and the viewport scrolls away from tail, the FAB surfaces; tapping
  // it calls term.scrollToBottom and the FAB disappears when we return to tail.
  //
  // Counterexample: setSeedReady wiring missing → conn.onOutput never flips
  // seedReady=true → FAB never appears no matter how far we scroll.
  // -------------------------------------------------------------------------
  it("UAC-014 / UAC-015 / FR-MOB-JUMP-005: seed-gated jump FAB surfaces only after first onOutput, click scrolls to bottom", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const viewport = container.querySelector(".xterm-viewport") as HTMLElement;

    // The mocked Terminal class has no scrollToBottom; stub it on the live
    // instance so useJumpToLatest.jumpToBottom can call through without
    // exploding. We assert it was called below.
    const scrollToBottomSpy = vi.fn();
    (created[0] as { scrollToBottom?: () => void }).scrollToBottom = scrollToBottomSpy;

    // happy-dom has no layout, so back the three scroll metrics by hand
    // (mirroring useJumpToLatest.test makeViewport).
    let top = 0;
    Object.defineProperty(viewport, "scrollHeight", { value: 1000, configurable: true });
    Object.defineProperty(viewport, "clientHeight", { value: 200, configurable: true });
    Object.defineProperty(viewport, "scrollTop", {
      configurable: true,
      get: () => top,
      set: (v: number) => {
        top = v;
      },
    });

    // Pre-seed: no scroll listener subscribed → no FAB no matter what.
    act(() => fireEvent.scroll(viewport));
    expect(container.querySelector(`[aria-label="${ARIA_JUMP_LATEST}"]`)).toBeNull();

    // First onOutput frame → setSeedReady(true) (TerminalPane.tsx seeded gate).
    act(() => {
      conn.onOutput?.([0, "o", btoa("seed"), "s1"]);
    });

    // Far from tail (tail = 1000-200 = 800; diff 700 > ±2px → FAB visible).
    top = 100;
    act(() => fireEvent.scroll(viewport));
    const jumpFab = container.querySelector(`[aria-label="${ARIA_JUMP_LATEST}"]`) as HTMLElement;
    expect(jumpFab).not.toBeNull();

    // Tap the FAB → scrollToBottom + back-at-tail flow makes the FAB disappear.
    act(() => fireEvent.click(jumpFab));
    expect(scrollToBottomSpy).toHaveBeenCalled();

    top = 800;
    act(() => fireEvent.scroll(viewport));
    expect(container.querySelector(`[aria-label="${ARIA_JUMP_LATEST}"]`)).toBeNull();
    unmount();
  });

  // -------------------------------------------------------------------------
  // UAC-003: KeyboardFAB tap focuses the helper textarea, removes readonly,
  // flips aria-pressed/aria-label, and the state persists past 200ms.
  // The 200ms persistence rules out the enter→immediate-exit race
  // counterexample (a stray blur/effect-order bug flipping state inside 200ms).
  // -------------------------------------------------------------------------
  it("UAC-003: KeyboardFAB tap focuses the helper textarea and the input state persists past 200ms", () => {
    vi.useFakeTimers();
    try {
      const conn = mobileConn();
      const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
      const host = container.querySelector(".terminal-host") as HTMLElement;
      const helper = container.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement;
      const openFab = container.querySelector(
        `[aria-label="${ARIA_KEYBOARD_OPEN}"]`,
      ) as HTMLElement;

      act(() => fireEvent.click(openFab));

      expect(host.getAttribute("data-input-active")).toBe("true");
      expect(helper.hasAttribute("readonly")).toBe(false);
      expect(document.activeElement).toBe(helper);
      const closeFab = container.querySelector(
        `[aria-label="${ARIA_KEYBOARD_CLOSE}"]`,
      ) as HTMLElement;
      expect(closeFab).not.toBeNull();
      expect(closeFab.getAttribute("aria-pressed")).toBe("true");

      // 200ms must not trigger any phantom exit (counterexample: enter→exit race).
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(host.getAttribute("data-input-active")).toBe("true");
      expect(document.activeElement).toBe(helper);
      unmount();
    } finally {
      vi.useRealTimers();
    }
  });

  // -------------------------------------------------------------------------
  // UAC-004: toggle return. Tapping the KeyboardFAB a second time exits input
  // mode silently (no announce), re-adds readonly to the helper, releases focus,
  // and flips the FAB label/pressed back to the open variant.
  //
  // Counterexample: a single-direction trigger (no toggle) → after two taps the
  // host remains 'true'.
  // -------------------------------------------------------------------------
  it("UAC-004: two KeyboardFAB taps round-trip the mode (readonly restored, helper not focused)", () => {
    const conn = mobileConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const helper = container.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement;

    act(() =>
      fireEvent.click(
        container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement,
      ),
    );
    expect(host.getAttribute("data-input-active")).toBe("true");
    expect(document.activeElement).toBe(helper);

    act(() =>
      fireEvent.click(
        container.querySelector(`[aria-label="${ARIA_KEYBOARD_CLOSE}"]`) as HTMLElement,
      ),
    );
    expect(host.getAttribute("data-input-active")).toBe("false");
    const reOpened = container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`) as HTMLElement;
    expect(reOpened).not.toBeNull();
    expect(reOpened.getAttribute("aria-pressed")).toBe("false");
    expect(helper.hasAttribute("readonly")).toBe(true);
    expect(document.activeElement).not.toBe(helper);
    unmount();
  });
});

// ===========================================================================
// [terminal] font_family / font_size (settings.toml → GET /api/session-config
// → daemon store → xterm grid). Empty / zero must leave the xterm.js default
// untouched; configured values apply both at construction and reactively when
// the REST fetch lands after the terminal already mounted.
// ===========================================================================

describe("[terminal] font_family / font_size apply to the xterm grid", () => {
  let createdTerminals: Terminal[];
  let openSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    createdTerminals = [];
    openSpy = vi.spyOn(Terminal.prototype, "open").mockImplementation(function (this: Terminal) {
      createdTerminals.push(this);
    });
    useDaemonStore.setState({ sessionConfig: null });
  });

  afterEach(() => {
    openSpy.mockRestore();
    useDaemonStore.setState({ sessionConfig: null });
  });

  function firstOptions(): Record<string, unknown> {
    expect(createdTerminals.length).toBeGreaterThan(0);
    return (createdTerminals[0] as unknown as { options: Record<string, unknown> }).options;
  }

  it("passes the configured font to new Terminal at construction", () => {
    act(() => {
      useDaemonStore.getState().setSessionConfig(fontSessionConfig("HackGen Console NF", 16));
    });
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    const opts = firstOptions();
    expect(opts.fontFamily).toBe("HackGen Console NF");
    expect(opts.fontSize).toBe(16);
    unmount();
  });

  it("leaves the xterm.js default font when config is unset (empty / 0)", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    const opts = firstOptions();
    // We never forward an empty fontFamily; the grid keeps xterm.js's built-in
    // monospace default rather than a blanked font.
    expect(opts.fontFamily).not.toBe("");
    unmount();
  });

  it("applies the font to a live terminal when session-config lands after mount", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const opts = firstOptions();

    act(() => {
      useDaemonStore.getState().setSessionConfig(fontSessionConfig("Iosevka", 18));
    });

    expect(opts.fontFamily).toBe("Iosevka");
    expect(opts.fontSize).toBe(18);
    unmount();
  });
});
