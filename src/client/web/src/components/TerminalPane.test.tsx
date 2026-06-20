import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TerminalPane } from "./TerminalPane";

const fakeConn = {
  subscribe: async () => {},
  unsubscribe: async () => {},
  send: () => {},
  onOutput: undefined as ((frame: [string, number, string]) => void) | undefined,
} as unknown as import("../socket/connection").Connection;

describe("TerminalPane", () => {
  it("mounts and unmounts without throwing", () => {
    const { unmount, container } = render(<TerminalPane conn={fakeConn} sessionId="s1" />);
    expect(container.querySelector(".terminal-host")).not.toBeNull();
    unmount();
  });
});
