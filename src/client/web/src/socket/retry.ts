import { serializeClientFrame } from "../wire/codec";
import type { RespErrFrame, RespOKFrame, ServerFrame } from "../wire/server";
import { type Rng, backoffDelay, exceededAttempts } from "./backoff";

export type SubscribeOutcome =
  | { status: "confirmed"; reqId: string }
  | { status: "exhausted"; lastError: string };

// Caller-provided side effects.
export type RetryDeps = {
  send: (frame: string) => void;
  // Awaits the next Resp frame matching reqId. Implementations: see connection.ts
  // which routes incoming frames to a per-reqId promise registry.
  awaitResponse: (reqId: string) => Promise<RespOKFrame | RespErrFrame>;
  newReqId: () => string;
  // sleep wrapper, injectable for tests
  sleep: (ms: number) => Promise<void>;
  rng?: Rng;
};

export async function subscribeWithRetry(
  sessionId: string,
  deps: RetryDeps,
): Promise<SubscribeOutcome> {
  let attempt = 0;
  let lastError = "";
  while (!exceededAttempts(attempt)) {
    const reqId = deps.newReqId();
    const frame = serializeClientFrame({ k: "s", reqId, sessionId });
    deps.send(frame);
    const resp = await deps.awaitResponse(reqId);
    if (resp.k === "r") {
      return { status: "confirmed", reqId };
    }
    // resp.k === "e"
    if (resp.code !== "frame-not-ready") {
      // non-retryable
      return { status: "exhausted", lastError: resp.code };
    }
    lastError = resp.code;
    attempt += 1;
    if (exceededAttempts(attempt)) break;
    await deps.sleep(backoffDelay(attempt, deps.rng));
  }
  return { status: "exhausted", lastError };
}

// helper for tests: classify a ServerFrame as Resp matching a reqId
export function matchResp(frame: ServerFrame, reqId: string): RespOKFrame | RespErrFrame | null {
  if (Array.isArray(frame)) return null;
  if (frame.k === "r" && frame.reqId === reqId) return frame;
  if (frame.k === "e" && frame.reqId === reqId) return frame;
  return null;
}
