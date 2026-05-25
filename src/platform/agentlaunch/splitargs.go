package agentlaunch

import (
	"fmt"
	"strings"
)

// SplitArgs tokenizes a POSIX-style shell command string into an argv slice.
// Handles single-quoted and double-quoted spans; backslash-escape inside
// double quotes. Intended for simple codex/claude command strings read from
// WORKFLOW.md — not a full POSIX shell lexer.
func SplitArgs(command string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inToken := false

	for i := 0; i < len(command); {
		c := command[i]
		switch {
		case c == '\'' : // single-quoted: literal content until closing '
			i++
			inToken = true
			for i < len(command) && command[i] != '\'' {
				cur.WriteByte(command[i])
				i++
			}
			if i >= len(command) {
				return nil, fmt.Errorf("agentlaunch: unterminated single quote in %q", command)
			}
			i++ // consume closing '
		case c == '"': // double-quoted: backslash-escape inside
			i++
			inToken = true
			for i < len(command) && command[i] != '"' {
				if command[i] == '\\' && i+1 < len(command) {
					i++
					cur.WriteByte(command[i])
				} else {
					cur.WriteByte(command[i])
				}
				i++
			}
			if i >= len(command) {
				return nil, fmt.Errorf("agentlaunch: unterminated double quote in %q", command)
			}
			i++ // consume closing "
		case c == ' ' || c == '\t' || c == '\n': // whitespace: flush token
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
			i++
		default:
			cur.WriteByte(c)
			inToken = true
			i++
		}
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args, nil
}
