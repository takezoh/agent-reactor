package vt

// PromptPhase classifies an OSC 133 semantic-prompt event.
type PromptPhase int

const (
	PromptPhaseNone     PromptPhase = iota
	PromptPhaseStart                // 133;A — prompt rendering started
	PromptPhaseInput                // 133;B — prompt done, awaiting input
	PromptPhaseCommand              // 133;C — command execution started
	PromptPhaseComplete             // 133;D — command finished
)

// PromptEvent is a single OSC 133 semantic-prompt event captured from the
// terminal stream.
type PromptEvent struct {
	Phase    PromptPhase
	ExitCode *int // non-nil only for PromptPhaseComplete (133;D;<exit-code>)
}

// OscNotification is a desktop-notification request captured from an OSC
// 9 / 99 / 777 escape sequence emitted by an agent process.
type OscNotification struct {
	Cmd     int    // 9, 99, or 777
	Payload string // raw payload (leading ';' stripped)
}
