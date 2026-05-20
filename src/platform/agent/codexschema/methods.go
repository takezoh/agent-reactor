// Package codexschema provides pinned JSON Schema and method-name constants for the
// Codex app-server stdio JSON-RPC protocol (codex-cli 0.128.0).
//
// Generated Go types live in the v1/ and v2/ sub-packages.
// The raw schema bundles used as the drift-detection reference are in schema/.
package codexschema

// Client → server requests (expect a response).
const (
	MethodInitialize   = "initialize"
	MethodThreadResume = "thread/resume"
)

// Client → server notifications (no response expected).
const (
	MethodInitialized = "initialized"
	MethodTurnStart   = "turn/start"
)

// Server → client notifications.
const (
	MethodThreadStarted         = "thread/started"
	MethodTurnStarted           = "turn/started"
	MethodTurnCompleted         = "turn/completed"
	MethodTurnPlanUpdated       = "turn/plan/updated"
	MethodTurnDiffUpdated       = "turn/diff/updated"
	MethodItemStarted           = "item/started"
	MethodItemCompleted         = "item/completed"
	MethodThreadStatusChanged   = "thread/status/changed"
	MethodItemAgentMessageDelta = "item/agentMessage/delta"
	MethodError                 = "error"
	MethodWarning               = "warning"
	MethodGuardianWarning       = "guardianWarning"
	MethodDeprecationNotice     = "deprecationNotice"
)

// Server → client requests (expect a reply from client).
const (
	MethodItemCommandExecutionRequestApproval = "item/commandExecution/requestApproval"
	MethodItemFileChangeRequestApproval       = "item/fileChange/requestApproval"
)

// Approval reply values.
const (
	ApprovalAccept           = "accept"
	ApprovalAcceptForSession = "acceptForSession"
)
