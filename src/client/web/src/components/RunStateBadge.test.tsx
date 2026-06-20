import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RunStateBadge } from "./RunStateBadge";

describe("RunStateBadge", () => {
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
});
