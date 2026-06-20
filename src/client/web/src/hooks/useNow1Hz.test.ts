import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatElapsed, useNow1Hz } from "./useNow1Hz";

describe("useNow1Hz", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns initial Date.now() then ticks every second", () => {
    const { result } = renderHook(() => useNow1Hz());
    const t0 = result.current;
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current).toBeGreaterThanOrEqual(t0 + 1000);
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(result.current).toBeGreaterThanOrEqual(t0 + 4000);
  });

  it("clears interval on unmount", () => {
    const { unmount } = renderHook(() => useNow1Hz());
    expect(vi.getTimerCount()).toBeGreaterThan(0);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("formatElapsed", () => {
  it.each([
    [0, "0s"],
    [999, "0s"],
    [1000, "1s"],
    [59_000, "59s"],
    [60_000, "1m 0s"],
    [125_000, "2m 5s"],
    [3_660_000, "1h 1m"],
    [-1, ""],
    [Number.NaN, ""],
  ])("formats %d ms as %s", (ms, want) => {
    expect(formatElapsed(ms)).toBe(want);
  });
});
