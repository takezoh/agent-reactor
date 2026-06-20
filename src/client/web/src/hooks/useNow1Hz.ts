import { useEffect, useState } from "react";

/**
 * Returns the current Date.now() value, updated once per second.
 * Used by DriverViewPanel to render StatusLine elapsed time without
 * requiring the daemon to push at 1Hz (ADR 0024).
 *
 * The interval is cleared on unmount.
 */
export function useNow1Hz(): number {
  const [now, setNow] = useState<number>(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
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
