package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-roost/client/runtime/subsystem/stream"
	"github.com/takezoh/agent-roost/client/state"
)

// resolveStreamSockPaths returns the host-side and container-side sock paths
// for the given session. Each session gets a unique sock file so multiple
// concurrent app-server processes do not collide.
func (r *Runtime) resolveStreamSockPaths(sessionID state.SessionID) (string, string, error) {
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	// All host-mode session sockets share a single directory. The routing
	// sockbridge watches this directory and routes by session ID.
	runDir, err := ensureStreamRunDir(filepath.Join(dataDir, "run", cstream.RunDirName))
	if err != nil {
		return "", "", fmt.Errorf("stream backend: run dir: %w", err)
	}
	sockName := cstream.SockPrefix + string(sessionID) + cstream.SockSuffix
	hostSock := filepath.Join(runDir, sockName)
	containerSock := hostSock
	if launcher(r.cfg).IsContainer(r.anyProject()) {
		// Container sockets use the fixed container run dir so the in-container
		// routing bridge can find them by session ID.
		containerSock = ContainerRunDir + "/" + sockName
	}
	return hostSock, containerSock, nil
}

// anyProject returns any project from the current session set, used only to
// check whether the runtime is configured for container mode.
func (r *Runtime) anyProject() string {
	for _, sess := range r.state.Sessions {
		if sess.Project != "" {
			return sess.Project
		}
	}
	return ""
}

// ensureStreamRunDir creates the stream run directory if it does not exist.
func ensureStreamRunDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
