import { afterEach, describe, expect, it, vi } from "vitest";
import { isMacPlatform } from "./platform";

// Each case stubs the entire `navigator` global so the four branches in
// `isMacPlatform()` can be exercised independently. We restore stubs after
// every test to avoid leaking into adjacent suites (the real happy-dom
// `navigator` is shared across tests).

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("isMacPlatform", () => {
  it("returns true when navigator.userAgentData.platform contains 'macOS' (UA-CH branch)", () => {
    vi.stubGlobal("navigator", {
      userAgentData: { platform: "macOS" },
      platform: "",
      userAgent: "",
    });
    expect(isMacPlatform()).toBe(true);
  });

  it("returns true when navigator.platform contains 'MAC' and userAgentData is absent", () => {
    vi.stubGlobal("navigator", {
      platform: "MacIntel",
      userAgent: "",
    });
    expect(isMacPlatform()).toBe(true);
  });

  it("returns true when only navigator.userAgent contains 'Macintosh' (last-resort branch)", () => {
    vi.stubGlobal("navigator", {
      platform: "",
      userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    });
    expect(isMacPlatform()).toBe(true);
  });

  it("returns false on non-Mac platforms (no signal matches)", () => {
    vi.stubGlobal("navigator", {
      platform: "",
      userAgent: "Linux x86_64",
    });
    expect(isMacPlatform()).toBe(false);
  });

  it("returns false when navigator is undefined (SSR / test safety, FR-D2)", () => {
    vi.stubGlobal("navigator", undefined);
    expect(isMacPlatform()).toBe(false);
  });
});
