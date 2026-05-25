package view

import (
	"encoding/json"
	"time"
)

// View is the complete TUI payload for one session, produced by its
// Driver.Step. JSON tags allow the proto layer to ship View values
// directly across the wire without a parallel type hierarchy.
type View struct {
	Card            Card       `json:"card"`
	DisplayName     string     `json:"display_name,omitempty"`
	LogTabs         []LogTab   `json:"log_tabs,omitempty"`
	InfoExtras      []InfoLine `json:"info_extras,omitempty"`
	SuppressInfo    bool       `json:"suppress_info,omitempty"`
	StatusLine      string     `json:"status_line,omitempty"`
	Status          Status     `json:"status,omitempty"`
	StatusChangedAt time.Time  `json:"status_changed_at,omitempty"`
}

// Card is the driver-specific portion of the session list card.
type Card struct {
	Title                string   `json:"title,omitempty"`
	Subtitle             string   `json:"subtitle,omitempty"`
	Tags                 []Tag    `json:"tags,omitempty"`
	Indicators           []string `json:"indicators,omitempty"`
	BorderTitle          Tag      `json:"border_title,omitempty"`
	BorderTitleSecondary Tag      `json:"border_title_secondary,omitempty"`
	BorderBadge          string   `json:"border_badge,omitempty"`
}

// Tag is a colored chip rendered in the session card.
type Tag struct {
	Text       string `json:"text"`
	Foreground string `json:"fg,omitempty"`
	Background string `json:"bg,omitempty"`
}

// HostTag returns a tag indicating a session was launched on the host instead
// of its configured sandbox. Sandbox-override sessions expose this in the card.
func HostTag() Tag {
	return Tag{Text: "host", Background: "#7C5CBF", Foreground: "#FFFFFF"}
}

// LogTab declares an additional log tab the driver wants the TUI to display.
type LogTab struct {
	Label       string          `json:"label"`
	Path        string          `json:"path"`
	Kind        TabKind         `json:"kind"`
	RendererCfg json.RawMessage `json:"renderer_cfg,omitempty"`
}

// TabKind selects the renderer the TUI applies to a tab's contents.
type TabKind string

// TabKindText is the built-in plain-text kind.
const TabKindText TabKind = "text"

// InfoLine is one entry in the INFO tab body.
type InfoLine struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
