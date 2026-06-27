// Package termvt is a self-contained terminal multiplexer primitive: it runs a
// command in a pty, parses its output through a server-side VT emulator (for
// reattach snapshots and OSC handling), and fans typed events out to any number
// of subscribers. It is shared base infrastructure and depends on neither
// client/ nor orchestrator/.
//
// Wire encoding (asciicast, JSON, …) is intentionally NOT this package's
// concern: it emits typed Event values and the caller (e.g. a web gateway)
// chooses how to serialize them.
package termvt

// EventKind discriminates the terminal stream events a Session emits.
type EventKind int

const (
	// EventOutput carries raw pty bytes for the client terminal to render.
	EventOutput EventKind = iota
	// EventControl carries a structured sequence handled server-side (OSC /
	// title / bell), delivered to the client as data rather than raw bytes.
	EventControl
	// EventExit signals the session's process has exited; no further events
	// follow and the channel is closed.
	EventExit
)

// Control is a structured terminal sequence captured server-side by the OSC
// "tee" (see Session.registerOSC).
type Control struct {
	Kind string `json:"kind"` // "osc" | "prompt" | "title" | "bell"
	Code int    `json:"code,omitempty"`
	Data string `json:"data,omitempty"`
}

// Event is one item in a session's output stream.
type Event struct {
	Kind EventKind
	Data []byte  // set when Kind == EventOutput
	Ctl  Control // set when Kind == EventControl
}
