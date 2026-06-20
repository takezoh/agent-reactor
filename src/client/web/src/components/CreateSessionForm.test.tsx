import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { CreateSessionForm } from "./CreateSessionForm";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

describe("CreateSessionForm", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.restoreAllMocks();
  });

  it("posts /api/sessions and selects returned id", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ id: "new" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    render(<CreateSessionForm conn={fakeConn} bearerToken="t" />);
    fireEvent.change(screen.getByPlaceholderText("New session title"), { target: { value: "x" } });
    fireEvent.click(screen.getByText("Create"));
    await waitFor(() => {
      expect(useDaemonStore.getState().activeSessionID).toBe("new");
    });
    expect(fetchSpy).toHaveBeenCalledWith(
      "/api/sessions",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Authorization: "Bearer t" }),
      }),
    );
  });
});
