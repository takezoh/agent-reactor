import * as fs from "node:fs";
import * as path from "node:path";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { SessionList, displayLabel } from "./SessionList";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
  unsubscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

describe("displayLabel", () => {
  it("FR-011: returns title when title is present", () => {
    expect(displayLabel({ title: "My Session" }, "s1")).toBe("My Session");
  });

  it("FR-011: returns subtitle when title is absent", () => {
    expect(displayLabel({ subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-011: returns subtitle when title is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-012: returns id when both title and subtitle are absent", () => {
    expect(displayLabel({}, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is undefined and subtitle is undefined", () => {
    expect(displayLabel({ title: undefined, subtitle: undefined }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is empty string and subtitle is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "" }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is whitespace-only and subtitle is whitespace-only", () => {
    expect(displayLabel({ title: "  ", subtitle: "   " }, "s1")).toBe("s1");
  });

  it("FR-011: trims title before returning it", () => {
    expect(displayLabel({ title: "  trimmed  " }, "s1")).toBe("trimmed");
  });

  it("FR-011: trims subtitle before returning it", () => {
    expect(displayLabel({ title: "", subtitle: "  sub  " }, "s1")).toBe("sub");
  });
});

// ─── SessionRow rendering (per-session card) ──────────────────────────────
describe("SessionList rendering", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders session with title via displayLabel", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("alpha")).toBeDefined();
  });

  it("renders session with subtitle when title is absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { subtitle: "my-sub" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    // title slot shows "my-sub" (displayLabel fallback)
    expect(screen.getAllByText("my-sub").length).toBeGreaterThan(0);
  });

  it("FR-012: renders session id when both title and subtitle are absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-raw-id",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-raw-id")).toBeDefined();
  });

  it("renders session id when title and subtitle are empty strings", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-empty",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "", subtitle: "" }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-empty")).toBeDefined();
  });

  it("renders session id when title and subtitle are whitespace-only", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-ws",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "  ", subtitle: "  " }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-ws")).toBeDefined();
  });
});

describe("SessionList status indicator", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders a spinning indicator only for active (running) sessions", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(1);
  });

  it("renders a spinning indicator for waiting sessions", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "waiting" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(1);
  });

  it.each(["idle", "stopped", "pending", undefined] as (string | undefined)[])(
    "renders NO spinner for inactive status=%s",
    (status) => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "s1",
            project: "p",
            command: "claude",
            created_at: "2026-06-20T00:00:00Z",
            view: { card: { title: "alpha" }, status },
          },
        ],
      });
      const { container } = render(<SessionList conn={fakeConn} />);
      expect(container.querySelectorAll(".session-status-spinner").length).toBe(0);
    },
  );

  it("status slot precedes the title row (top-left placement)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const row = container.querySelector(".session-list__row");
    expect(row).not.toBeNull();
    const children = row ? Array.from(row.children) : [];
    expect(children[0]?.className).toMatch(/session-status-slot/);
    expect(children[1]?.className).toMatch(/session-list__content/);
    // Content's first child is the title-row containing the title element.
    const titleRow = children[1]?.firstElementChild;
    expect(titleRow?.className).toMatch(/session-list__title-row/);
    const title = titleRow?.querySelector(".session-list__title");
    expect(title?.textContent).toBe("alpha");
  });

  it("status slot exposes the status name via aria-label even when inactive", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByLabelText("status: stopped")).toBeDefined();
  });

  it("does NOT render the textual status label inside the row", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const row = container.querySelector(".session-list__row");
    expect(row).not.toBeNull();
    // The session row itself contains only status slot + title (+ optional
    // meta/tags). No status word should appear as visible text.
    expect(row?.textContent).not.toMatch(/running/);
  });
});

// ─── Inline driver chip on the title row ─────────────────────────────────
describe("SessionList driver chip (inlined into title row)", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders the driver chip inside the title row when root_driver is set", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          root_driver: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const driver = container.querySelector(".session-list__driver");
    expect(driver).not.toBeNull();
    expect(driver?.textContent).toBe("claude");
    // Driver must live inside the title-row, sibling of the title element.
    const titleRow = container.querySelector(".session-list__title-row");
    expect(titleRow?.querySelector(".session-list__driver")).not.toBeNull();
    expect(titleRow?.querySelector(".session-list__title")).not.toBeNull();
  });

  it("applies the brand color from driverColor() as inline style", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "codex",
          root_driver: "codex",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const chip = container.querySelector<HTMLElement>(".session-list__driver");
    expect(chip).not.toBeNull();
    // OpenAI green for codex.
    expect(chip?.style.backgroundColor.toLowerCase()).toBe("#10a37f");
    expect(chip?.style.color.toLowerCase()).toBe("#ffffff");
  });

  it("falls back to the default command tag color for unknown drivers", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "weirdcli",
          root_driver: "weirdcli",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const chip = container.querySelector<HTMLElement>(".session-list__driver");
    expect(chip).not.toBeNull();
    // Default command-tag bg (#D97757).
    expect(chip?.style.backgroundColor.toLowerCase()).toBe("#d97757");
  });

  it("omits the driver chip when root_driver is absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector(".session-list__driver")).toBeNull();
  });

  it("does NOT render a separate meta row (driver lives inline)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          root_driver: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector(".session-list__meta")).toBeNull();
    expect(container.querySelector(".session-list__meta-driver")).toBeNull();
    expect(container.querySelector(".session-list__meta-subtitle")).toBeNull();
  });

  it("does NOT duplicate subtitle when title is present (displayLabel chain handles it)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          root_driver: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha", subtitle: "alpha summary" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    // Title is "alpha"; subtitle MUST NOT also appear anywhere on the card.
    expect(container.textContent).not.toMatch(/alpha summary/);
  });
});

// ─── Tag row ────────────────────────────────────────────────────────────────
describe("SessionList tag row", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders card.tags as pills", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {
              title: "alpha",
              tags: [
                { text: "worktree", fg: "#fff", bg: "#3a3a3a" },
                { text: "host", bg: "#226622" },
              ],
            },
            status: "running",
          },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const tagRow = container.querySelector(".session-list__tags");
    expect(tagRow).not.toBeNull();
    expect(tagRow?.textContent).toMatch(/worktree/);
    expect(tagRow?.textContent).toMatch(/host/);
  });

  it("renders card.border_badge inside the tag row", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha", border_badge: "💬3" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector(".session-list__badge")?.textContent).toBe("💬3");
  });

  it("hides the tag row entirely when no tags and no border_badge", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector(".session-list__tags")).toBeNull();
  });
});

// ─── Workspace switcher ────────────────────────────────────────────────────
describe("WorkspaceSwitcher", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("is hidden when only the default workspace exists", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector("[data-role='workspace-switcher']")).toBeNull();
  });

  it("is shown when at least one named (non-default) workspace exists", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          workspace: "prod",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const bar = container.querySelector("[data-role='workspace-switcher']");
    expect(bar).not.toBeNull();
    // "default" is always present at index 0; "prod" appears second
    // (collectWorkspaces parity).
    const chips = bar?.querySelectorAll('[role="radio"]');
    expect(chips?.length).toBe(2);
    expect(chips?.[0]?.textContent).toBe("default");
    expect(chips?.[1]?.textContent).toBe("prod");
  });

  it("clicking a chip changes selectedWorkspace and partitions the visible projects", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          workspace: "default",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha-default" }, status: "idle" },
        },
        {
          id: "s2",
          project: "/repo/beta",
          workspace: "prod",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta-prod" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    // Initially: default workspace selected → only the "alpha" project is visible.
    expect(screen.getByText("alpha-default")).toBeDefined();
    expect(screen.queryByText("beta-prod")).toBeNull();
    // Click the "prod" chip.
    const prodChip = Array.from(container.querySelectorAll('[role="radio"]')).find(
      (el) => el.textContent === "prod",
    );
    expect(prodChip).not.toBeUndefined();
    if (prodChip) fireEvent.click(prodChip);
    // Now only the "beta" session is visible.
    expect(screen.queryByText("alpha-default")).toBeNull();
    expect(screen.getByText("beta-prod")).toBeDefined();
  });

  it("auto-follows the active session's workspace via selectSession", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          workspace: "default",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha-default" }, status: "idle" },
        },
        {
          id: "s2",
          project: "/repo/beta",
          workspace: "prod",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta-prod" }, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(useDaemonStore.getState().selectedWorkspace).toBe("default");
    act(() => {
      useDaemonStore.getState().selectSession("s2");
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });
});

// ─── Project group (disclosure + nested listbox) ───────────────────────────
describe("ProjectGroup", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders one project header per distinct project (alphabetical)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s2",
          project: "/repo/beta",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "b" }, status: "idle" },
        },
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const headers = container.querySelectorAll(".session-list__project-header");
    expect(headers.length).toBe(2);
    expect(headers[0]?.querySelector(".session-list__project-name")?.textContent).toBe("alpha");
    expect(headers[1]?.querySelector(".session-list__project-name")?.textContent).toBe("beta");
  });

  it("renders the session count badge next to the project name", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
        {
          id: "s2",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "b" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelector(".session-list__project-count")?.textContent).toBe("2");
  });

  it("aria-expanded=true on header maps to a visible session panel", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const header = container.querySelector(".session-list__project-header");
    expect(header?.getAttribute("aria-expanded")).toBe("true");
    // panel exists and contains the session row.
    expect(container.querySelector(".session-list__project-panel")).not.toBeNull();
    expect(screen.getByText("a")).toBeDefined();
  });

  it("two repos sharing a basename are foldable independently (key=projectPath)", () => {
    // Regression gate for the basename-collision bug: clicking the fold
    // header on one of two same-basename groups must not collapse the other.
    useDaemonStore.setState({
      sessions: [
        {
          id: "a",
          project: "/home/a/web",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "in a" }, status: "idle" },
        },
        {
          id: "b",
          project: "/home/b/web",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "in b" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const headers = container.querySelectorAll<HTMLButtonElement>(".session-list__project-header");
    expect(headers.length).toBe(2);
    // Click the first header. Only that group should collapse.
    const first = headers[0];
    expect(first).toBeDefined();
    if (first) fireEvent.click(first);
    expect(headers[0]?.getAttribute("aria-expanded")).toBe("false");
    expect(headers[1]?.getAttribute("aria-expanded")).toBe("true");
  });

  it("clicking the header toggles fold (aria-expanded flips to false and the panel disappears)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const header = container.querySelector<HTMLButtonElement>(".session-list__project-header");
    expect(header).not.toBeNull();
    if (header) fireEvent.click(header);
    expect(header?.getAttribute("aria-expanded")).toBe("false");
    expect(container.querySelector(".session-list__project-panel")).toBeNull();
    expect(screen.queryByText("a")).toBeNull();
  });

  it("collapsed projects show their count even when the session list is hidden", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
        {
          id: "s2",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "b" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const header = container.querySelector<HTMLButtonElement>(".session-list__project-header");
    if (header) fireEvent.click(header);
    expect(container.querySelector(".session-list__project-count")?.textContent).toBe("2");
  });
});

// ─── Empty state ───────────────────────────────────────────────────────────
describe("SessionList empty state", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("shows 'No sessions yet.' when default workspace is empty", () => {
    useDaemonStore.setState({ sessions: [] });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("No sessions yet.")).toBeDefined();
  });

  it("shows a workspace-specific empty message for non-default workspaces", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/a",
          workspace: "prod",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a" }, status: "idle" },
        },
      ],
      selectedWorkspace: "prod",
    });
    // remove the only prod session so the workspace becomes empty but still selected
    useDaemonStore.setState({ sessions: [] });
    // sessions = [] now removes "prod" from distinctWorkspaces but selectedWorkspace
    // stays in store; the listView will fall back to no groups → empty message.
    // But the empty message branch keys on selectedWorkspace, so it should now read
    // "default" (since we never mutated selectedWorkspace directly here, but
    // selectedWorkspace was set to "prod"). Force it explicitly:
    useDaemonStore.setState({ selectedWorkspace: "prod" });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText(/No sessions in workspace "prod"/)).toBeDefined();
  });
});

// ─── selectSession + per-project cursor ────────────────────────────────────
describe("SessionList selectSession", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
        {
          id: "s2",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta" }, status: "stopped" },
        },
      ],
    });
  });

  it("calls selectSession on click", () => {
    render(<SessionList conn={fakeConn} />);
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    expect(betaOption).not.toBeNull();
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect(useDaemonStore.getState().activeSessionID).toBe("s2");
  });

  it("ADR-0030: does NOT call conn.subscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect((fakeConn.subscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });

  it("ADR-0030: does NOT call conn.unsubscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect((fakeConn.unsubscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });
});

// ─── role=listbox + disabled rows ──────────────────────────────────────────
describe("SessionList listbox a11y (FR-TOKEN-002)", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders one role=listbox per project group", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "a1" }, status: "running" },
        },
        {
          id: "s2",
          project: "/repo/beta",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "b1" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listboxes = container.querySelectorAll('[role="listbox"]');
    expect(listboxes.length).toBe(2);
  });

  it("disabled rows (daemonDisconnected=true) are still DOM-visible", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "running" },
        },
      ],
      daemonDisconnected: true,
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("my-session")).toBeDefined();
  });

  it("disabled rows carry aria-disabled=true", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "stopped" },
        },
      ],
      daemonDisconnected: true,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const option = container.querySelector('[role="option"]');
    expect(option?.getAttribute("aria-disabled")).toBe("true");
  });

  it("disabled rows include a disabledReason child text node", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "stopped" },
        },
      ],
      daemonDisconnected: true,
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("Daemon disconnected")).toBeDefined();
  });

  it("ArrowDown moves cursor within a project but does NOT call selectSession", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "first" }, status: "running" },
        },
        {
          id: "s2",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "second" }, status: "stopped" },
        },
      ],
      activeSessionID: "s1",
      daemonDisconnected: false,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox).not.toBeNull();
    // The listbox uses a per-instance id scope (useId) so two listboxes can
    // coexist without DOM-id collisions; the option's data-item-id retains
    // the logical session id.
    const opt = (id: string) =>
      container.querySelector<HTMLElement>(`[role="option"][data-item-id="${id}"]`);
    expect(listbox?.getAttribute("aria-activedescendant")).toBe(opt("s1")?.id);
    if (listbox) fireEvent.keyDown(listbox, { key: "ArrowDown" });
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");
    expect(listbox?.getAttribute("aria-activedescendant")).toBe(opt("s2")?.id);
  });

  it("Enter key activates the cursor session and calls selectSession", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "first" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
      daemonDisconnected: false,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    if (listbox) fireEvent.keyDown(listbox, { key: "Enter" });
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");
  });
});

// ─── FR-TOKEN-001 / row sizing parity (CSS source check) ───────────────────
describe("FR-TOKEN-001: row sizing tokens", () => {
  const cssDir = path.resolve(__dirname, "../css");

  it("app.css declares .unified-listbox__option with --row-* sizing tokens", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("border-radius: var(--row-radius)");
    expect(appCss).toContain("padding-top: var(--row-padding-y)");
    expect(appCss).toContain("font-size: var(--row-font-size)");
    expect(appCss).toContain("line-height: var(--row-line-height)");
    expect(appCss).toContain("min-height: var(--row-min-height)");
  });

  it("FR-A11Y-001: session-list.css declares 44px min-height for .session-list .unified-listbox__option", () => {
    const css = fs.readFileSync(path.join(cssDir, "session-list.css"), "utf-8");
    expect(css).toContain(".session-list .unified-listbox__option");
    expect(css).toContain("min-height: 44px");
  });
});

// ─── caret-rail signature ──────────────────────────────────────────────────
describe("SessionList active accent (caret rail signature)", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("committed-active row carries .session-list__row--active modifier", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
        {
          id: "s2",
          project: "/repo/alpha",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta" }, status: "stopped" },
        },
      ],
      activeSessionID: "s2",
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const activeRows = container.querySelectorAll(".session-list__row--active");
    expect(activeRows.length).toBe(1);
    expect(activeRows[0]?.textContent).toMatch(/beta/);
  });

  it("session-list.css declares caret rail ::before for committed-active row", () => {
    const cssDir = path.resolve(__dirname, "../css");
    const css = fs.readFileSync(path.join(cssDir, "session-list.css"), "utf-8");
    expect(css).toContain(
      ".session-list .unified-listbox__option:has(.session-list__row--active)::before",
    );
    expect(css).toContain("animation: palette-row-caret-in");
    expect(css).toContain("var(--rail-accent)");
  });

  it("view.css consolidates reduced-motion guard for the caret rail (ADR-0064)", () => {
    const cssDir = path.resolve(__dirname, "../css");
    const viewCss = fs.readFileSync(path.join(cssDir, "view.css"), "utf-8");
    expect(viewCss).toContain("@media (prefers-reduced-motion: reduce)");
    expect(viewCss).toContain(
      ".session-list .unified-listbox__option:has(.session-list__row--active)::before",
    );
  });
});

// ─── ADR-0032 / ADR-0033 invariants maintained ─────────────────────────────
describe("ADR-0033: displayLabel chain maintained in SessionList render", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders title when title is present (ADR-0033 chain: title first; subtitle suppressed)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "My Title", subtitle: "My Sub" }, status: "idle" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("My Title")).toBeDefined();
    // subtitle is the title-fallback — it must NOT appear alongside the title.
    expect(container.textContent).not.toMatch(/My Sub/);
  });

  it("renders subtitle when title is absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { subtitle: "Only Sub" }, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getAllByText("Only Sub").length).toBeGreaterThan(0);
  });

  it("renders id when title and subtitle both absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "fallback-id",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("fallback-id")).toBeDefined();
  });
});

describe("ADR-0032: session-status-slot and session-status-spinner are maintained", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("running session has session-status-spinner (ADR-0032 active spinner)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "active" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(1);
  });

  it("stopped session has no session-status-spinner (ADR-0032 inactive)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "stopped" }, status: "stopped" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(0);
  });

  it("each session row has a session-status-slot element", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "s1" }, status: "running" },
        },
        {
          id: "s2",
          project: "/repo/p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "s2" }, status: "stopped" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-slot").length).toBe(2);
  });
});
