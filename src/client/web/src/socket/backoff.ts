// Exponential backoff with full jitter. Shared by reconnect (connection.ts) and
// subscribe retry (retry.ts) per ADR 0022.

export const INITIAL_MS = 250;
export const CAP_MS = 4000;
export const MAX_ATTEMPTS = 16;

export type Rng = () => number; // 0..1, defaults to Math.random

export function backoffDelay(attempt: number, rng: Rng = Math.random): number {
  if (attempt < 0) return 0;
  const exp = Math.min(CAP_MS, INITIAL_MS * 2 ** attempt);
  // full jitter: random in [0, exp]
  return Math.floor(rng() * exp);
}

export function exceededAttempts(attempt: number): boolean {
  return attempt >= MAX_ATTEMPTS;
}
