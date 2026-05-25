package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func sendToast(ctx context.Context, psPath, winPath, title, body string) error {
	cmd := exec.CommandContext(ctx, psPath,
		"-NoProfile", "-ExecutionPolicy", "Bypass",
		"-File", winPath,
		"-Title", xmlEscape(title),
		"-Body", xmlEscape(body),
	)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell: %w: %s", err, out)
	}
	return nil
}

func toWindowsPath(ctx context.Context, linuxPath string) (string, error) {
	out, err := exec.CommandContext(ctx, "wslpath", "-w", linuxPath).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// xmlEscape escapes the five XML special characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
