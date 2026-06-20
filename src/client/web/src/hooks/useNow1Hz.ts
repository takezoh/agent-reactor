import { useEffect, useState } from "react";

/**
 * useNow1Hz — returns the current Date.now() value, refreshed at 1 Hz.
 *
 * Design (ADR 0024 — "The 1 Hz ticker is a single timer, not N timers"):
 *   "One global ticker shared by all StatusLine displays — not per-component
 *   intervals."
 *
 * Implementation: a module-scope subscriber set drives a single setInterval.
 *   - First subscriber starts the interval.
 *   - Each subscriber receives a setState setter; the tick fires all setters.
 *   - Last subscriber's unmount clears the interval.
 *   - A new subscription after full teardown restarts it.
 *
 * Clock note: values are sourced from Date.now() on each tick; StatusChangedAt
 * from the daemon vs Date.now() may drift slightly, but that is accepted for α
 * (ADR 0024 open question 3).
 *
 * Used by DriverViewPanel to render StatusLine elapsed time without requiring
 * the daemon to push at 1 Hz.
 */

type Setter = (now: number) => void;

// Module-scope shared ticker state.
const subscribers = new Set<Setter>();
let timerId: ReturnType<typeof setInterval> | null = null;

function tick(): void {
  const now = Date.now();
  subscribers.forEach((set) => set(now));
}

function subscribe(setter: Setter): void {
  subscribers.add(setter);
  if (timerId === null) {
    timerId = setInterval(tick, 1000);
  }
}

function unsubscribe(setter: Setter): void {
  subscribers.delete(setter);
  if (subscribers.size === 0 && timerId !== null) {
    clearInterval(timerId);
    timerId = null;
  }
}

export function useNow1Hz(): number {
  const [now, setNow] = useState<number>(() => Date.now());

  useEffect(() => {
    subscribe(setNow);
    return () => unsubscribe(setNow);
  }, []);

  return now;
}

/**
 * Formats elapsed time (milliseconds) as "Ns" / "Nm Ns" / "Nh Nm" — the same
 * style the TUI sidebar uses for status_changed_at deltas. Negative or NaN
 * input returns an empty string.
 */
export function formatElapsed(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "";
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ${sec % 60}s`;
  const hr = Math.floor(min / 60);
  return `${hr}h ${min % 60}m`;
}
