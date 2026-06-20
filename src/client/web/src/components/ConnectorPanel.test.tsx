import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { useConnectorsStore } from "../store/connectors";
import type { ConnectorInfo } from "../wire/server";
import { ConnectorPanel } from "./ConnectorPanel";

function makeConnector(overrides: Partial<ConnectorInfo> = {}): ConnectorInfo {
  return {
    name: "github",
    label: "GitHub",
    summary: "GitHub integration",
    available: true,
    sections: [
      {
        title: "Repositories",
        items: [
          { symbol: "R", title: "my-repo", meta: "main" },
          { symbol: "R", title: "other-repo", meta: "dev" },
        ],
      },
    ],
    ...overrides,
  };
}

describe("ConnectorPanel", () => {
  beforeEach(() => {
    useConnectorsStore.getState().reset();
  });

  it("TestRendersNullWhenEmpty: store empty → renders nothing", () => {
    const { container } = render(<ConnectorPanel />);
    expect(container.firstChild).toBeNull();
  });

  it("TestRendersConnectorWithSections: connector label and sections appear in DOM", () => {
    useConnectorsStore.getState().setConnectors([makeConnector()]);
    render(<ConnectorPanel />);

    expect(screen.getByText(/GitHub/)).toBeTruthy();
    expect(screen.getByText(/GitHub integration/)).toBeTruthy();
    expect(screen.getByText("Repositories")).toBeTruthy();
    expect(screen.getByText("my-repo")).toBeTruthy();
    expect(screen.getByText("other-repo")).toBeTruthy();
    expect(screen.getByText("main")).toBeTruthy();
  });

  it("TestUnavailableMarker: available:false shows (unavailable) marker", () => {
    useConnectorsStore.getState().setConnectors([makeConnector({ available: false })]);
    render(<ConnectorPanel />);
    expect(screen.getByText(/(unavailable)/)).toBeTruthy();
  });

  it("TestUsesDetailsElementForAccordion: <details> element is used for accordion", () => {
    useConnectorsStore.getState().setConnectors([makeConnector()]);
    const { container } = render(<ConnectorPanel />);
    const details = container.querySelector("details");
    expect(details).not.toBeNull();
  });

  it("renders symbol and meta for each item", () => {
    useConnectorsStore.getState().setConnectors([makeConnector()]);
    render(<ConnectorPanel />);
    const symbols = screen.getAllByText("R", { selector: "span" });
    expect(symbols.length).toBeGreaterThan(0);
    expect(screen.getByText("main")).toBeTruthy();
  });

  it("renders multiple connectors", () => {
    useConnectorsStore
      .getState()
      .setConnectors([
        makeConnector({ name: "github", label: "GitHub", summary: "GitHub integration" }),
        makeConnector({ name: "jira", label: "Jira", summary: "Jira integration", sections: [] }),
      ]);
    render(<ConnectorPanel />);
    expect(screen.getByText(/GitHub/)).toBeTruthy();
    expect(screen.getByText(/Jira/)).toBeTruthy();
  });
});
