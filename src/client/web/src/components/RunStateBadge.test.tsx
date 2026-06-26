import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RunStateBadge } from "./RunStateBadge";

describe("RunStateBadge", () => {
  // FR-010: existing textContent / aria-label contract preserved for all statuses
  it.each([
    ["running", "running"],
    ["waiting", "waiting"],
    ["idle", "idle"],
    ["stopped", "stopped"],
    ["pending", "pending"],
    [undefined, "unknown"],
  ] as [string | undefined, string][])(
    "status=%s renders class run-state-%s and text %s",
    (status, want) => {
      render(<RunStateBadge status={status} />);
      const el = screen.getByLabelText(/status:/);
      expect(el.className).toContain(`run-state-${want}`);
      expect(el.textContent).toBe(want);
    },
  );

  // FR-009 (ADR-0078): all statuses render exactly one .status-icon — the
  // visual differs per status but the slot is always populated.
  it.each(["running", "waiting", "idle", "stopped", "pending", undefined] as (
    | string
    | undefined
  )[])("status=%s renders one .status-icon", (status) => {
    const { container } = render(<RunStateBadge status={status} />);
    const icons = container.querySelectorAll(".status-icon");
    expect(icons.length).toBe(1);
  });

  // FR-009 (ADR-0078): active states keep the legacy .run-state-spinner
  // contract class so existing DOM queries continue to find them.
  it.each(["running", "waiting"] as string[])(
    "active status=%s carries .run-state-spinner on its icon",
    (status) => {
      const { container } = render(<RunStateBadge status={status} />);
      const spinners = container.querySelectorAll(".run-state-spinner");
      expect(spinners.length).toBe(1);
    },
  );

  // FR-009 (ADR-0078): inactive states never carry .run-state-spinner
  // (they get .run-state-icon instead).
  it.each(["idle", "stopped", "pending", undefined] as (string | undefined)[])(
    "inactive status=%s carries .run-state-icon (not .run-state-spinner)",
    (status) => {
      const { container } = render(<RunStateBadge status={status} />);
      expect(container.querySelectorAll(".run-state-spinner").length).toBe(0);
      expect(container.querySelectorAll(".run-state-icon").length).toBe(1);
    },
  );

  // ADR-0078: every variant has a matching .status-icon--<kind> modifier
  // so CSS can target it for per-status visuals.
  it.each([
    ["running", "status-icon--running"],
    ["waiting", "status-icon--waiting"],
    ["idle", "status-icon--idle"],
    ["stopped", "status-icon--stopped"],
    ["pending", "status-icon--pending"],
    [undefined, "status-icon--unknown"],
  ] as [string | undefined, string][])(
    "status=%s renders .status-icon with modifier %s",
    (status, modifier) => {
      const { container } = render(<RunStateBadge status={status} />);
      const icon = container.querySelector(`.status-icon.${modifier}`);
      expect(icon).not.toBeNull();
    },
  );
});
