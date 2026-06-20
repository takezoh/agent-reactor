import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { View } from "../wire/server";
import { DriverViewPanel } from "./DriverViewPanel";

function makeView(overrides: Partial<View> = {}): View {
  return {
    card: {},
    ...overrides,
  };
}

describe("DriverViewPanel", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders card title and subtitle", () => {
    const view = makeView({ card: { title: "My Title", subtitle: "My Subtitle" } });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByText("My Title")).toBeTruthy();
    expect(screen.getByText("My Subtitle")).toBeTruthy();
  });

  it("renders tags", () => {
    const view = makeView({
      card: {
        tags: [
          { text: "alpha", fg: "#fff" },
          { text: "beta", bg: "#333" },
        ],
      },
    });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByText("alpha")).toBeTruthy();
    expect(screen.getByText("beta")).toBeTruthy();
  });

  it("renders RunStateBadge for view.status", () => {
    const view = makeView({ status: "running" });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByLabelText("status: running")).toBeTruthy();
  });

  it("renders status_line and ticking elapsed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));

    const view = makeView({
      status_line: "Running task",
      status_changed_at: "2026-06-19T23:59:55Z",
    });
    render(<DriverViewPanel view={view} />);

    // Initial render: 5 seconds elapsed
    expect(screen.getByLabelText("elapsed").textContent).toBe("5s");

    // Advance 2 seconds — hook fires twice → elapsed becomes 7s
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByLabelText("elapsed").textContent).toBe("7s");
  });

  it("hides border row when all border fields are empty", () => {
    const view = makeView({ card: { title: "T" } });
    const { container } = render(<DriverViewPanel view={view} />);
    const borderRow = container.querySelector(".driver-view-border");
    expect(borderRow).toBeNull();
  });

  it("suppresses status_line when absent", () => {
    const view = makeView({ card: { title: "T" } });
    const { container } = render(<DriverViewPanel view={view} />);
    const footer = container.querySelector(".driver-view-footer");
    expect(footer).toBeNull();
  });
});
