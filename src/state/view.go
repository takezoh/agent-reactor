package state

import (
	v "github.com/takezoh/agent-roost/state/view"
)

// View and related types are defined in state/view and re-exported here
// as type aliases so existing state-internal and external code is unchanged.
type View = v.View
type Card = v.Card
type Tag = v.Tag
type LogTab = v.LogTab
type TabKind = v.TabKind
type InfoLine = v.InfoLine

const TabKindText = v.TabKindText

// HostTag re-exports view.HostTag for callers that only import state.
var HostTag = v.HostTag
