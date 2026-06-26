import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import type { View } from "../wire/server";
import { RunStateBadge } from "./RunStateBadge";
import { TagPill, resolveTagPillStyle } from "./primitives/TagPill";
import "../css/view.css";

export type DriverViewPanelProps = {
  view: View;
};

// Re-export for backwards compatibility (DriverViewPanel.test.tsx imports it
// from this module path).
export { resolveTagPillStyle };

export function DriverViewPanel({ view }: DriverViewPanelProps) {
  const now = useNow1Hz();
  const card = view.card;
  const elapsed = view.status_changed_at
    ? formatElapsed(now - new Date(view.status_changed_at).getTime())
    : "";
  return (
    <section className="driver-view-panel" aria-label="driver view">
      <header className="driver-view-header">
        <div className="driver-view-titles">
          {card.title && <h2 className="driver-view-title">{card.title}</h2>}
          {card.subtitle && <p className="driver-view-subtitle">{card.subtitle}</p>}
        </div>
        <RunStateBadge status={view.status} />
      </header>
      {card.tags && card.tags.length > 0 && (
        <div className="driver-view-tags">
          {card.tags.map((t, i) => (
            <TagPill key={`${i}-${t.text}`} tag={t} />
          ))}
        </div>
      )}
      {(card.border_title?.text || card.border_title_secondary?.text || card.border_badge) && (
        <div className="driver-view-border">
          {card.border_title?.text && (
            <span className="border-title">{card.border_title.text}</span>
          )}
          {card.border_title_secondary?.text && (
            <span className="border-title-secondary">{card.border_title_secondary.text}</span>
          )}
          {card.border_badge && <span className="border-badge">{card.border_badge}</span>}
        </div>
      )}
      {view.status_line && (
        <footer className="driver-view-footer">
          <span className="status-line">{view.status_line}</span>
          {elapsed && (
            <span className="status-elapsed" aria-label="elapsed">
              {elapsed}
            </span>
          )}
        </footer>
      )}
    </section>
  );
}
