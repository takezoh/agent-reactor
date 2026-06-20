import { describe, expect, it } from "vitest";
import { SubscriptionRegistry } from "./subscribe";

describe("SubscriptionRegistry", () => {
  it("set/list/remove flow", () => {
    const r = new SubscriptionRegistry();
    r.set({ sessionId: "s1", reqId: "r1" });
    expect(r.size()).toBe(1);
    expect(r.list()[0]?.sessionId).toBe("s1");
    r.remove("s1");
    expect(r.size()).toBe(0);
  });

  it("overwriting same sessionId keeps size=1", () => {
    const r = new SubscriptionRegistry();
    r.set({ sessionId: "s1", reqId: "r1" });
    r.set({ sessionId: "s1", reqId: "r2" });
    expect(r.size()).toBe(1);
    expect(r.list()[0]?.reqId).toBe("r2");
  });

  it("clear removes all entries", () => {
    const r = new SubscriptionRegistry();
    r.set({ sessionId: "s1", reqId: "r1" });
    r.set({ sessionId: "s2", reqId: "r2" });
    r.clear();
    expect(r.size()).toBe(0);
  });

  it("remove non-existent key is a no-op", () => {
    const r = new SubscriptionRegistry();
    r.remove("nonexistent");
    expect(r.size()).toBe(0);
  });
});
