// Package codexschema provides pinned JSON Schema and method-name constants for the
// Codex app-server stdio JSON-RPC protocol (codex-cli 0.133.0).
//
// Generated Go types live in the v1/ and v2/ sub-packages.
// The raw schema bundles used as the drift-detection reference are in schema/.
package codexschema

// Client → server requests (expect a response).
const (
	MethodInitialize   = "initialize"
	MethodThreadStart  = "thread/start"
	MethodThreadResume = "thread/resume"
)

// Client → server notifications (no response expected).
const (
	MethodInitialized = "initialized"
	MethodTurnStart   = "turn/start"
)

// Server → client notifications.
const (
	MethodThreadStarted            = "thread/started"
	MethodTurnStarted              = "turn/started"
	MethodTurnCompleted            = "turn/completed"
	MethodTurnPlanUpdated          = "turn/plan/updated"
	MethodTurnDiffUpdated          = "turn/diff/updated"
	MethodItemStarted              = "item/started"
	MethodItemCompleted            = "item/completed"
	MethodThreadStatusChanged      = "thread/status/changed"
	MethodItemAgentMessageDelta    = "item/agentMessage/delta"
	MethodThreadTokenUsageUpdated  = "thread/tokenUsage/updated"
	MethodAccountRateLimitsUpdated = "account/rateLimits/updated"
	MethodError                    = "error"
	MethodWarning                  = "warning"
	MethodGuardianWarning          = "guardianWarning"
	MethodDeprecationNotice        = "deprecationNotice"
)

// Server → client requests (expect a reply from client).
const (
	MethodItemCommandExecutionRequestApproval = "item/commandExecution/requestApproval"
	MethodItemFileChangeRequestApproval       = "item/fileChange/requestApproval"
	// MethodItemToolCall is the ServerRequest the agent sends when the model invokes
	// a client-side tool (SPEC §10.5). The client replies with the tool result.
	MethodItemToolCall = "item/tool/call"
	// MethodItemToolRequestUserInput is the EXPERIMENTAL server → client request
	// the agent sends when the model requires user input (SPEC §10.5).
	// Documented posture: automated orchestration treats this as a hard fail.
	MethodItemToolRequestUserInput = "item/tool/requestUserInput"
)

// Approval reply values.
const (
	ApprovalAccept           = "accept"
	ApprovalAcceptForSession = "acceptForSession"
)
