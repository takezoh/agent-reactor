package cli

import "strings"

const (
	sandboxSkipFlag = "--allow-dangerously-skip-permissions"
	autoModeFlag    = "--enable-auto-mode"
)

// SandboxFlags enforces sandbox-required flag adjustments on a command string:
//   - strips --enable-auto-mode (conflicts with bypass-permissions semantics)
//   - appends --allow-dangerously-skip-permissions unless already present
//
// Returns command unchanged when sandboxed is false.
func SandboxFlags(command string, sandboxed bool) string {
	if !sandboxed {
		return command
	}
	command = stripToken(command, autoModeFlag)
	if hasToken(command, sandboxSkipFlag) {
		return command
	}
	return strings.TrimSpace(command) + " " + sandboxSkipFlag
}

// AppServerArgs returns the claude CLI argv for a non-interactive prompt turn.
// --verbose is mandatory: current claude versions reject -p --output-format stream-json without it.
// When resumeSessionID is non-empty, --resume <id> is appended before the prompt.
func AppServerArgs(resumeSessionID, appendSystemPrompt, prompt string) []string {
	args := []string{"-p", "--output-format", "stream-json", "--verbose"}
	if appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", appendSystemPrompt)
	}
	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}
	return append(args, prompt)
}

// hasToken returns true when command contains the exact flag as a whitespace-delimited token.
func hasToken(command, flag string) bool {
	for _, f := range strings.Fields(command) {
		if f == flag {
			return true
		}
	}
	return false
}

// stripToken removes exact flag tokens from command; "--flag=value" form is left intact.
func stripToken(command, flag string) string {
	parts := strings.Fields(command)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != flag {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
