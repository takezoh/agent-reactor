import { vi } from "vitest";

// xterm.js relies on canvas/DOM features happy-dom does not implement.
// Mock the module so TerminalPane.test can render without exploding.
vi.mock("@xterm/xterm", () => {
  class FakeTerminal {
    /** Mutable options bag — tests can read options.theme to verify FR-THEME-002.
     *  Initialised from the constructor argument so that new Terminal({ theme })
     *  is reflected immediately (mirrors xterm.js public API contract). */
    options: Record<string, unknown>;

    constructor(opts?: Record<string, unknown>) {
      this.options = opts ? { ...opts } : {};
    }

    onData(_cb: (d: string) => void) {
      return { dispose() {} };
    }
    onResize(_cb: (s: { cols: number; rows: number }) => void) {
      return { dispose() {} };
    }
    attachCustomKeyEventHandler(_cb: (e: KeyboardEvent) => boolean) {}
    open(_el: HTMLElement) {}
    loadAddon(_a: unknown) {}
    write(_d: string) {}
    dispose() {}
  }
  return { Terminal: FakeTerminal };
});
vi.mock("@xterm/addon-fit", () => {
  class FakeFitAddon {
    fit() {}
  }
  return { FitAddon: FakeFitAddon };
});
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// ---------------------------------------------------------------------------
// ResizeObserver mock (ADR 0034)
// happy-dom does not implement ResizeObserver. We provide a minimal mock that
// lets tests manually trigger resize callbacks via globalThis.__triggerResize.
// ---------------------------------------------------------------------------
type ResizeCallback = (entries: ResizeObserverEntry[]) => void;

const _roInstances = new Map<Element, ResizeCallback>();

class MockResizeObserver {
  private _cb: ResizeCallback;
  private _target: Element | null = null;

  constructor(cb: ResizeCallback) {
    this._cb = cb;
  }

  observe(target: Element) {
    this._target = target;
    _roInstances.set(target, this._cb);
  }

  disconnect() {
    if (this._target) {
      _roInstances.delete(this._target);
      this._target = null;
    }
  }
}

// triggerResizeImpl(target, entries) — manually fire the observer callback
// registered for `target`. Exposed on globalThis as `__triggerResize` so test
// files can call it without per-call `as unknown as { ... }` casts. The
// observer mock does not actually inspect entries, so the type is intentionally
// loose to let tests pass partial entry shapes for documentation purposes.
function triggerResizeImpl(target: Element, entries: unknown[] = []) {
  const cb = _roInstances.get(target);
  if (cb) cb(entries as ResizeObserverEntry[]);
}

// Augment globalThis so tests can call `globalThis.__triggerResize(...)` typed.
declare global {
  // eslint-disable-next-line no-var
  var __triggerResize: (target: Element, entries?: unknown[]) => void;
}

globalThis.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver;
globalThis.__triggerResize = triggerResizeImpl;

// ---------------------------------------------------------------------------
// requestAnimationFrame synchronous mock (ADR 0034)
// happy-dom's rAF does not flush synchronously. We replace it with an
// implementation that runs callbacks immediately (synchronous flush) so that
// scheduleFit tests can assert fit.fit() is called after a single rAF tick.
// ---------------------------------------------------------------------------
let _rafIdCounter = 0;

globalThis.requestAnimationFrame = (cb: FrameRequestCallback): number => {
  const id = ++_rafIdCounter;
  cb(performance.now());
  return id;
};

globalThis.cancelAnimationFrame = (_id: number) => {
  // no-op: callbacks have already fired synchronously
};

// ---------------------------------------------------------------------------
// matchMedia mock (for prefers-color-scheme / prefers-reduced-motion tests)
// happy-dom does not implement window.matchMedia. We provide a minimal stub
// backed by an internal Map so tests can override values via setMatchMedia().
// ---------------------------------------------------------------------------

/** Default values for media queries used by the app. */
const _mqDefaults: Record<string, boolean> = {
  "(prefers-color-scheme: dark)": true,
  "(prefers-reduced-motion: reduce)": false,
};

type MQLListener = (event: MediaQueryListEvent) => void;

interface MQLStub {
  matches: boolean;
  media: string;
  listeners: Set<MQLListener>;
}

/** Internal store: query string → stub state */
const _mqStore = new Map<string, MQLStub>();

function _getOrCreateStub(query: string): MQLStub {
  let stub = _mqStore.get(query);
  if (stub === undefined) {
    const defaultMatches = _mqDefaults[query] ?? false;
    stub = { matches: defaultMatches, media: query, listeners: new Set() };
    _mqStore.set(query, stub);
  }
  return stub;
}

function _makeMediaQueryList(query: string): MediaQueryList {
  // Ensure the stub exists in the store for this query.
  _getOrCreateStub(query);

  const mql: MediaQueryList = {
    get matches() {
      return _getOrCreateStub(query).matches;
    },
    get media() {
      return query;
    },
    onchange: null,
    addEventListener(_type: string, listener: EventListenerOrEventListenerObject) {
      if (typeof listener === "function") {
        _getOrCreateStub(query).listeners.add(listener as MQLListener);
      }
    },
    removeEventListener(_type: string, listener: EventListenerOrEventListenerObject) {
      if (typeof listener === "function") {
        _getOrCreateStub(query).listeners.delete(listener as MQLListener);
      }
    },
    dispatchEvent(_event: Event): boolean {
      return true;
    },
    /** @deprecated */
    addListener(listener: ((this: MediaQueryList, ev: MediaQueryListEvent) => unknown) | null) {
      if (listener) {
        _getOrCreateStub(query).listeners.add(listener as unknown as MQLListener);
      }
    },
    /** @deprecated */
    removeListener(listener: ((this: MediaQueryList, ev: MediaQueryListEvent) => unknown) | null) {
      if (listener) {
        _getOrCreateStub(query).listeners.delete(listener as unknown as MQLListener);
      }
    },
  };

  return mql;
}

globalThis.matchMedia = _makeMediaQueryList as unknown as typeof window.matchMedia;

/** Update a media query's matches value and fire change listeners. */
function setMatchMediaImpl(query: string, matches: boolean): void {
  const stub = _getOrCreateStub(query);
  stub.matches = matches;

  const event = {
    matches,
    media: query,
    type: "change",
    bubbles: false,
    cancelable: false,
    isTrusted: true,
  } as unknown as MediaQueryListEvent;

  for (const listener of stub.listeners) {
    listener(event);
  }
}

/** Reset all stubs to defaults (called in afterEach). */
function _resetMqStore(): void {
  _mqStore.clear();
}

// Augment globalThis so tests can call `globalThis.setMatchMedia(...)` typed.
declare global {
  // eslint-disable-next-line no-var
  var setMatchMedia: (query: string, matches: boolean) => void;
}

globalThis.setMatchMedia = setMatchMediaImpl;

// Reset the matchMedia store to defaults after each test.
import { afterEach } from "vitest";
afterEach(() => {
  _resetMqStore();
});

// ---------------------------------------------------------------------------
// Default xterm CSS tokens (FR-THEME-003 / ADR-0059)
// Set xterm custom properties as inline styles on :root so that tests which
// mount TerminalPane (or use useXtermTheme() indirectly) do not trigger the
// "[ThemeProvider] CSS token missing or empty" console.warn on every run.
// Using inline styles (not a <style> tag) allows individual tests that need
// to exercise the missing-tokens path (e.g. ThemeProvider fallback test) to
// remove them via document.documentElement.style.removeProperty() and have
// getComputedStyle() return empty — as it would without any declaration.
// beforeEach re-applies the defaults so the next test starts clean.
// ---------------------------------------------------------------------------

/** Xterm token defaults that mirror tokens.css dark-mode values. */
const _xtermTokenDefaults: ReadonlyArray<[string, string]> = [
  ["--xterm-fg", "#e6e6e6"],
  ["--xterm-cursor", "#e6e6e6"],
  ["--xterm-selection", "rgba(74, 158, 255, 0.3)"],
];

function _applyXtermTokenDefaults(): void {
  for (const [token, value] of _xtermTokenDefaults) {
    // Only set if not already explicitly set by the current test's beforeEach.
    // Use setProperty unconditionally — a test that needs different values
    // will override these in its own beforeEach which runs after this one.
    document.documentElement.style.setProperty(token, value);
  }
}

function _removeXtermTokenDefaults(): void {
  for (const [token] of _xtermTokenDefaults) {
    document.documentElement.style.removeProperty(token);
  }
}

// Apply defaults before each test so useXtermTheme() sees valid tokens by
// default. Tests that need the missing-token path call removeProperty() in
// their own setup, overriding these inline values.
import { beforeEach as _beforeEach } from "vitest";
_beforeEach(() => {
  _applyXtermTokenDefaults();
});

afterEach(() => {
  _removeXtermTokenDefaults();
});

// ---------------------------------------------------------------------------
// MutationObserver intercept for data-theme flush (ThemeProvider tests)
// happy-dom's MutationObserver does not fire on documentElement attribute
// mutations. We wrap MutationObserver so tests can call
// globalThis.flushThemeObservers() to manually trigger any observer that
// watches documentElement[data-theme], simulating the production path.
// ---------------------------------------------------------------------------

type MutationCallback = (mutations: MutationRecord[], observer: MutationObserver) => void;

interface ThemeObserverEntry {
  callback: MutationCallback;
  options: MutationObserverInit;
  target: Node;
  self: MutationObserver;
}

const _themeObservers: Set<ThemeObserverEntry> = new Set();

const _OriginalMutationObserver = globalThis.MutationObserver;

class InterceptedMutationObserver implements MutationObserver {
  private _callback: MutationCallback;
  private _entry: ThemeObserverEntry | null = null;

  constructor(callback: MutationCallback) {
    this._callback = callback;
  }

  observe(target: Node, options?: MutationObserverInit): void {
    const entry: ThemeObserverEntry = {
      callback: this._callback,
      options: options ?? {},
      target,
      self: this as unknown as MutationObserver,
    };
    this._entry = entry;
    // Only track observers on documentElement that watch data-theme.
    if (target === document.documentElement && options?.attributeFilter?.includes("data-theme")) {
      _themeObservers.add(entry);
    }
  }

  disconnect(): void {
    if (this._entry !== null) {
      _themeObservers.delete(this._entry);
      this._entry = null;
    }
  }

  takeRecords(): MutationRecord[] {
    return [];
  }
}

globalThis.MutationObserver = InterceptedMutationObserver as unknown as typeof MutationObserver;

/**
 * Manually flush all MutationObserver callbacks registered for
 * documentElement[data-theme]. Call this in tests after mutating
 * document.documentElement.dataset.theme to simulate the production
 * browser path where MutationObserver fires automatically.
 */
function flushThemeObserversImpl(): void {
  const currentTheme = document.documentElement.dataset.theme ?? "";
  const record: MutationRecord = {
    type: "attributes",
    attributeName: "data-theme",
    attributeNamespace: null,
    oldValue: null,
    target: document.documentElement,
    addedNodes: [] as unknown as NodeList,
    removedNodes: [] as unknown as NodeList,
    previousSibling: null,
    nextSibling: null,
  } as unknown as MutationRecord;

  for (const entry of _themeObservers) {
    if (entry.target === document.documentElement) {
      entry.callback([record], entry.self);
    }
  }

  // Suppress unused-variable warning in the closure above.
  void currentTheme;
}

// Augment globalThis so tests can call `globalThis.flushThemeObservers()` typed.
declare global {
  // eslint-disable-next-line no-var
  var flushThemeObservers: () => void;
}

globalThis.flushThemeObservers = flushThemeObserversImpl;

// Clear theme observers after each test to prevent cross-test leakage.
afterEach(() => {
  _themeObservers.clear();
});

// Restore original MutationObserver if needed (noop — the interceptor is
// compatible and does not break non-theme uses).
void _OriginalMutationObserver;
