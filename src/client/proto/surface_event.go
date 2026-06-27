package proto

// Wire names for Surface 系 events.
const (
	EvtNameSurfaceOutput = "surface-output"
	EvtNamePromptEvent   = "prompt-event"
)

// EvtSurfaceOutput pushes one chunk of pane output encoded as base64
// to subscribed clients. Each message carries exactly one binary chunk
// so the receiver can reconstruct the raw byte stream in order.
type EvtSurfaceOutput struct {
	SessionID string `json:"session_id"`

	// TimeSec is the wall-clock offset in seconds from the start of the
	// recording at which this chunk was captured.
	TimeSec float64 `json:"time_sec"`

	// DataB64 holds the raw terminal output as a plain base64 string.
	// We use an explicit string rather than []byte so that
	// encoding/json does not apply its automatic base64 pass; the
	// decoder is responsible for decoding the value itself, keeping
	// the wire contract simple and explicit.
	DataB64 string `json:"data_b64"`

	// Sequence は subscribe 単位で単調増加するチャンク番号。
	// 同じ SessionID への subscribe を張り直すたびに 0 にリセットされる。
	// 別 subscribe 間で連続性は保証されない。
	// (ADR 0010)
	Sequence uint64 `json:"sequence"`
}

func (EvtSurfaceOutput) isEvent()          {}
func (EvtSurfaceOutput) EventName() string { return EvtNameSurfaceOutput }

// EvtPromptEvent notifies clients of a frame's prompt state transition
// (e.g. start / end of a prompt cycle).
type EvtPromptEvent struct {
	// FrameID identifies the frame whose prompt state changed.
	FrameID string `json:"frame_id"`

	// Phase is a string identifier for the transition, e.g. "start" or "end".
	Phase string `json:"phase"`

	// ExitCode is the exit code of the completed command.
	// Omitted (zero-value omitted) when Phase is "start".
	ExitCode int `json:"exit_code,omitempty"`

	// NowRFC is the wall-clock time of the event in RFC3339 format.
	NowRFC string `json:"now_rfc"`
}

func (EvtPromptEvent) isEvent()          {}
func (EvtPromptEvent) EventName() string { return EvtNamePromptEvent }
