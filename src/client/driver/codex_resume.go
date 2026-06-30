package driver

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-reactor/client/state"
	_ "modernc.org/sqlite"
)

const codexStateDBName = "state_5.sqlite"

func (cs *CodexState) setRolloutPath(path string) {
	path = strings.TrimSpace(path)
	cs.RolloutPath = path
	cs.TranscriptPath = path
}

func (cs CodexState) resolvedRolloutPath() string {
	if rolloutPath := strings.TrimSpace(cs.RolloutPath); rolloutPath != "" {
		return rolloutPath
	}
	return strings.TrimSpace(cs.TranscriptPath)
}

func logCodexResumeSkip(project, threadID, rolloutPath, reason string) {
	slog.Debug("codex: coldstart without resume",
		"project", project,
		"thread", threadID,
		"rollout_path", rolloutPath,
		"reason", reason)
}

func (cs CodexState) coldStartResumePlan() (state.ResumeTarget, string, bool, error) {
	rawThreadID := strings.TrimSpace(cs.ThreadID)
	sessionID := strings.TrimSpace(cs.SessionID)
	rawRolloutPath := strings.TrimSpace(cs.resolvedRolloutPath())
	threadID := ""
	if isAlphanumHyphen(rawThreadID) {
		threadID = rawThreadID
	} else if isAlphanumHyphen(sessionID) {
		threadID = sessionID
	}
	rolloutPath := usableRolloutPath(rawRolloutPath)
	if rolloutPath == "" && threadID != "" {
		rolloutPath = resolveCodexRolloutPath(threadID)
	}
	if threadID == "" && rawRolloutPath == "" {
		return state.ResumeTarget{}, "", false, nil
	}
	if rawThreadID != "" && threadID == "" {
		if rolloutPath == "" {
			return state.ResumeTarget{}, "", false, fmt.Errorf("codex cold-start resume requires a valid thread_id, got %q", rawThreadID)
		}
	}
	if rolloutPath == "" {
		return state.ResumeTarget{}, "", false, fmt.Errorf("codex cold-start resume requires a usable rollout_path for thread_id %q", threadID)
	}
	return state.ResumeTarget{ThreadID: threadID, RolloutPath: rolloutPath}, resolveCodexSessionID(rolloutPath, sessionID), true, nil
}

func resolveCodexRolloutPath(threadID string) string {
	codexHome, err := codexHomeDir()
	if err != nil {
		slog.Debug("codex: rollout path lookup skipped", "thread", threadID, "err", err)
		return ""
	}
	rolloutPath, err := lookupCodexRolloutByThread(codexHome, threadID)
	if err != nil {
		slog.Debug("codex: rollout path lookup skipped",
			"thread", threadID, "codex_home", codexHome, "err", err)
		return ""
	}
	if usable := usableRolloutPath(rolloutPath); usable != "" {
		return usable
	}
	slog.Debug("codex: rollout path lookup skipped",
		"thread", threadID, "rollout_path", rolloutPath, "reason", "unusable_rollout_path")
	return ""
}

func resolveCodexSessionID(rolloutPath, persistedSessionID string) string {
	if rolloutPath == "" || persistedSessionID != "" {
		return persistedSessionID
	}
	codexHome, err := codexHomeDir()
	if err != nil {
		slog.Debug("codex: session id lookup skipped",
			"rollout_path", rolloutPath, "err", err)
		return ""
	}
	sessionID, err := lookupCodexThreadByRollout(codexHome, rolloutPath)
	if err != nil {
		slog.Debug("codex: session id lookup skipped",
			"rollout_path", rolloutPath, "codex_home", codexHome, "err", err)
		return ""
	}
	return sessionID
}

func usableRolloutPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return ""
	}
	return path
}

func codexHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume could not determine CODEX_HOME: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func lookupCodexThreadByRollout(codexHome, rolloutPath string) (string, error) {
	return lookupCodexThreadValue(codexHome, "id", "rollout_path", rolloutPath)
}

func lookupCodexRolloutByThread(codexHome, threadID string) (string, error) {
	return lookupCodexThreadValue(codexHome, "rollout_path", "id", threadID)
}

func lookupCodexThreadValue(codexHome, selectColumn, whereColumn, value string) (string, error) {
	dbPath := filepath.Join(codexHome, codexStateDBName)
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("codex cold-start resume local session source missing %s: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to open local session source %s: %w", dbPath, err)
	}
	defer func() { _ = db.Close() }()
	query := fmt.Sprintf("SELECT %s FROM threads WHERE %s = ? LIMIT 2", selectColumn, whereColumn)
	rows, err := db.Query(query, value)
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to query local session source %s: %w", dbPath, err)
	}
	defer func() { _ = rows.Close() }()
	var matches []string
	for rows.Next() {
		var match string
		if err := rows.Scan(&match); err != nil {
			return "", fmt.Errorf("codex cold-start resume got malformed sqlite row for %s %s: %w", whereColumn, value, err)
		}
		matches = append(matches, strings.TrimSpace(match))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to read local session source %s: %w", dbPath, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("codex cold-start resume local session source has no thread row for %s %s", whereColumn, value)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("codex cold-start resume local session source returned multiple rows for %s %s", whereColumn, value)
	}
	if matches[0] == "" {
		return "", fmt.Errorf("codex cold-start resume local session source returned empty %s for %s %s", selectColumn, whereColumn, value)
	}
	return matches[0], nil
}
