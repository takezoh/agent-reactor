// TODO(plan A1-δ open question): proto ConnectorItem に href がない。href 対応は別 PR。
import type { JSX } from "react";
import { useConnectorsStore } from "../store/connectors";
import type { ConnectorInfo, ConnectorItem, ConnectorSection } from "../wire/server";

function renderItem(item: ConnectorItem, idx: number): JSX.Element {
  return (
    <li key={idx} style={{ display: "flex", gap: "0.5rem", alignItems: "baseline" }}>
      <span>{item.symbol}</span>
      <span>{item.title}</span>
      <em>{item.meta}</em>
    </li>
  );
}

function renderSection(section: ConnectorSection, idx: number): JSX.Element {
  return (
    <div key={idx}>
      <h4 style={{ margin: "0.25rem 0" }}>{section.title}</h4>
      <ul style={{ margin: 0, paddingLeft: "1.25rem" }}>
        {(section.items ?? []).map((item, i) => renderItem(item, i))}
      </ul>
    </div>
  );
}

function renderConnector(connector: ConnectorInfo, idx: number): JSX.Element {
  const availabilityNote = connector.available ? "" : " (unavailable)";
  return (
    <details key={idx} style={{ marginBottom: "0.5rem" }}>
      <summary>
        {connector.label} — {connector.summary}
        {availabilityNote}
      </summary>
      <div style={{ paddingLeft: "1rem" }}>
        {(connector.sections ?? []).map((section, i) => renderSection(section, i))}
      </div>
    </details>
  );
}

export function ConnectorPanel(): JSX.Element | null {
  const connectors = useConnectorsStore((s) => s.connectors);

  if (connectors.length === 0) {
    return null;
  }

  return (
    <aside className="connector-panel" aria-label="connectors" style={{ padding: "0.5rem" }}>
      {connectors.map((connector, i) => renderConnector(connector, i))}
    </aside>
  );
}
