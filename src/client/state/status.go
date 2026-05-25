package state

import v "github.com/takezoh/agent-roost/client/state/view"

// Status and related types are defined in state/view and re-exported here
// as type aliases so existing state-internal and external code is unchanged.
type Status = v.Status
type StatusInfo = v.StatusInfo

const (
	StatusRunning = v.StatusRunning
	StatusWaiting = v.StatusWaiting
	StatusIdle    = v.StatusIdle
	StatusStopped = v.StatusStopped
	StatusPending = v.StatusPending
)

// ParseStatus is the inverse of Status.String().
func ParseStatus(name string) (Status, bool) { return v.ParseStatus(name) }
