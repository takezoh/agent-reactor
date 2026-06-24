import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { CreateSessionForm } from "./CreateSessionForm";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

// stubFetch routes by URL so tests can answer /api/session-config (dialog
// hydration) and /api/sessions (form submit) with independent fixtures.
function stubFetch(handlers: Record<string, () => Response | Promise<Response>>) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    for (const [key, handler] of Object.entries(handlers)) {
      if (url === key || url.startsWith(key)) return handler();
    }
    throw new Error(`unexpected fetch ${url}`);
  });
}

function sessionConfigResponse(cfg: {
  default_command?: string;
  commands?: string[];
  projects?: string[];
}) {
  return new Response(
    JSON.stringify({
      default_command: cfg.default_command ?? "",
      commands: cfg.commands ?? [],
      projects: cfg.projects ?? [],
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

// Query helpers — we go through placeholder text rather than ARIA role so
// the tests survive happy-dom's partial <dialog> a11y-tree implementation
// (getByRole({hidden:true}) returns the wrapped label, not the input).
function projectInput(): HTMLInputElement {
  return screen.getByPlaceholderText(/Project directory/i) as HTMLInputElement;
}
function commandInput(): HTMLInputElement {
  return screen.getByPlaceholderText(/Command \(filter or type/i) as HTMLInputElement;
}
function queryProjectInput(): HTMLInputElement | null {
  return screen.queryByPlaceholderText(/Project directory/i) as HTMLInputElement | null;
}
function queryCommandInput(): HTMLInputElement | null {
  return screen.queryByPlaceholderText(/Command \(filter or type/i) as HTMLInputElement | null;
}

function openDialog() {
  fireEvent.click(screen.getByRole("button", { name: /new session/i }));
}

async function waitForDialogReady() {
  await waitFor(() => {
    expect(commandInput().disabled).toBe(false);
  });
}

describe("CreateSessionForm", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.restoreAllMocks();
    // fakeConn is module-scoped, so clear its vi.fn() call records between
    // tests to keep "not.toHaveBeenCalled()" assertions hermetic.
    (fakeConn.subscribe as ReturnType<typeof vi.fn>).mockClear();
  });

  it("renders only a trigger button until clicked", () => {
    stubFetch({});
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    expect(screen.getByRole("button", { name: /new session/i })).not.toBeNull();
    expect(queryProjectInput()).toBeNull();
    expect(queryCommandInput()).toBeNull();
  });

  it("opens dialog, hydrates command and projects from /api/session-config, posts the input", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({
          default_command: "claude",
          commands: ["claude", "shell", "npm run dev"],
          projects: ["/home/me/repo-a"],
        }),
      "/api/sessions": () =>
        new Response(JSON.stringify({ id: "new" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);

    openDialog();
    await waitForDialogReady();

    // default_command seeds the command input.
    expect(commandInput().value).toBe("claude");

    fireEvent.change(projectInput(), { target: { value: "/home/me/repo-a" } });
    fireEvent.change(commandInput(), { target: { value: "npm run dev" } });
    fireEvent.click(screen.getByRole("button", { name: /create/i, hidden: true }));

    await waitFor(() => {
      expect(useDaemonStore.getState().activeSessionID).toBe("new");
    });

    const sessionsCall = fetchSpy.mock.calls.find(
      ([url]) => typeof url === "string" && url === "/api/sessions",
    );
    expect(sessionsCall).toBeDefined();
    const init = sessionsCall?.[1] as RequestInit;
    expect(JSON.parse(init.body as string)).toEqual({
      project: "/home/me/repo-a",
      command: "npm run dev",
    });

    // ADR 0030: subscribe ownership lives on TerminalPane's keyed remount.
    // CreateSessionForm must NOT subscribe — doing so would double-subscribe
    // (this form + the new keyed TerminalPane mount that follows selectSession).
    expect(fakeConn.subscribe).not.toHaveBeenCalled();
  });

  it("populates both datalists (projects + commands) so the inputs are filterable", async () => {
    stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({
          default_command: "shell",
          commands: ["shell", "claude", "claude --resume"],
          projects: ["/home/me/repo-a", "/home/me/repo-b"],
        }),
    });
    const { container } = render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();
    await waitForDialogReady();

    // Both inputs wire to a <datalist> via the `list` attribute; resolve
    // each one through that attribute so the assertion is robust to DOM
    // order and label nesting.
    const projectListId = projectInput().getAttribute("list") ?? "";
    const commandListId = commandInput().getAttribute("list") ?? "";
    const projectOptions = container.querySelector(`datalist#${CSS.escape(projectListId)}`);
    const commandOptions = container.querySelector(`datalist#${CSS.escape(commandListId)}`);
    expect(projectOptions).not.toBeNull();
    expect(commandOptions).not.toBeNull();
    const valuesFrom = (el: Element | null) =>
      el
        ? Array.from(el.querySelectorAll("option")).map((o) => (o as HTMLOptionElement).value)
        : [];
    expect(valuesFrom(projectOptions)).toEqual(["/home/me/repo-a", "/home/me/repo-b"]);
    expect(valuesFrom(commandOptions)).toEqual(["shell", "claude", "claude --resume"]);
  });

  it("sends worktree:true and sandbox:'host' when the toggles are on", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({
          default_command: "shell",
          commands: ["shell"],
          projects: ["/home/me/repo-a"],
        }),
      "/api/sessions": () =>
        new Response(JSON.stringify({ id: "new" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);

    openDialog();
    await waitForDialogReady();

    fireEvent.change(projectInput(), { target: { value: "/home/me/repo-a" } });
    fireEvent.change(commandInput(), { target: { value: "shell" } });
    // Aria labels expose the two toggles without coupling to visible copy.
    fireEvent.click(screen.getByLabelText(/Create git worktree/i));
    fireEvent.click(screen.getByLabelText(/Run on host/i));
    fireEvent.click(screen.getByRole("button", { name: /create/i, hidden: true }));

    await waitFor(() => {
      expect(useDaemonStore.getState().activeSessionID).toBe("new");
    });
    const sessionsCall = fetchSpy.mock.calls.find(
      ([url]) => typeof url === "string" && url === "/api/sessions",
    );
    expect(JSON.parse((sessionsCall?.[1] as RequestInit).body as string)).toEqual({
      project: "/home/me/repo-a",
      command: "shell",
      worktree: true,
      sandbox: "host",
    });
  });

  it("omits worktree and sandbox from the body when the toggles are off (legacy wire shape)", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({ default_command: "shell", commands: ["shell"] }),
      "/api/sessions": () =>
        new Response(JSON.stringify({ id: "new" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);

    openDialog();
    await waitForDialogReady();

    fireEvent.change(projectInput(), { target: { value: "/abs/path" } });
    fireEvent.click(screen.getByRole("button", { name: /create/i, hidden: true }));

    await waitFor(() => {
      expect(useDaemonStore.getState().activeSessionID).toBe("new");
    });
    const sessionsCall = fetchSpy.mock.calls.find(
      ([url]) => typeof url === "string" && url === "/api/sessions",
    );
    const parsed = JSON.parse((sessionsCall?.[1] as RequestInit).body as string);
    // Default path must keep the minimal {project,command} shape so the
    // gateway-side default (Worktree.Enabled=false, Sandbox=Auto) is reached.
    expect(parsed).toEqual({ project: "/abs/path", command: "shell" });
    expect(parsed).not.toHaveProperty("worktree");
    expect(parsed).not.toHaveProperty("sandbox");
  });

  it("accepts a custom command not present in the suggestions", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({ default_command: "shell", commands: ["shell"], projects: [] }),
      "/api/sessions": () =>
        new Response(JSON.stringify({ id: "new" }), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();
    await waitForDialogReady();

    fireEvent.change(projectInput(), { target: { value: "/abs/path" } });
    // Custom command — the daemon's RegisterDefaultFactory builds a generic
    // driver from any first-token, so the UI must allow free input.
    fireEvent.change(commandInput(), { target: { value: "./scripts/dev.sh" } });
    fireEvent.click(screen.getByRole("button", { name: /create/i, hidden: true }));

    await waitFor(() => {
      expect(useDaemonStore.getState().activeSessionID).toBe("new");
    });
    const sessionsCall = fetchSpy.mock.calls.find(
      ([url]) => typeof url === "string" && url === "/api/sessions",
    );
    expect(JSON.parse((sessionsCall?.[1] as RequestInit).body as string)).toEqual({
      project: "/abs/path",
      command: "./scripts/dev.sh",
    });
  });

  it("Cancel closes the dialog without posting /api/sessions", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({ default_command: "shell", commands: ["shell"] }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();
    await waitForDialogReady();

    fireEvent.click(screen.getByRole("button", { name: /cancel/i, hidden: true }));

    await waitFor(() => {
      expect(queryCommandInput()).toBeNull();
    });
    expect(
      fetchSpy.mock.calls.some(([url]) => typeof url === "string" && url === "/api/sessions"),
    ).toBe(false);
  });

  it("blocks submit and shows inline error when project is not an absolute path", async () => {
    const fetchSpy = stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({ default_command: "shell", commands: ["shell"] }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();
    await waitForDialogReady();

    fireEvent.change(projectInput(), { target: { value: "myrepo" } });
    // canSubmit gates the button — submit the form directly to exercise the
    // inline error path (not just the disabled button).
    const form = projectInput().closest("form");
    expect(form).not.toBeNull();
    fireEvent.submit(form as HTMLFormElement);

    expect(screen.getByText(/absolute path/i)).not.toBeNull();
    expect(
      fetchSpy.mock.calls.some(([url]) => typeof url === "string" && url === "/api/sessions"),
    ).toBe(false);
  });

  it("surfaces the response body (with request_id) when POST /api/sessions fails", async () => {
    stubFetch({
      "/api/session-config": () =>
        sessionConfigResponse({ default_command: "shell", commands: ["shell"] }),
      "/api/sessions": () =>
        new Response("project must be an absolute path (request_id=abc123)", { status: 400 }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();
    await waitForDialogReady();

    fireEvent.change(projectInput(), { target: { value: "/abs/path" } });
    fireEvent.click(screen.getByRole("button", { name: /create/i, hidden: true }));

    await waitFor(() => {
      expect(screen.getByText(/request_id=abc123/)).not.toBeNull();
    });
  });

  it("surfaces a /api/session-config load error inline", async () => {
    stubFetch({
      "/api/session-config": () =>
        new Response("settings.toml load failed (request_id=xyz)", { status: 500 }),
    });
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    openDialog();

    await waitFor(() => {
      expect(screen.getByText(/session-config:.*settings\.toml load failed/i)).not.toBeNull();
    });
  });
});
