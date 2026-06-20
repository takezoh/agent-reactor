import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useNotificationsStore } from "../store/notifications";
import { NotificationToast } from "./NotificationToast";

describe("NotificationToast", () => {
  beforeEach(() => {
    useNotificationsStore.getState().clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("TestRendersUpTo3Items: shows only the latest 3 of 5 items", () => {
    for (let i = 1; i <= 5; i++) {
      useNotificationsStore.getState().add({ level: "info", message: `msg${i}` });
    }
    render(<NotificationToast />);
    // Latest 3 are msg3, msg4, msg5
    expect(screen.queryByText("msg1")).toBeNull();
    expect(screen.queryByText("msg2")).toBeNull();
    expect(screen.getByText("msg3")).toBeTruthy();
    expect(screen.getByText("msg4")).toBeTruthy();
    expect(screen.getByText("msg5")).toBeTruthy();
  });

  it("TestClickDismisses: clicking a toast removes it from the store", () => {
    useNotificationsStore.getState().add({ level: "info", message: "click-me" });
    render(<NotificationToast />);
    const toast = screen.getByText("click-me");
    fireEvent.click(toast);
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("TestAutoDismissAfter5s: auto-dismisses after 5000ms", () => {
    vi.useFakeTimers();
    useNotificationsStore.getState().add({ level: "info", message: "auto" });
    render(<NotificationToast />);
    expect(screen.getByText("auto")).toBeTruthy();
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("TestAriaLive: container has aria-label and toasts have role=status aria-live=polite", () => {
    useNotificationsStore.getState().add({ level: "info", message: "aria-test" });
    render(<NotificationToast />);
    const container = screen.getByLabelText("notifications");
    expect(container).toBeTruthy();
    const statusEl = screen.getByRole("status");
    expect(statusEl.getAttribute("aria-live")).toBe("polite");
  });
});
