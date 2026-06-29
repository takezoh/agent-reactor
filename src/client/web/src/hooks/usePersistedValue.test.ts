// usePersistedValue.test.ts — ADR 0070, FR-MOB-PERSIST-001/002.
//
// Discriminating coverage of the UAC-019 counterexample ("'999' is streamed
// straight to term.options.fontSize without clamp"): the adapter must SPLIT
// "parse failed → fallback" from "parsed-but-out-of-range → clamp". A naive
// "anything invalid → fallback" implementation would turn '999' into 14 and
// pass this file only if we did not assert 28 specifically — so we do.
//
// Storage is injected as an in-memory Map adapter (DI) so the four parse/clamp
// cases and the private-mode throw-degrade path are deterministic and do not
// depend on happy-dom's localStorage.

import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { type StorageLike, usePersistedValue } from "./usePersistedValue";

/** In-memory StorageLike backed by a Map (the DI seam from ADR 0070). */
function mapStorage(initial?: Record<string, string>): StorageLike & { map: Map<string, string> } {
  const map = new Map<string, string>(initial ? Object.entries(initial) : []);
  return {
    map,
    getItem: (k) => (map.has(k) ? (map.get(k) as string) : null),
    setItem: (k, v) => {
      map.set(k, v);
    },
  };
}

const KEY = "web.term.fontSize";

/** The fontSize-shaped config exercised throughout (parseInt + finite + clamp[8,28]). */
function fontSizeConfig(storage: StorageLike | null) {
  return {
    key: KEY,
    parse: (raw: string) => Number.parseInt(raw, 10),
    serialize: (n: number) => String(n),
    fallback: 14,
    validate: (n: number) => (Number.isFinite(n) ? Math.min(28, Math.max(8, n)) : null),
    storage,
  };
}

describe("usePersistedValue — read parse/validate/clamp (ADR 0070)", () => {
  it("UAC-019: '999' parses successfully and CLAMPS to 28 (not fallback 14)", () => {
    const storage = mapStorage({ [KEY]: "999" });
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(28);
  });

  it("'' fails to parse (NaN) → fallback 14", () => {
    const storage = mapStorage({ [KEY]: "" });
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(14);
  });

  it("'foo' fails to parse (NaN) → fallback 14", () => {
    const storage = mapStorage({ [KEY]: "foo" });
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(14);
  });

  it("absent key (null) → fallback 14", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(14);
  });

  it("in-range stored value '20' is accepted as-is", () => {
    const storage = mapStorage({ [KEY]: "20" });
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(20);
  });

  it("below-range '3' clamps up to 8 (lower-bound clamp, UAC-017 adapter half)", () => {
    const storage = mapStorage({ [KEY]: "3" });
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));
    expect(result.current[0]).toBe(8);
  });
});

describe("usePersistedValue — write + persistence", () => {
  it("set updates memory state and writes the serialized value to storage", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(storage)));

    act(() => result.current[1](20));

    expect(result.current[0]).toBe(20);
    expect(storage.map.get(KEY)).toBe("20");
  });
});

describe("usePersistedValue — private-mode degrade (try/catch)", () => {
  it("setItem throwing still updates memory state (no crash)", () => {
    const throwing: StorageLike = {
      getItem: () => null,
      setItem: () => {
        throw new Error("QuotaExceededError (private mode)");
      },
    };
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(throwing)));

    expect(() => act(() => result.current[1](22))).not.toThrow();
    // Memory state degrades-only: the value is applied even though the write failed.
    expect(result.current[0]).toBe(22);
  });

  it("getItem throwing on read degrades to fallback (no crash on mount)", () => {
    const throwing: StorageLike = {
      getItem: () => {
        throw new Error("SecurityError (private mode)");
      },
      setItem: () => {},
    };
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const { result } = renderHook(() => usePersistedValue(fontSizeConfig(throwing)));
    expect(result.current[0]).toBe(14);
    spy.mockRestore();
  });
});
