import { beforeEach, describe, expect, it } from "vitest";
import type { ConnectorInfo } from "../wire/server";
import { useConnectorsStore } from "./connectors";

function makeConnector(overrides: Partial<ConnectorInfo> = {}): ConnectorInfo {
  return {
    name: "github",
    label: "GitHub",
    summary: "GitHub integration",
    available: true,
    sections: [],
    ...overrides,
  };
}

describe("connectorsStore", () => {
  beforeEach(() => {
    useConnectorsStore.getState().reset();
  });

  it("TestSetConnectorsReplacesArray: initial [] → setConnectors → length 1", () => {
    expect(useConnectorsStore.getState().connectors).toHaveLength(0);
    useConnectorsStore.getState().setConnectors([makeConnector({ name: "github" })]);
    expect(useConnectorsStore.getState().connectors).toHaveLength(1);
    expect(useConnectorsStore.getState().connectors[0]?.name).toBe("github");
  });

  it("TestSetConnectorsReplacesArray: replaces existing array entirely", () => {
    useConnectorsStore
      .getState()
      .setConnectors([makeConnector({ name: "github" }), makeConnector({ name: "jira" })]);
    expect(useConnectorsStore.getState().connectors).toHaveLength(2);

    useConnectorsStore.getState().setConnectors([makeConnector({ name: "slack" })]);
    expect(useConnectorsStore.getState().connectors).toHaveLength(1);
    expect(useConnectorsStore.getState().connectors[0]?.name).toBe("slack");
  });

  it("TestReset: reset returns to empty array", () => {
    useConnectorsStore.getState().setConnectors([makeConnector(), makeConnector({ name: "jira" })]);
    expect(useConnectorsStore.getState().connectors).toHaveLength(2);

    useConnectorsStore.getState().reset();
    expect(useConnectorsStore.getState().connectors).toHaveLength(0);
  });
});
