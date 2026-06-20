import { describe, expect, it } from "vitest";
import { CAP_MS, INITIAL_MS, MAX_ATTEMPTS, backoffDelay, exceededAttempts } from "./backoff";

describe("backoff", () => {
  it("attempt 0 returns 0..INITIAL_MS with seeded rng", () => {
    const d0 = backoffDelay(0, () => 0);
    const d1 = backoffDelay(0, () => 0.999);
    expect(d0).toBe(0);
    expect(d1).toBeLessThanOrEqual(INITIAL_MS);
  });

  it("grows exponentially capped at CAP_MS", () => {
    const upper = backoffDelay(100, () => 0.999);
    expect(upper).toBeLessThanOrEqual(CAP_MS);
  });

  it("exceededAttempts(MAX_ATTEMPTS) is true", () => {
    expect(exceededAttempts(MAX_ATTEMPTS)).toBe(true);
    expect(exceededAttempts(MAX_ATTEMPTS - 1)).toBe(false);
  });

  it("full jitter sequence with seed 0.5: values stay within bounds", () => {
    // seed fixed at 0.5 — deterministic sequence
    const rng = () => 0.5;
    const results: number[] = [];
    for (let i = 0; i < MAX_ATTEMPTS; i++) {
      results.push(backoffDelay(i, rng));
    }
    // all values within [0, CAP_MS]
    for (const d of results) {
      expect(d).toBeGreaterThanOrEqual(0);
      expect(d).toBeLessThanOrEqual(CAP_MS);
    }
    // sequence is non-decreasing in expectation (with fixed rng=0.5, each step doubles until cap)
    // attempt 0: floor(0.5 * 250) = 125
    // attempt 1: floor(0.5 * 500) = 250
    // attempt 2: floor(0.5 * 1000) = 500
    // attempt 3: floor(0.5 * 2000) = 1000
    // attempt 4: floor(0.5 * 4000) = 2000
    // attempt 5+: floor(0.5 * 4000) = 2000 (capped)
    expect(results[0]).toBe(125);
    expect(results[1]).toBe(250);
    expect(results[2]).toBe(500);
    expect(results[3]).toBe(1000);
    expect(results[4]).toBe(2000);
    // capped at 2000 with rng=0.5 (half of CAP_MS=4000)
    for (let i = 5; i < MAX_ATTEMPTS; i++) {
      expect(results[i]).toBe(2000);
    }
  });

  it("negative attempt returns 0", () => {
    expect(backoffDelay(-1)).toBe(0);
  });
});
