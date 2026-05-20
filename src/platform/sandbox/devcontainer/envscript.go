package devcontainer

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/platform/config"
)

// RunEnvScript runs scriptPath with projectPath as its first argument and
// returns the parsed KEY=VALUE pairs. When allow is false the script is skipped
// and a warning is logged.
func RunEnvScript(ctx context.Context, scriptPath, projectPath string, allow bool) map[string]string {
	if scriptPath == "" {
		return nil
	}
	scriptPath = config.ExpandPath(scriptPath)
	if !allow {
		slog.Warn("devcontainer: env_script skipped (not in allow_project_env_script)",
			"script", scriptPath, "project", projectPath)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, scriptPath, projectPath).Output()
	if err != nil {
		slog.Warn("devcontainer: env_script failed", "script", scriptPath, "project", projectPath, "err", err)
		return nil
	}
	env, parseErr := ParseDotenv(strings.NewReader(string(out)))
	if parseErr != nil {
		slog.Warn("devcontainer: env_script output parse error", "script", scriptPath, "err", parseErr)
	}
	return env
}

// ParseDotenv parses KEY=VALUE lines from r.
// Lines starting with # are ignored. Quoted values (single or double) are unquoted.
func ParseDotenv(r io.Reader) (map[string]string, error) {
	env := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = unquote(val)
		env[key] = val
	}
	return env, scanner.Err()
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
