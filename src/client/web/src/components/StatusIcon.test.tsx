import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusIcon, isActiveStatus, normalizeStatus } from "./StatusIcon";

describe("normalizeStatus", () => {
  it.each([
    ["running", "running"],
    ["waiting", "waiting"],
    ["idle", "idle"],
    ["stopped", "stopped"],
    ["pending", "pending"],
  ] as const)("known status %s → kind %s", (raw, want) => {
    expect(normalizeStatus(raw)).toBe(want);
  });

  it.each([undefined, "", "foo", "RUNNING", "bogus"] as (string | undefined)[])(
    "unknown / malformed status %s → 'unknown'",
    (raw) => {
      expect(normalizeStatus(raw)).toBe("unknown");
    },
  );
});

describe("isActiveStatus", () => {
  it("returns true only for running and waiting", () => {
    expect(isActiveStatus("running")).toBe(true);
    expect(isActiveStatus("waiting")).toBe(true);
    expect(isActiveStatus("idle")).toBe(false);
    expect(isActiveStatus("stopped")).toBe(false);
    expect(isActiveStatus("pending")).toBe(false);
    expect(isActiveStatus("unknown")).toBe(false);
  });
});

describe("StatusIcon", () => {
  // ADR-0078: every status renders an <svg> with status-icon + per-kind modifier.
  it.each([
    ["running", "status-icon--running"],
    ["waiting", "status-icon--waiting"],
    ["idle", "status-icon--idle"],
    ["stopped", "status-icon--stopped"],
    ["pending", "status-icon--pending"],
    ["unknown", "status-icon--unknown"],
  ] as const)("renders <svg.status-icon.%s> for status=%s", (kind, modifier) => {
    const { container } = render(<StatusIcon status={kind} />);
    const svg = container.querySelector(`svg.status-icon.${modifier}`);
    expect(svg).not.toBeNull();
  });

  // ADR-0078: distinctive inner elements per status so CSS can target them.
  it("running renders ring + arc paths", () => {
    const { container } = render(<StatusIcon status="running" />);
    expect(container.querySelector(".status-icon__ring")).not.toBeNull();
    expect(container.querySelector(".status-icon__arc")).not.toBeNull();
  });

  it("waiting renders three .status-icon__dot circles with stagger modifiers", () => {
    const { container } = render(<StatusIcon status="waiting" />);
    expect(container.querySelectorAll(".status-icon__dot").length).toBe(3);
    expect(container.querySelector(".status-icon__dot--1")).not.toBeNull();
    expect(container.querySelector(".status-icon__dot--2")).not.toBeNull();
    expect(container.querySelector(".status-icon__dot--3")).not.toBeNull();
  });

  it("pending renders a dashed ring", () => {
    const { container } = render(<StatusIcon status="pending" />);
    expect(container.querySelector(".status-icon__dashed")).not.toBeNull();
  });

  it("idle renders a filled dot", () => {
    const { container } = render(<StatusIcon status="idle" />);
    expect(container.querySelector(".status-icon__filled")).not.toBeNull();
  });

  it("stopped renders a square", () => {
    const { container } = render(<StatusIcon status="stopped" />);
    expect(container.querySelector(".status-icon__square")).not.toBeNull();
  });

  it("unknown renders a dash line", () => {
    const { container } = render(<StatusIcon status="unknown" />);
    expect(container.querySelector(".status-icon__dash")).not.toBeNull();
  });

  // activeClass / inactiveClass: caller-supplied class layered for legacy
  // DOM contracts (run-state-spinner / session-status-spinner).
  it("layers activeClass on active states (running)", () => {
    const { container } = render(
      <StatusIcon
        status="running"
        activeClass="run-state-spinner"
        inactiveClass="run-state-icon"
      />,
    );
    expect(container.querySelector(".run-state-spinner")).not.toBeNull();
    expect(container.querySelector(".run-state-icon")).toBeNull();
  });

  it("layers inactiveClass on inactive states (idle)", () => {
    const { container } = render(
      <StatusIcon status="idle" activeClass="run-state-spinner" inactiveClass="run-state-icon" />,
    );
    expect(container.querySelector(".run-state-icon")).not.toBeNull();
    expect(container.querySelector(".run-state-spinner")).toBeNull();
  });

  it("svg carries aria-hidden=true so screen readers ignore the decorative icon", () => {
    const { container } = render(<StatusIcon status="running" />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("aria-hidden")).toBe("true");
  });
});
