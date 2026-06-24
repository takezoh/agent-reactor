import { useEffect, useRef, useState } from "react";
import type { TranscriptKindParam } from "../api/transcripts";
import { useTranscript } from "../hooks/useTranscript";
import { bufferKey, useTranscriptStore } from "../store/transcripts";
import type { LogTab } from "../wire/server";
import "../css/view.css";

export type LogTabsProps = {
  tabs: LogTab[];
  /** Session ID used for transcript REST + store lookups. Required when tabs
   *  contain transcript/event-log kinds; ts-app-wiring will wire this once
   *  App.tsx is updated. */
  sessionId?: string;
  /** Bearer token forwarded to the transcript REST API. */
  bearerToken?: string;
  fetchFn?: typeof fetch;
  /** When true, suppress content for tabs labelled "INFO" (FR-γ08). */
  suppressInfo?: boolean;
};

/**
 * kindOfTab resolves the TranscriptKindParam for a given LogTab, or null if the
 * tab does not map to a known transcript kind.
 *
 * Detection order: path suffix wins over label, so drivers can set an explicit
 * path extension regardless of label capitalisation.
 *
 * Symmetric with server matchLogTab (src/server/web/transcript.go):
 *   pathSuffixes=[.log, .jsonl] + labelTokens=[events, event-log]
 */
export function kindOfTab(tab: LogTab): TranscriptKindParam | null {
  const p = tab.path.toLowerCase();
  if (p.endsWith(".transcript") || p.endsWith("/transcript")) return "transcript";
  if (p.endsWith(".event-log") || p.endsWith("/event-log")) return "event-log";
  if (p.endsWith(".log") || p.endsWith(".jsonl")) return "event-log";

  const l = tab.label.toLowerCase();
  if (l === "transcript") return "transcript";
  if (l === "event-log") return "event-log";
  if (l.includes("events") || l.includes("event-log")) return "event-log";

  return null;
}

export type ContentAreaProps = {
  sessionId: string;
  kind: TranscriptKindParam;
  bearerToken: string;
  fetchFn?: typeof fetch;
};

export function ContentArea({ sessionId, kind, bearerToken, fetchFn }: ContentAreaProps) {
  useTranscript({ sessionId, kind, bearerToken, fetchFn });

  const buffer = useTranscriptStore((s) => s.buffers[bufferKey(sessionId, kind)]);
  const linesLength = buffer?.lines.length ?? 0;
  const scrollRef = useRef<HTMLDivElement>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: linesLength is the trigger; effect intentionally re-runs when line count changes
  useEffect(() => {
    const el = scrollRef.current;
    if (el) {
      el.scrollTo(0, el.scrollHeight);
    }
  }, [linesLength]);

  return (
    <div className="log-tab-content" role="tabpanel" ref={scrollRef}>
      <pre>{(buffer?.lines ?? []).join("\n")}</pre>
    </div>
  );
}

/** Returns true when this tab's content should be suppressed by suppressInfo. */
export function isSuppressed(tab: LogTab, suppressInfo: boolean): boolean {
  return suppressInfo && tab.label.toUpperCase() === "INFO";
}

export function LogTabs({
  tabs,
  sessionId = "",
  bearerToken = "",
  fetchFn,
  suppressInfo = false,
}: LogTabsProps) {
  const [active, setActive] = useState<number>(0);

  if (tabs.length === 0) {
    return null;
  }

  const activeTab = tabs[active];
  const kind = activeTab ? kindOfTab(activeTab) : null;
  const suppressed = activeTab ? isSuppressed(activeTab, suppressInfo) : false;

  return (
    <div className="log-tab-selector">
      <div className="log-tab-row" role="tablist">
        {tabs.map((t, i) => (
          <button
            key={`${i}-${t.label}`}
            role="tab"
            type="button"
            aria-selected={i === active}
            className={i === active ? "log-tab active" : "log-tab"}
            onClick={() => setActive(i)}
          >
            {t.label}
          </button>
        ))}
      </div>
      {!suppressed && kind !== null && activeTab ? (
        <ContentArea
          sessionId={sessionId}
          kind={kind}
          bearerToken={bearerToken}
          fetchFn={fetchFn}
        />
      ) : (
        <div className="log-tab-content" role="tabpanel" />
      )}
    </div>
  );
}
