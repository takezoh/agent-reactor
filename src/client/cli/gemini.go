package cli

import "github.com/takezoh/agent-roost/platform/lib/gemini"

func init() {
	Register("gemini", "Gemini CLI integration (setup)", gemini.Run)
}
