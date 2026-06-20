import { useState } from "react";
import type { LogTab } from "../wire/server";
import "../css/view.css";

export type LogTabSelectorProps = {
  tabs: LogTab[];
};

export function LogTabSelector({ tabs }: LogTabSelectorProps) {
  const [active, setActive] = useState<number>(0);
  if (tabs.length === 0) {
    return null;
  }
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
      <div className="log-tab-placeholder" role="tabpanel">
        (log contents coming in δ — path: {tabs[active]?.path})
      </div>
    </div>
  );
}
