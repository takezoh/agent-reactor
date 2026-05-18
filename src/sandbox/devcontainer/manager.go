package devcontainer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
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

	if ctr != nil {
		return m.reuseContainer(ctx, instanceKey, ctr, spec)
	}
	return m.createContainer(ctx, instanceKey, image, spec)
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
func (m *Manager) BuildLaunchCommand(inst *sandbox.Instance[*ContainerState], plan state.LaunchPlan, env map[string]string) (string, map[string]string, error) {
	cs := inst.Internal
	if cs == nil {
		return "", nil, fmt.Errorf("devcontainer: nil ContainerState for %s", inst.ProjectPath)
	}

	cs.mu.Lock()
	containerID := cs.containerID
	spec := cs.spec
	cs.mu.Unlock()

	workDir := spec.WorkspaceTarget()
	workDir = translateWorkDir(plan.StartDir, inst.ProjectPath, workDir)

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
	for k, v := range spec.RemoteEnv {
		sb.WriteString(" -e ")
		sb.WriteString(shellEscape(k + "=" + v))
	}
	for k, v := range env {
		sb.WriteString(" -e ")
		sb.WriteString(shellEscape(k + "=" + v))
	}
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

// DestroyInstance stops and removes the container.
// For shared containers only the in-memory entry is cleared; the docker container
// persists so it can be reused by the next frame or roost restart.
// Image layers are kept by Docker so the next "docker create" reuses the cache.
func (m *Manager) DestroyInstance(ctx context.Context, inst *sandbox.Instance[*ContainerState]) error {
	cs := inst.Internal

	instanceKey := inst.ProjectPath
	if cs.IsShared() {
		instanceKey = SharedContainerKey
	}

	m.mu.Lock()
	delete(m.containers, instanceKey)
	m.mu.Unlock()

	if cs.IsShared() {
		slog.Debug("devcontainer: shared container released (container kept alive)")
		return nil
	}

	cs.mu.Lock()
	id := cs.containerID
	cs.mu.Unlock()

	slog.Info("devcontainer: removing container", "id", shortID(id), "project", inst.ProjectPath)
	rmCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return RemoveContainer(rmCtx, id)
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
