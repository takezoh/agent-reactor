// Package socketpath は server daemon が listen する Unix socket path を
// 一意に解決するヘルパを提供する。
//
// 優先順位:
//  1. 明示的に渡された flag 値(空文字でなければ採用)
//  2. envName で指定された環境変数(空でなければ採用)
//  3. fallbackBasename を $HOME/.agent-reactor/<fallback> に展開
//
// どれも解決できなければ、最後の手段として "/tmp/<fallback>" を返す
// (HOME が取れない CI 環境向けの安全網)。
package socketpath

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveDaemonSocket は server daemon の Unix socket path を解決して返す。
//
//   - flag: コマンドライン等で明示的に渡された path (空文字なら無視)
//   - envName: 参照する環境変数名 (空文字なら env ステップをスキップ)
//   - fallbackBasename: デフォルト使用するファイル名 (例: "server.sock")
func ResolveDaemonSocket(flag, envName, fallbackBasename string) string {
	if v := strings.TrimSpace(flag); v != "" {
		return v
	}
	if envName != "" {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			return v
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".agent-reactor", fallbackBasename)
	}
	return filepath.Join(os.TempDir(), fallbackBasename)
}
