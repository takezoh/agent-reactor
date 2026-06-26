import * as fs from "node:fs";
import * as path from "node:path";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusIcon, isActiveStatus, normalizeStatus } from "./StatusIcon";

const STATUS_ICON_CSS = fs.readFileSync(
  path.resolve(__dirname, "../css/status-icon.css"),
  "utf-8",
);

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
  // ADR-0078: every status renders a <span class="status-icon status-icon--<kind>">
  // wrapping a <svg> with the geometry. The OUTER span is the element that
  // carries any rotation animation (HTML-wrapper pattern — see StatusIcon.tsx).
  it.each([
    ["running", "status-icon--running"],
    ["waiting", "status-icon--waiting"],
    ["idle", "status-icon--idle"],
    ["stopped", "status-icon--stopped"],
    ["pending", "status-icon--pending"],
    ["unknown", "status-icon--unknown"],
  ] as const)("renders <span.status-icon.%s> wrapping an <svg> for status=%s", (kind, modifier) => {
    const { container } = render(<StatusIcon status={kind} />);
    const span = container.querySelector(`span.status-icon.${modifier}`);
    expect(span, `expected span.status-icon.${modifier}`).not.toBeNull();
    expect(span?.querySelector("svg"), "expected nested <svg>").not.toBeNull();
  });

  // ADR-0078: distinctive inner elements per status so CSS can target them.
  it("running renders ring + arc paths inside the SVG", () => {
    const { container } = render(<StatusIcon status="running" />);
    const span = container.querySelector(".status-icon--running");
    expect(span?.querySelector("svg .status-icon__ring")).not.toBeNull();
    expect(span?.querySelector("svg .status-icon__arc")).not.toBeNull();
  });

  it("pending renders a dashed ring inside the SVG", () => {
    const { container } = render(<StatusIcon status="pending" />);
    const span = container.querySelector(".status-icon--pending");
    expect(span?.querySelector("svg .status-icon__dashed")).not.toBeNull();
  });

  it("waiting renders three .status-icon__dot circles with stagger modifiers", () => {
    const { container } = render(<StatusIcon status="waiting" />);
    expect(container.querySelectorAll(".status-icon__dot").length).toBe(3);
    expect(container.querySelector(".status-icon__dot--1")).not.toBeNull();
    expect(container.querySelector(".status-icon__dot--2")).not.toBeNull();
    expect(container.querySelector(".status-icon__dot--3")).not.toBeNull();
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
  // DOM contracts (run-state-spinner / session-status-spinner). Now applied
  // to the OUTER span (where the rotation animation lives).
  it("layers activeClass on active states (running)", () => {
    const { container } = render(
      <StatusIcon
        status="running"
        activeClass="run-state-spinner"
        inactiveClass="run-state-icon"
      />,
    );
    expect(container.querySelector("span.run-state-spinner")).not.toBeNull();
    expect(container.querySelector(".run-state-icon")).toBeNull();
  });

  it("layers inactiveClass on inactive states (idle)", () => {
    const { container } = render(
      <StatusIcon status="idle" activeClass="run-state-spinner" inactiveClass="run-state-icon" />,
    );
    expect(container.querySelector("span.run-state-icon")).not.toBeNull();
    expect(container.querySelector(".run-state-spinner")).toBeNull();
  });

  it("wrapper span carries aria-hidden=true so screen readers ignore the decorative icon", () => {
    const { container } = render(<StatusIcon status="running" />);
    const span = container.querySelector("span.status-icon");
    expect(span?.getAttribute("aria-hidden")).toBe("true");
  });

  // ADR-0078 rotation contract: animation rule must target the wrapper span
  // directly (.status-icon--running / .status-icon--pending), not a child.
  it.each(["running", "pending"] as const)(
    "%s animation rule targets the wrapper span, not a child element",
    (kind) => {
      const rule = new RegExp(`\\.status-icon--${kind}\\s*\\{[^}]*animation:\\s*status-icon-spin`);
      expect(STATUS_ICON_CSS).toMatch(rule);
    },
  );
});
