import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { StatusBanner } from "./StatusBanner";

describe("StatusBanner", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
  });

  it("hidden on status open and not daemon-disconnected", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    const { container } = render(<StatusBanner />);
    expect(container.firstChild).toBeNull();
  });

  it("shows reconnecting message", () => {
    useDaemonStore.setState({ status: "reconnecting" });
    render(<StatusBanner />);
    expect(screen.getByRole("status").textContent).toMatch(/reconnecting/);
  });

  it("shows daemon-disconnected message", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: true });
    render(<StatusBanner />);
    expect(screen.getByRole("status").textContent).toMatch(/daemon/);
  });
});
