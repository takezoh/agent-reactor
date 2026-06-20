import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";

describe("App", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    // Stub fetch to hang forever so Connection.start() never rejects and
    // no unhandled rejection leaks out of the voided conn.start() in useEffect.
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    // hash token を仕込んで Connection を初期化させる
    window.location.hash = "#token=test";
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    window.location.hash = "";
  });

  it("renders DriverViewPanel for active session", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Hello driver" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    render(<App />);
    // Title appears in both SessionList row and DriverViewPanel heading
    const titles = screen.getAllByText("Hello driver");
    expect(titles.length).toBeGreaterThanOrEqual(1);
    // RunStateBadge appears in sidebar and in DriverViewPanel header
    const badges = screen.getAllByLabelText(/status: running/);
    expect(badges.length).toBeGreaterThanOrEqual(1);
    // DriverViewPanel section is rendered
    expect(screen.getByLabelText("driver view")).toBeTruthy();
  });

  it("hides driver view when no active session", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("driver view")).toBeNull();
  });

  it("renders LogTabs tablist when active session has log_tabs", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {},
            status: "running",
            log_tabs: [{ label: "Output", path: "/tmp/out.log", kind: "text" }],
          },
        },
      ],
      activeSessionID: "s2",
    });
    render(<App />);
    expect(screen.getByRole("tablist")).toBeTruthy();
  });

  it("renders ConnectorPanel container as null when connectors store is empty", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    // ConnectorPanel returns null when connectors is empty
    expect(screen.queryByLabelText("connectors")).toBeNull();
  });

  it("renders NotificationToast aria-label container", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.getByLabelText("notifications")).toBeTruthy();
  });
});
