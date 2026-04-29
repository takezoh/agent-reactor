package devcontainer

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
)

// ContainerState holds runtime data for one project's devcontainer.
type ContainerState struct {
	mu          sync.Mutex
	containerID string
	spec        *DevcontainerSpec
	refCount    int
}

// Config carries devcontainer-specific parameters.
type Config struct {
	ExtraCreateArgs []string // extra args for "docker create"
}

// Manager implements sandbox.Manager[*ContainerState] using direct docker commands.
// Roost does not build images; the image name is read from devcontainer.json (image: or build.name).
type Manager struct {
	overlayFn  OverlayFunc
	cfg        Config
	mu         sync.Mutex
	inflight   singleflight.Group
	containers map[string]*ContainerState // key = projectPath
}

// New returns a Manager. overlayFn may be nil.
func New(overlayFn OverlayFunc, cfg Config) *Manager {
	return &Manager{
		overlayFn:  overlayFn,
		cfg:        cfg,
		containers: make(map[string]*ContainerState),
	}
}

// EnsureInstance ensures the devcontainer for projectPath is running.
// Returns an error if the image declared in devcontainer.json does not exist locally.
func (m *Manager) EnsureInstance(ctx context.Context, projectPath, _ string, _ sandbox.StartOptions) (*sandbox.Instance[*ContainerState], error) {
	_, err, _ := m.inflight.Do(projectPath, func() (any, error) {
		return nil, m.ensureContainer(ctx, projectPath)
	})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	cs := m.containers[projectPath]
	m.mu.Unlock()

	return &sandbox.Instance[*ContainerState]{
		ProjectPath: projectPath,
		Image:       "devcontainer",
		Internal:    cs,
	}, nil
}

func (m *Manager) ensureContainer(ctx context.Context, projectPath string) error {
	m.mu.Lock()
	_, exists := m.containers[projectPath]
	m.mu.Unlock()
	if exists {
		return nil
	}

	t := time.Now()
	findCtx, findCancel := context.WithTimeout(ctx, 5*time.Second)
	ctr, err := FindContainer(findCtx, projectPath)
	findCancel()
	slog.Info("devcontainer: stage", "name", "find", "elapsed", time.Since(t), "project", projectPath)
	if err != nil {
		return fmt.Errorf("devcontainer: find container: %w", err)
	}

	dcPath, err := FindDevcontainerPath(projectPath)
	if err != nil {
		return fmt.Errorf("devcontainer: %w", err)
	}

	t = time.Now()
	spec, err := m.loadSpec(projectPath, filepath.Dir(dcPath))
	if err != nil {
		return err
	}
	slog.Info("devcontainer: stage", "name", "load_spec", "image", spec.Image, "elapsed", time.Since(t), "project", projectPath)

	image := spec.Image
	if imgEnv, err := ImageEnv(ctx, image); err != nil {
		slog.Warn("devcontainer: image env probe failed; ${containerEnv:*} not resolved",
			"image", image, "err", err)
	} else {
		spec.ResolveContainerEnvPlaceholders(imgEnv)
	}

	if ctr != nil {
		return m.reuseContainer(ctx, projectPath, ctr, spec)
	}
	return m.createContainer(ctx, projectPath, image, spec)
}

func (m *Manager) loadSpec(projectPath, dcDir string) (*DevcontainerSpec, error) {
	spec, err := LoadSpec(projectPath, dcDir)
	if err != nil {
		return nil, fmt.Errorf("devcontainer: load spec: %w", err)
	}

	if m.overlayFn != nil {
		overlay, err := m.overlayFn(projectPath, dcDir)
		if err != nil {
			slog.Warn("devcontainer: overlay failed, continuing without overlay", "project", projectPath, "err", err)
		} else {
			spec.Apply(overlay)
		}
	}
	return spec, nil
}

func (m *Manager) reuseContainer(ctx context.Context, projectPath string, ctr *ContainerInfo, spec *DevcontainerSpec) error {
	if ctr.State != "running" {
		slog.Info("devcontainer: starting existing container", "id", shortID(ctr.ID), "state", ctr.State, "project", projectPath)
		t := time.Now()
		startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
		err := StartContainer(startCtx, ctr.ID)
		startCancel()
		slog.Info("devcontainer: stage", "name", "start_existing", "elapsed", time.Since(t), "project", projectPath)
		if err != nil {
			return fmt.Errorf("devcontainer: %w", err)
		}
	} else {
		slog.Info("devcontainer: reusing running container", "id", shortID(ctr.ID), "project", projectPath)
	}

	m.mu.Lock()
	m.containers[projectPath] = &ContainerState{containerID: ctr.ID, spec: spec}
	m.mu.Unlock()
	return nil
}

func (m *Manager) createContainer(ctx context.Context, projectPath, image string, spec *DevcontainerSpec) error {
	containerID, err := m.createAndStart(ctx, projectPath, image, spec)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.containers[projectPath] = &ContainerState{containerID: containerID, spec: spec}
	m.mu.Unlock()

	m.runPostCreate(containerID, spec)
	slog.Info("devcontainer: container ready", "id", shortID(containerID), "project", projectPath)
	return nil
}

func (m *Manager) createAndStart(ctx context.Context, projectPath, image string, spec *DevcontainerSpec) (string, error) {
	createArgs := append(spec.BuildCreateArgs(image), m.cfg.ExtraCreateArgs...)
	slog.Info("devcontainer: creating container", "image", image, "project", projectPath)
	t := time.Now()
	createCtx, createCancel := context.WithTimeout(ctx, 30*time.Second)
	containerID, err := CreateContainer(createCtx, createArgs)
	createCancel()
	slog.Info("devcontainer: stage", "name", "create", "elapsed", time.Since(t), "project", projectPath)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}

	slog.Info("devcontainer: starting container", "id", shortID(containerID), "project", projectPath)
	t = time.Now()
	startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
	err = StartContainer(startCtx, containerID)
	startCancel()
	slog.Info("devcontainer: stage", "name", "start_new", "elapsed", time.Since(t), "project", projectPath)
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
	RunPostCreate(pcCtx, containerID, spec.PostCreate)
	for _, argv := range spec.ExtraPostCreate {
		RunPostCreate(pcCtx, containerID, argv)
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

	workDir := spec.workspaceTarget()
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
// Image layers are kept by Docker so the next "docker create" reuses the cache.
func (m *Manager) DestroyInstance(ctx context.Context, inst *sandbox.Instance[*ContainerState]) error {
	cs := inst.Internal
	m.mu.Lock()
	delete(m.containers, inst.ProjectPath)
	m.mu.Unlock()

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
	return filepath.Join(remoteWorkspace, rel)
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

