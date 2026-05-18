package devcontainer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
)

// SharedContainerKey is the containers map key used for shared-mode instances.
// Overlay functions compare against this to detect shared vs project context.
const SharedContainerKey = "__shared__"

// ContainerState holds runtime data for one project's devcontainer.
type ContainerState struct {
	mu          sync.Mutex
	containerID string
	spec        *DevcontainerSpec
	refCount    int
}

// WorkspaceFolder returns the container-absolute workspace path from the spec,
// or "" if the spec is not available. spec is immutable post-construction.
func (cs *ContainerState) WorkspaceFolder() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.WorkspaceFolder
}

// WorkspaceTarget returns the effective container-side workspace path, falling
// back to /workspaces/<basename> when workspaceFolder is absent from devcontainer.json.
// Use this for pathmap registration so the mount covers the actual container cwd.
func (cs *ContainerState) WorkspaceTarget() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.WorkspaceTarget()
}

func (cs *ContainerState) BindMounts() []BindMount {
	if cs == nil || cs.spec == nil {
		return nil
	}
	return cs.spec.BindMounts()
}

// IsShared reports whether this container state belongs to the shared container.
func (cs *ContainerState) IsShared() bool {
	return cs != nil && cs.spec != nil && cs.spec.Isolation == IsolationShared
}

func (cs *ContainerState) ContainerID() string {
	if cs == nil {
		return ""
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.containerID
}

// PreExec returns the devcontainer.json `preExecCommand` (or the spec
// fallback). Callers that build their own docker exec command (e.g. the
// codex backend bypassing BuildLaunchCommand) need this to wrap their
// command with the same `bash -lc 'preExec; exec ...'` envelope the pane
// uses, otherwise binaries that depend on the shell init (mise shims,
// tool-version managers, env loaders) won't see the expected setup.
func (cs *ContainerState) PreExec() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.PreExec
}

func (cs *ContainerState) EffectiveUser() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.spec.EffectiveUser()
}

// Manager implements sandbox.Manager[*ContainerState] using direct docker commands.
// Roost does not build images; the image name is read from devcontainer.json (image: or build.name).
type Manager struct {
	overlayFn  OverlayFunc
	mu         sync.Mutex
	inflight   singleflight.Group
	containers map[string]*ContainerState // key = projectPath
}

// New returns a Manager. overlayFn may be nil.
func New(overlayFn OverlayFunc) *Manager {
	return &Manager{
		overlayFn:  overlayFn,
		containers: make(map[string]*ContainerState),
	}
}

// EnsureInstance ensures the devcontainer for projectPath is running.
// When opts.SharedMode is true, a single shared container is used across projects.
// Returns an error if the image declared in devcontainer.json does not exist locally.
func (m *Manager) EnsureInstance(ctx context.Context, projectPath, _ string, opts sandbox.StartOptions) (*sandbox.Instance[*ContainerState], error) {
	instanceKey := projectPath
	if opts.SharedMode {
		instanceKey = SharedContainerKey
	}
	_, err, _ := m.inflight.Do(instanceKey, func() (any, error) {
		return nil, m.ensureContainer(ctx, instanceKey, projectPath, opts)
	})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	cs := m.containers[instanceKey]
	m.mu.Unlock()

	image := ""
	if cs != nil && cs.spec != nil {
		image = cs.spec.Image
	}
	return &sandbox.Instance[*ContainerState]{
		ProjectPath: projectPath,
		Image:       image,
		Internal:    cs,
	}, nil
}

func (m *Manager) ensureContainer(ctx context.Context, instanceKey, projectPath string, opts sandbox.StartOptions) error {
	m.mu.Lock()
	_, exists := m.containers[instanceKey]
	m.mu.Unlock()
	if exists {
		return nil
	}

	var ctr *ContainerInfo
	var dcPath string
	var err error

	t := time.Now()
	findCtx, findCancel := context.WithTimeout(ctx, 5*time.Second)
	if opts.SharedMode {
		ctr, err = FindSharedContainer(findCtx)
	} else {
		ctr, err = FindContainer(findCtx, projectPath)
	}
	findCancel()
	slog.Info("devcontainer: stage", "name", "find", "elapsed", time.Since(t), "project", projectPath, "shared", opts.SharedMode)
	if err != nil {
		return fmt.Errorf("devcontainer: find container: %w", err)
	}

	if opts.SharedMode {
		if opts.DevcontainerDir != "" {
			p := filepath.Join(opts.DevcontainerDir, "devcontainer.json")
			if _, statErr := os.Stat(p); statErr != nil {
				return fmt.Errorf("devcontainer: shared devcontainer path %q: devcontainer.json not found", opts.DevcontainerDir)
			}
			dcPath = p
		} else {
			dcPath, err = UserBaseDC()
			if err != nil {
				return fmt.Errorf("devcontainer: %w", err)
			}
		}
	} else {
		dcPath, err = FindDevcontainerPath(projectPath, opts.DevcontainerDir)
		if err != nil {
			return fmt.Errorf("devcontainer: %w", err)
		}
	}

	t = time.Now()
	spec, err := m.loadSpec(instanceKey, projectPath, filepath.Dir(dcPath))
	if err != nil {
		return err
	}
	if opts.SharedMode {
		spec.Isolation = IsolationShared
	}
	slog.Info("devcontainer: stage", "name", "load_spec", "image", spec.Image, "elapsed", time.Since(t), "project", projectPath)

	image := spec.Image
	imgEnv, err := ImageEnv(ctx, image)
	if err != nil {
		slog.Warn("devcontainer: image env probe failed; resolving without image baseline",
			"image", image, "err", err)
		imgEnv = map[string]string{}
	}
	spec.ResolveContainerEnvPlaceholders(imgEnv)

	if ctr != nil && opts.SharedMode {
		expected := spec.ExtraWorkspacesHash()
		if ctr.MountHash != expected {
			slog.Info("devcontainer: mount mismatch, recreating shared container",
				"id", shortID(ctr.ID), "old", ctr.MountHash, "new", expected)
			rmCtx, rmCancel := context.WithTimeout(ctx, 30*time.Second)
			rmErr := RemoveContainer(rmCtx, ctr.ID)
			rmCancel()
			if rmErr != nil {
				return fmt.Errorf("devcontainer: remove stale shared container: %w", rmErr)
			}
			ctr = nil
		}
	}

	if ctr != nil {
		err := m.reuseContainer(ctx, instanceKey, ctr, spec)
		if err == nil {
			return nil
		}
		// Docker Desktop's WSL bind-mount cache occasionally drops the
		// "/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/.../<hash>"
		// entry for file mounts (notably ~/.claude.json) once the container
		// has been stopped. The container itself is healthy but `docker start`
		// fails the OCI mount step with "no such file or directory". When we
		// see that exact failure, removing and recreating the container is
		// safe: image layers and the mount-hash label are preserved, and the
		// only state we lose is what already lived in the now-broken container
		// layer. Host bind-mounts (~/.claude/sessions, ~/.codex/sessions)
		// are unaffected, so claude / codex session resume still works.
		if !isStaleBindMountError(err) {
			return err
		}
		slog.Warn("devcontainer: stale bind-mount cache, recreating container",
			"id", shortID(ctr.ID), "key", instanceKey)
		rmCtx, rmCancel := context.WithTimeout(ctx, 30*time.Second)
		rmErr := RemoveContainer(rmCtx, ctr.ID)
		rmCancel()
		if rmErr != nil {
			return fmt.Errorf("devcontainer: recover after stale bind-mount: %w (original: %v)", rmErr, err)
		}
		m.mu.Lock()
		delete(m.containers, instanceKey)
		m.mu.Unlock()
	}
	return m.createContainer(ctx, instanceKey, image, spec)
}

// isStaleBindMountError detects the specific Docker Desktop failure mode where
// the WSL bind-mount cache has lost a file mount's source path. We only retry
// on this exact pattern so unrelated start failures (image missing, permission
// problems, network) still surface to the user untouched.
func isStaleBindMountError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "OCI runtime create failed") &&
		strings.Contains(msg, "error mounting") &&
		strings.Contains(msg, "no such file or directory")
}

func (m *Manager) loadSpec(instanceKey, projectPath, dcDir string) (*DevcontainerSpec, error) {
	spec, err := LoadSpec(projectPath, dcDir)
	if err != nil {
		return nil, fmt.Errorf("devcontainer: load spec: %w", err)
	}

	if m.overlayFn != nil {
		overlay, err := m.overlayFn(instanceKey, projectPath, dcDir)
		if err != nil {
			return nil, fmt.Errorf("devcontainer: overlay: %w", err)
		}
		spec.Apply(overlay)
	}
	return spec, nil
}

func (m *Manager) reuseContainer(ctx context.Context, instanceKey string, ctr *ContainerInfo, spec *DevcontainerSpec) error {
	if ctr.State != "running" {
		slog.Info("devcontainer: starting existing container", "id", shortID(ctr.ID), "state", ctr.State, "key", instanceKey)
		t := time.Now()
		startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
		err := StartContainer(startCtx, ctr.ID)
		startCancel()
		slog.Info("devcontainer: stage", "name", "start_existing", "elapsed", time.Since(t), "key", instanceKey)
		if err != nil {
			slog.Error("devcontainer: container start failed, manual recovery required",
				"id", shortID(ctr.ID), "key", instanceKey,
				"hint", "if Docker Desktop bind-mount cache is stale, run `docker rm -f "+ctr.ID+"` and restart roost",
				"err", err)
			return fmt.Errorf("devcontainer: %w", err)
		}
	} else {
		slog.Info("devcontainer: reusing running container", "id", shortID(ctr.ID), "key", instanceKey)
	}

	m.mu.Lock()
	m.containers[instanceKey] = &ContainerState{containerID: ctr.ID, spec: spec}
	m.mu.Unlock()
	return nil
}

func (m *Manager) createContainer(ctx context.Context, instanceKey, image string, spec *DevcontainerSpec) error {
	containerID, err := m.createAndStart(ctx, instanceKey, image, spec)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.containers[instanceKey] = &ContainerState{containerID: containerID, spec: spec}
	m.mu.Unlock()

	m.runPostCreate(containerID, spec)
	slog.Info("devcontainer: container ready", "id", shortID(containerID), "key", instanceKey)
	return nil
}

func (m *Manager) createAndStart(ctx context.Context, instanceKey, image string, spec *DevcontainerSpec) (string, error) {
	createArgs := spec.BuildCreateArgs(image)
	slog.Info("devcontainer: creating container", "image", image, "key", instanceKey)
	t := time.Now()
	createCtx, createCancel := context.WithTimeout(ctx, 30*time.Second)
	containerID, err := CreateContainer(createCtx, createArgs)
	createCancel()
	slog.Info("devcontainer: stage", "name", "create", "elapsed", time.Since(t), "key", instanceKey)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}

	slog.Info("devcontainer: starting container", "id", shortID(containerID), "key", instanceKey)
	t = time.Now()
	startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
	err = StartContainer(startCtx, containerID)
	startCancel()
	slog.Info("devcontainer: stage", "name", "start_new", "elapsed", time.Since(t), "key", instanceKey)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}
	return containerID, nil
}

func (m *Manager) runPostCreate(containerID string, spec *DevcontainerSpec) {
	if len(spec.PostCreate) == 0 && len(spec.ExtraPostCreate) == 0 {
		return
	}
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := waitForContainer(readyCtx, containerID); err != nil {
		slog.Warn("devcontainer: container not ready for exec, skipping postCreate", "err", err)
		return
	}
	pcCtx, pcCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pcCancel()
	user := spec.EffectiveUser()
	RunPostCreate(pcCtx, containerID, user, spec.PostCreate)
	for _, argv := range spec.ExtraPostCreate {
		RunPostCreate(pcCtx, containerID, user, argv)
	}
}

// BuildLaunchCommand generates a "docker exec" command to run plan inside inst.
// frameCtx carries per-frame values (workDir, env) the launcher resolved at
// launch time; in shared mode this is the only path by which per-frame state
// reaches docker exec (the container-scoped spec stays user-scope only).
func (m *Manager) BuildLaunchCommand(inst *sandbox.Instance[*ContainerState], plan state.LaunchPlan, frameCtx sandbox.FrameContext, env map[string]string) (string, map[string]string, error) {
	cs := inst.Internal
	if cs == nil {
		return "", nil, fmt.Errorf("devcontainer: nil ContainerState for %s", inst.ProjectPath)
	}

	cs.mu.Lock()
	containerID := cs.containerID
	spec := cs.spec
	cs.mu.Unlock()

	workDir := resolveWorkDir(spec, frameCtx.WorkDir, plan.StartDir, inst.ProjectPath)

	command := plan.Command
	if command == "shell" {
		command = "sh -c " + shellEscape(`exec "$(getent passwd "$(id -un)" | cut -d: -f7)" -l`)
	}
	if spec.PreExec != "" {
		command = "bash -lc " + shellEscape(spec.PreExec+"; exec "+command)
	}

	var sb strings.Builder
	sb.WriteString("docker exec -it")
	if u := spec.EffectiveUser(); u != "" {
		sb.WriteString(" -u ")
		sb.WriteString(shellEscape(u))
	}
	sb.WriteString(" -w ")
	sb.WriteString(shellEscape(workDir))
	// docker exec -e accepts repeated KEY=VAL; the last occurrence wins.
	// Emit in priority order: spec (container default) → frameCtx (per-frame
	// credential) → env (driver-specific token). Keys are sorted within each
	// source to keep test output deterministic.
	writeEnv := func(m map[string]string) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(" -e ")
			sb.WriteString(shellEscape(k + "=" + m[k]))
		}
	}
	writeEnv(spec.RemoteEnv)
	writeEnv(frameCtx.Env)
	writeEnv(env)
	sb.WriteString(" ")
	sb.WriteString(containerID)
	sb.WriteString(" ")
	sb.WriteString(command)

	return sb.String(), map[string]string{}, nil
}

// AcquireFrame increments the ref-count for the instance.
func (m *Manager) AcquireFrame(inst *sandbox.Instance[*ContainerState]) {
	cs := inst.Internal
	cs.mu.Lock()
	cs.refCount++
	cs.mu.Unlock()
}

// ReleaseFrame decrements the ref-count. Returns true when count drops to zero.
func (m *Manager) ReleaseFrame(inst *sandbox.Instance[*ContainerState]) bool {
	cs := inst.Internal
	cs.mu.Lock()
	cs.refCount--
	zero := cs.refCount <= 0
	cs.mu.Unlock()
	return zero
}

// DestroyInstance handles end-of-life for an instance.
// Project-mode containers are removed (docker rm). Shared containers are
// stopped (docker stop) but not removed, so a later EnsureInstance restarts
// the same container without losing image layer cache or the spec's mount set.
func (m *Manager) DestroyInstance(ctx context.Context, inst *sandbox.Instance[*ContainerState]) error {
	cs := inst.Internal

	instanceKey := inst.ProjectPath
	if cs.IsShared() {
		instanceKey = SharedContainerKey
	}

	m.mu.Lock()
	delete(m.containers, instanceKey)
	m.mu.Unlock()

	cs.mu.Lock()
	id := cs.containerID
	cs.mu.Unlock()
	if id == "" {
		return nil
	}

	if cs.IsShared() {
		slog.Info("devcontainer: stopping shared container", "id", shortID(id))
		stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		return StopContainer(stopCtx, id)
	}

	slog.Info("devcontainer: removing container", "id", shortID(id), "project", inst.ProjectPath)
	rmCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return RemoveContainer(rmCtx, id)
}

// resolveWorkDir returns the container-side working directory for a launch.
// Shared containers have no canonical project root, so the caller is expected
// to have already translated plan.StartDir to a container path via pathmap;
// it is used as-is. Project containers fall back to translateWorkDir, which
// maps a host path under projectPath into the container workspace.
// resolveWorkDir returns the container-side cwd for docker exec.
// Priority: frameCtx WorkDir (already container-side, resolved via pathmap by
// the launcher) → shared-mode plan.StartDir → translateWorkDir (project mode).
func resolveWorkDir(spec *DevcontainerSpec, ctxWorkDir, planStartDir, projectPath string) string {
	if ctxWorkDir != "" {
		return ctxWorkDir
	}
	if spec.Isolation == IsolationShared {
		if planStartDir != "" {
			return planStartDir
		}
		return spec.WorkspaceTarget()
	}
	return translateWorkDir(planStartDir, projectPath, spec.WorkspaceTarget())
}

// translateWorkDir maps a host-side launch directory to its container-side
// equivalent under remoteWorkspace. Paths outside projectPath fall back to remoteWorkspace.
func translateWorkDir(hostStartDir, projectPath, remoteWorkspace string) string {
	if hostStartDir == "" || hostStartDir == projectPath {
		return remoteWorkspace
	}
	rel, err := filepath.Rel(projectPath, hostStartDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return remoteWorkspace
	}
	return path.Join(remoteWorkspace, rel)
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
