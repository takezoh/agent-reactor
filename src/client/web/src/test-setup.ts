import { vi } from "vitest";

// xterm.js relies on canvas/DOM features happy-dom does not implement.
// Mock the module so TerminalPane.test can render without exploding.
vi.mock("@xterm/xterm", () => {
  class FakeTerminal {
    onData(_cb: (d: string) => void) {
      return { dispose() {} };
    }
    onResize(_cb: (s: { cols: number; rows: number }) => void) {
      return { dispose() {} };
    }
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
