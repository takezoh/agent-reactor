import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { SessionList } from "./SessionList";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
  unsubscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

describe("SessionList", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    useDaemonStore.setState({
      sessions: [
        { id: "s1", title: "alpha", status: "running", createdAt: 1 },
        { id: "s2", title: "beta", status: "stopped", createdAt: 2 },
      ],
    });
  });

  it("renders sessions", () => {
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("alpha")).toBeDefined();
    expect(screen.getByText("beta")).toBeDefined();
  });

  it("unsubscribes previous then subscribes new on selection", async () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    fireEvent.click(screen.getByText("beta"));
    // unsubscribe should be called for s1, subscribe for s2
    await Promise.resolve();
    await Promise.resolve();
    expect((fakeConn.unsubscribe as ReturnType<typeof vi.fn>).mock.calls.flat()).toContain("s1");
    expect((fakeConn.subscribe as ReturnType<typeof vi.fn>).mock.calls.flat()).toContain("s2");
  });
});
