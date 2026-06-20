import { describe, expect, it } from "vitest";
import type { RespErrFrame, RespOKFrame } from "../wire/server";
import { type RetryDeps, subscribeWithRetry } from "./retry";

function makeDeps(responses: (RespOKFrame | RespErrFrame)[]): {
  deps: RetryDeps;
  sent: string[];
  sleeps: number[];
} {
  const sent: string[] = [];
  const sleeps: number[] = [];
  let idx = 0;
  const deps: RetryDeps = {
    send: (s) => sent.push(s),
    awaitResponse: async (_reqId) => {
      const r = responses[idx];
      idx += 1;
      if (!r) throw new Error("ran out of fake responses");
      return r;
    },
    newReqId: () => `req-${idx}`,
    sleep: async (ms) => {
      sleeps.push(ms);
    },
    rng: () => 0.5,
  };
  return { deps, sent, sleeps };
}

describe("subscribeWithRetry", () => {
  it("succeeds on first try when RespOK comes back", async () => {
    const { deps, sent } = makeDeps([{ k: "r", reqId: "x" }]);
    const out = await subscribeWithRetry("s1", deps);
    expect(out.status).toBe("confirmed");
    expect(sent).toHaveLength(1);
  });

  it("retries on frame-not-ready 3 times then confirms (fake WS scenario)", async () => {
    const errs: RespErrFrame[] = Array.from({ length: 3 }, (_, i) => ({
      k: "e" as const,
      reqId: `req-${i}`,
      code: "frame-not-ready",
      message: "not yet",
    }));
    const { deps, sent, sleeps } = makeDeps([...errs, { k: "r", reqId: "ok" }]);
    const out = await subscribeWithRetry("s1", deps);
    expect(out.status).toBe("confirmed");
    // 4 sends: 3 retries + 1 success
    expect(sent).toHaveLength(4);
    // 3 sleeps between attempts
    expect(sleeps).toHaveLength(3);
  });

  it("gives up after 16 attempts — user-action waiting transition", async () => {
    // 16 frame-not-ready errors exhaust MAX_ATTEMPTS=16
    const errs: RespErrFrame[] = Array.from({ length: 16 }, () => ({
      k: "e" as const,
      reqId: "x",
      code: "frame-not-ready",
      message: "not yet",
    }));
    const { deps, sent } = makeDeps(errs);
    const out = await subscribeWithRetry("s1", deps);
    // exhausted — caller transitions to user-action-waiting state
    expect(out.status).toBe("exhausted");
    if (out.status === "exhausted") {
      expect(out.lastError).toBe("frame-not-ready");
    }
    // sent exactly 16 frames (one per attempt)
    expect(sent).toHaveLength(16);
  });

  it("does not retry on non-frame-not-ready error", async () => {
    const { deps, sent } = makeDeps([{ k: "e", reqId: "x", code: "unauthorized", message: "no" }]);
    const out = await subscribeWithRetry("s1", deps);
    expect(out.status).toBe("exhausted");
    if (out.status === "exhausted") expect(out.lastError).toBe("unauthorized");
    expect(sent).toHaveLength(1);
  });
});
