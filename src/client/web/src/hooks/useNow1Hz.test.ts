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
    const { result, unmount } = renderHook(() => useNow1Hz());
    const t0 = result.current;
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current).toBeGreaterThanOrEqual(t0 + 1000);
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(result.current).toBeGreaterThanOrEqual(t0 + 4000);
    unmount();
  });

  it("clears the shared interval only when the last subscriber unmounts", () => {
    const { unmount: unmountA } = renderHook(() => useNow1Hz());
    const { unmount: unmountB } = renderHook(() => useNow1Hz());

    // Two hooks share a single timer.
    expect(vi.getTimerCount()).toBe(1);

    // Unmounting one hook leaves the timer running for the remaining subscriber.
    act(() => { unmountA(); });
    expect(vi.getTimerCount()).toBe(1);

    // Unmounting the last subscriber clears the interval.
    act(() => { unmountB(); });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("both hooks receive ticks from the single shared timer", () => {
    const { result: resultA, unmount: unmountA } = renderHook(() => useNow1Hz());
    const { result: resultB, unmount: unmountB } = renderHook(() => useNow1Hz());
    const t0 = resultA.current;

    act(() => { vi.advanceTimersByTime(2000); });

    expect(resultA.current).toBeGreaterThanOrEqual(t0 + 2000);
    expect(resultB.current).toBeGreaterThanOrEqual(t0 + 2000);

    unmountA();
    unmountB();
  });

  it("surviving subscriber still ticks after the other unmounts", () => {
    const { unmount: unmountA } = renderHook(() => useNow1Hz());
    const { result: resultB, unmount: unmountB } = renderHook(() => useNow1Hz());
    const t0 = resultB.current;

    act(() => { unmountA(); });

    // After A is gone the single timer still drives B.
    act(() => { vi.advanceTimersByTime(1000); });
    expect(resultB.current).toBeGreaterThanOrEqual(t0 + 1000);

    unmountB();
  });

  it("restarts a new single timer after all subscribers have left", () => {
    const { unmount: unmountFirst } = renderHook(() => useNow1Hz());
    act(() => { unmountFirst(); });
    expect(vi.getTimerCount()).toBe(0);

    // A fresh subscription should restart exactly one timer.
    const { unmount: unmountSecond } = renderHook(() => useNow1Hz());
    expect(vi.getTimerCount()).toBe(1);
    act(() => { unmountSecond(); });
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
