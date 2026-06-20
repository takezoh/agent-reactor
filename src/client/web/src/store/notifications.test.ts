import { beforeEach, describe, expect, it } from "vitest";
import type { NotificationFrame } from "../wire/server";
import { useNotificationsStore } from "./notifications";

describe("notificationsStore", () => {
  beforeEach(() => {
    useNotificationsStore.getState().clear();
  });

  it("add appends an item with a fresh id", () => {
    useNotificationsStore.getState().add({ level: "info", message: "hi" });
    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(1);
    expect(items[0]?.level).toBe("info");
    expect(items[0]?.message).toBe("hi");
  });

  it("dismiss removes by id", () => {
    useNotificationsStore.getState().add({ level: "warn", message: "x" });
    const id = useNotificationsStore.getState().items[0]?.id;
    if (id === undefined) throw new Error("expected id");
    useNotificationsStore.getState().dismiss(id);
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("LRU evicts oldest when exceeding 32 items", () => {
    for (let i = 0; i < 40; i++) {
      useNotificationsStore.getState().add({ level: "info", message: `m${i}` });
    }
    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(32);
    // oldest preserved should be m8 (40 - 32 = 8 dropped)
    expect(items[0]?.message).toBe("m8");
    expect(items[items.length - 1]?.message).toBe("m39");
  });

  describe("addFromFrame", () => {
    const makeFrame = (overrides: Partial<NotificationFrame> = {}): NotificationFrame => ({
      k: "n",
      sessionId: "s1",
      cmd: 9,
      nowMs: 123,
      ...overrides,
    });

    it("TestAddFromFrame: adds item with expected fields", () => {
      useNotificationsStore.getState().addFromFrame(makeFrame({ title: "hi", body: "world" }));
      const items = useNotificationsStore.getState().items;
      expect(items).toHaveLength(1);
      const item = items[0];
      expect(item?.message).toBe("hi");
      expect(item?.level).toBe("info");
      expect(item?.sessionId).toBe("s1");
      expect(item?.cmd).toBe(9);
      expect(item?.title).toBe("hi");
      expect(item?.body).toBe("world");
    });

    it("TestAddFromFrame_LRU32: caps at 32 items after 33 frames", () => {
      for (let i = 0; i < 33; i++) {
        useNotificationsStore.getState().addFromFrame(makeFrame({ title: `t${i}` }));
      }
      expect(useNotificationsStore.getState().items).toHaveLength(32);
    });

    it("TestAddFromFrame_BodyOnly: uses body as message when title absent", () => {
      useNotificationsStore.getState().addFromFrame(makeFrame({ title: undefined, body: "x" }));
      const items = useNotificationsStore.getState().items;
      expect(items[0]?.message).toBe("x");
    });

    it("falls back to OSC <cmd> when both title and body absent", () => {
      useNotificationsStore
        .getState()
        .addFromFrame(makeFrame({ title: undefined, body: undefined, cmd: 42 }));
      const items = useNotificationsStore.getState().items;
      expect(items[0]?.message).toBe("OSC 42");
    });
  });
});
