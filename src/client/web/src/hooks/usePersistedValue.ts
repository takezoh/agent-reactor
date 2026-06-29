// usePersistedValue — the single localStorage-persistence adapter (ADR 0070).
//
// Every device-scoped preference (`useFontSize`, and chunk-07's `useCoachmarkOnce`)
// reads/writes through this one hook so the four cross-cutting concerns live in
// exactly one place and never drift between call sites:
//
//   1. try/catch around storage access — private mode throws on getItem/setItem
//      must degrade to in-memory state, never crash the app (FR-MOB-PERSIST-001).
//   2. parse — turn the raw string into T (e.g. parseInt). May fail (NaN).
//   3. validate — distinguish "parse failed → fallback" from "parsed but out of
//      range → clamp". This split is load-bearing for UAC-019: '999' parses fine
//      and must clamp to 28, whereas '' / 'foo' / null fail to parse and fall back
//      to the default. Collapsing both into "anything invalid → fallback" silently
//      passes the UAC-019 counterexample (ADR 0070 rejected alternative).
//   4. serialize — write the value back as a string.
//
// The `storage` adapter is injected (DI). Production passes window.localStorage;
// tests pass an in-memory Map adapter so coverage is deterministic and does not
// depend on happy-dom's localStorage implementation.

import { useCallback, useRef, useState } from "react";

/** Minimal storage surface this adapter touches — satisfied by window.localStorage. */
export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export interface PersistedValueConfig<T> {
  /** localStorage key, e.g. `web.term.fontSize`. */
  key: string;
  /** Raw string → T. May produce an invalid value (e.g. NaN) — `validate` filters it. */
  parse: (raw: string) => T;
  /** T → raw string for writing. */
  serialize: (value: T) => string;
  /** Value used when the key is absent, unreadable, or fails validation. */
  fallback: T;
  /**
   * Normalise a *successfully parsed* value: return the (possibly clamped) value
   * to accept it, or `null` to reject it (→ fallback). For fontSize this is
   * `Number.isFinite(n) ? clamp(n, 8, 28) : null`, which both rejects NaN and
   * clamps '999' to 28 (UAC-019).
   */
  validate: (value: T) => T | null;
  /** Injected storage; defaults to window.localStorage when available. */
  storage?: StorageLike | null;
}

/** Resolve the default storage without throwing in SSR / locked-down contexts. */
function defaultStorage(): StorageLike | null {
  try {
    return typeof window !== "undefined" && window.localStorage ? window.localStorage : null;
  } catch {
    // Some browsers throw on the `localStorage` getter itself in private mode.
    return null;
  }
}

/**
 * usePersistedValue reads the persisted value once on mount (through parse +
 * validate + clamp), and returns `[value, set]`. `set` updates the in-memory
 * state *and* attempts to persist; a persistence failure is swallowed so the UI
 * only degrades (no crash), exactly matching the private-mode contract.
 */
export function usePersistedValue<T>(
  config: PersistedValueConfig<T>,
): readonly [T, (next: T) => void] {
  const { key, parse, serialize, fallback, validate } = config;

  // Resolve storage once; a passed adapter (test Map) wins over window.localStorage.
  const storageRef = useRef<StorageLike | null>(
    config.storage !== undefined ? config.storage : defaultStorage(),
  );

  // Read once, lazily, on mount: parse + validate/clamp, degrading to fallback on
  // a private-mode throw or an absent/invalid value (ADR 0070).
  const [value, setValue] = useState<T>(() => {
    const storage = storageRef.current;
    if (!storage) return fallback;
    let raw: string | null;
    try {
      raw = storage.getItem(key);
    } catch {
      // Read threw (private mode) → degrade to default.
      return fallback;
    }
    if (raw === null) return fallback;
    const validated = validate(parse(raw));
    return validated === null ? fallback : validated;
  });

  const set = useCallback(
    (next: T): void => {
      // Memory state always updates first so the UX never blocks on storage.
      setValue(next);
      const storage = storageRef.current;
      if (!storage) return;
      try {
        storage.setItem(key, serialize(next));
      } catch {
        // Write threw (private mode / quota) → swallow; memory state already set.
      }
    },
    [key, serialize],
  );

  return [value, set] as const;
}
