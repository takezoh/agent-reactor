// StatusBadge — inline badge for palette status text (FR-024 / FR-025)
//
// Priority:
//   1. ctx === null → 'Unavailable'
//   2. submitting → 'Sending...'
//   3. toolSelect + 0 enabled rows + sessionConfig === null → 'Loading commands...'
//   4. toolSelect + 0 enabled rows + sessionConfig hydrated → 'No commands available'
//   5. otherwise → null (no badge)
//
// ADR-0057: StatusBadge intentionally has NO aria-live or role='status'.
// Announce-worthy transitions (Sending… / Unavailable) must be routed
// through InlineStatus (the single aria-live='polite' slot) so that the
// UA announce order is deterministic. Adding aria-live here would recreate
// the "multiple polite region" problem ADR-0057 was written to prevent.

export interface StatusBadgeProps {
  text: string | null;
  submitting?: boolean;
}

export function StatusBadge({ text, submitting }: StatusBadgeProps): JSX.Element | null {
  if (text === null) return null;
  return (
    <div className="palette-status-badge" data-testid="palette-progress">
      {submitting && (
        <span aria-hidden="true" className="palette-status-badge__spinner">
          ⟳{" "}
        </span>
      )}
      <span>{text}</span>
    </div>
  );
}
