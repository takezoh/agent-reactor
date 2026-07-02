# Workspace Snapshot Strategy

## Context

Game development workspaces can easily reach several hundred GB or multiple TB because source files, binary assets, generated data, engine files, caches, and build artifacts are often colocated. Creating a disposable workspace for each feature branch by doing a normal full copy is too slow and consumes too much storage.

The goal is to define an OS-specific strategy for creating editable, disposable workspaces that behave like snapshots or copy-on-write clones.

Windows is the primary target. Linux and macOS are secondary targets and should be treated as portability strategies rather than the first implementation target.

## Goals

- Create a disposable workspace from a clean base workspace quickly.
- Avoid physically duplicating unchanged data when possible.
- Allow the disposable workspace to be edited normally by existing tools.
- Support large game projects with hundreds of GB or TB-scale assets.
- Keep the base workspace clean and reusable.
- Make deletion of disposable workspaces simple and safe.
- Avoid depending on a specific game engine where possible.
- Prefer native OS/file-system features over custom file-level virtualization.

## Non-Goals

- Do not build a custom source control system.
- Do not replace Git, Plastic SCM / Unity Version Control, Perforce, or other SCM tools.
- Do not attempt cross-OS identical behavior in the first version.
- Do not require all workspaces to include generated caches or build artifacts.
- Do not assume that a filesystem-level snapshot replaces regular backups.

## Common Workspace Model

Use a clean base workspace and create feature workspaces from it.

```text
/workspace-root
  base-main/
  work-feature-a/
  work-feature-b/
  shared/
    engine/
    tools/
    cache/
  trash/
```

The base workspace should be treated as immutable during feature work. Feature workspaces are disposable and can be deleted after the work is merged, abandoned, or recreated.

## Data Classification

Not every directory should be part of the snapshot or clone target.

### Clone / Snapshot Target

- Project source files
- Game assets
- Config files
- Project files
- Plugins that are part of the project
- SCM metadata, only if copying it is verified to be safe for the selected SCM

### Prefer Shared or Regenerated

- Derived data cache
- Intermediate build files
- Saved/editor temporary files
- Local build artifacts
- IDE indexes
- Engine installation
- SDKs and external tools
- Download caches

For Unreal Engine, directories such as `DerivedDataCache`, `Intermediate`, `Saved`, `Binaries`, and `.vs` should usually be excluded from disposable workspace cloning unless there is a specific reason to include them.

## Strategy Priority

1. Windows: ReFS Dev Drive block clone strategy
2. Linux: Btrfs writable subvolume snapshot strategy
3. Linux alternative: ZFS snapshot + clone strategy
4. macOS: APFS clone copy strategy
5. Windows fallback: VHDX differencing disk strategy
6. Generic fallback: SCM-native workspace creation with shared caches

## Windows Strategy: ReFS Dev Drive Block Clone

### Position

This is the primary strategy.

Windows should use a ReFS Dev Drive as the workspace root and rely on block cloning for fast copy-on-write-like duplication of files. The workspace is still represented as a normal directory tree, so existing editors, build tools, launchers, and SCM clients can continue to work with minimal changes.

### Expected Layout

```text
D:\GameWorkspaces
  _base\
    main-clean\
  _work\
    feature-rendering-test\
    feature-networking-test\
  _shared\
    Engine\
    DDC\
    Tools\
  _trash\
```

### Create Workspace

```powershell
$base = "D:\GameWorkspaces\_base\main-clean"
$name = "feature-rendering-test"
$work = "D:\GameWorkspaces\_work\$name"

robocopy $base $work /MIR /MT:32 /XD Intermediate Saved DerivedDataCache Binaries .vs /R:1 /W:1
```

### Delete Workspace

Prefer moving to a trash area first, then deleting after verification.

```powershell
Move-Item "D:\GameWorkspaces\_work\feature-rendering-test" "D:\GameWorkspaces\_trash\feature-rendering-test"
Remove-Item "D:\GameWorkspaces\_trash\feature-rendering-test" -Recurse -Force
```

### Advantages

- Best fit for Windows-first development.
- Normal directory layout.
- Works with existing Windows tools.
- No need to mount virtual disks per workspace.
- Good match for large binary assets when unchanged data can be block-cloned.

### Risks / Constraints

- Requires Windows 11 24H2 or newer for the intended Dev Drive behavior.
- Requires ReFS / Dev Drive.
- Does not provide a Btrfs/ZFS-style first-class folder snapshot UX.
- Behavior may vary depending on the copy API/tool used.
- Cross-volume copies cannot preserve block sharing.
- Antivirus, indexing, and editor background processes may reduce perceived performance.
- If the SCM stores absolute paths or workspace identity metadata, copying the whole workspace may require validation.

### Validation Tasks

- Verify ReFS Dev Drive creation on target developer machines.
- Measure copy time from `base-main` to a new work directory.
- Measure physical disk usage before and after clone.
- Modify large binary files and confirm only changed data grows significantly.
- Verify Plastic SCM / Unity Version Control behavior after copying a workspace.
- Verify Git / Git LFS behavior if applicable.
- Verify Unreal/Unity editor behavior from cloned workspace.
- Confirm delete performance for large cloned workspaces.

## Windows Fallback Strategy: Differencing VHDX

### Position

Use this when stronger isolation is required than a directory-level clone can provide.

A base VHDX can contain a clean workspace. Each feature workspace can be represented as a differencing VHDX. The child disk stores only changes relative to the parent.

### Expected Layout

```text
D:\GameWorkspaceImages
  base-main.vhdx
  feature-rendering-test.diff.vhdx
  feature-networking-test.diff.vhdx
```

### Advantages

- Stronger isolation than normal directories.
- Natural editable snapshot model.
- Easy to discard a child disk.
- Useful for risky experiments, toolchain tests, or CI-like local reproduction.

### Disadvantages

- Workspace is disk-image based, not folder based.
- Requires mount/unmount workflow.
- Drive-letter or mount-point management is needed.
- Parent update workflow is more complex.
- Less convenient for everyday feature work than ReFS directory cloning.

### Recommended Use

Use VHDX differencing only for:

- highly isolated experiments,
- destructive tooling tests,
- local reproduction environments,
- CI worker images,
- cases where ReFS block clone behavior is insufficient.

## Linux Strategy: Btrfs Writable Subvolume Snapshot

### Position

This is the best Linux strategy and the cleanest conceptual model overall.

Btrfs supports writable snapshots of subvolumes. A clean base workspace can be a subvolume, and each feature workspace can be created as a writable snapshot.

### Expected Layout

```text
/mnt/game-workspaces
  base-main
  work-feature-rendering-test
  work-feature-networking-test
  shared
    engine
    ddc
    tools
```

### Create Workspace

```bash
sudo btrfs subvolume snapshot /mnt/game-workspaces/base-main /mnt/game-workspaces/work-feature-rendering-test
```

### Delete Workspace

```bash
sudo btrfs subvolume delete /mnt/game-workspaces/work-feature-rendering-test
```

### Advantages

- Native writable snapshot semantics.
- Very fast workspace creation.
- Clean deletion model.
- Strong fit for disposable feature workspaces.
- Better snapshot UX than Windows ReFS.

### Risks / Constraints

- Requires Linux development environment.
- Requires Btrfs volume setup and operational knowledge.
- Quotas, compression, and mount options need deliberate configuration.
- Developer tooling must support Linux if used as a primary local environment.

### Recommended Use

Use Btrfs for:

- Linux-native developers,
- Linux build machines,
- local or remote Linux workstations,
- large-scale workspace experiments where snapshot UX matters more than Windows-native compatibility.

## Linux Alternative Strategy: ZFS Snapshot + Clone

### Position

Use ZFS when dataset management, integrity, compression, send/receive, or server-side workflows are more important than simple local workstation setup.

### Create Workspace

```bash
sudo zfs snapshot tank/game/base-main@clean
sudo zfs clone tank/game/base-main@clean tank/game/work-feature-rendering-test
```

### Delete Workspace

```bash
sudo zfs destroy tank/game/work-feature-rendering-test
```

### Advantages

- Strong data integrity model.
- Excellent snapshot and clone features.
- Good for shared storage, build servers, and CI infrastructure.
- Useful send/receive workflow for remote replication.

### Disadvantages

- Heavier operational model than Btrfs.
- Less likely to be the default filesystem on developer machines.
- Dataset hierarchy and snapshot lifecycle require discipline.

### Recommended Use

Use ZFS for:

- shared build storage,
- CI workers,
- remote development servers,
- workstation setups where ZFS is already standard.

## macOS Strategy: APFS Clone Copy

### Position

macOS should use APFS clone copy rather than relying on APFS snapshots as the primary developer workflow.

APFS supports copy-on-write file and directory clones. This can make same-volume workspace copies fast and space-efficient for unchanged files.

### Expected Layout

```text
/Volumes/GameWorkspaces
  base-main/
  work-feature-rendering-test/
  work-feature-networking-test/
  shared/
    engine/
    ddc/
    tools/
```

### Create Workspace

```bash
cp -cR /Volumes/GameWorkspaces/base-main /Volumes/GameWorkspaces/work-feature-rendering-test
```

### Delete Workspace

```bash
rm -rf /Volumes/GameWorkspaces/work-feature-rendering-test
```

### Advantages

- Native macOS filesystem support.
- Normal directory layout.
- Good enough for many local macOS development workflows.
- Minimal tooling changes.

### Risks / Constraints

- Less explicit snapshot management than Btrfs/ZFS.
- Snapshot-like behavior is achieved through clone copy, not writable snapshot datasets.
- Same-volume requirement matters for clone efficiency.
- Tool behavior should be verified for large binary assets.

### Recommended Use

Use APFS clone copy for:

- macOS developers,
- content editing workflows on macOS,
- cases where normal directory layout matters more than formal snapshot management.

Do not treat APFS snapshots as the main feature-workspace strategy. They are better suited for backup, restore, and system snapshot workflows.

## SCM Integration Policy

The snapshot strategy must not assume that copying SCM metadata is always safe.

### Git / Git LFS

- Copying `.git` may be acceptable for local clones, but should be validated.
- Git worktree may be better for source-only workflows.
- Large binary assets managed by Git LFS should be tested carefully with clone-copy semantics.

### Plastic SCM / Unity Version Control

- Validate whether copied workspace metadata causes identity, checkout, lock, or path conflicts.
- If unsafe, create the SCM workspace normally and use filesystem cloning only for asset payloads or cacheable data.
- Confirm behavior for pending changes, exclusive locks, branch switching, and update operations.

### Perforce

- Client spec and workspace root must be handled explicitly.
- Do not blindly copy a Perforce workspace without validating client identity and have-list behavior.
- Consider using filesystem snapshots below a stable client root only after testing.

## Implementation Phases

### Phase 1: Windows Prototype

- Create a ReFS Dev Drive on a Windows 11 24H2+ test machine.
- Place a representative game workspace in `_base/main-clean`.
- Exclude generated directories from the clone operation.
- Create disposable workspaces using `robocopy`.
- Measure time, physical disk growth, editor startup, build behavior, and delete behavior.
- Validate SCM safety.

### Phase 2: Windows Tooling

Create a small wrapper script or CLI command:

```text
grid-workspace create <name> --from main-clean
grid-workspace delete <name>
grid-workspace list
grid-workspace refresh-base
```

The first implementation can call `robocopy` directly.

### Phase 3: Linux Prototype

- Create a Btrfs volume on a Linux test machine.
- Convert or copy the base workspace into a Btrfs subvolume.
- Create and delete writable snapshots.
- Compare behavior with the Windows ReFS strategy.
- Evaluate whether Linux should be used for CI/build-worker workspace cloning.

### Phase 4: macOS Prototype

- Create same-volume APFS clone-copy workspaces.
- Measure clone time and physical disk growth.
- Validate editor and SCM behavior.
- Decide whether macOS support should be official or best-effort.

### Phase 5: Unified Abstraction

Define an OS-specific backend interface:

```text
WorkspaceBackend
  create(base, name)
  delete(name)
  list()
  usage(name)
  verify(base, name)
```

Backend mapping:

```text
windows-refs   -> ReFS Dev Drive + robocopy/block clone
windows-vhdx   -> differencing VHDX
linux-btrfs    -> btrfs subvolume snapshot
linux-zfs      -> zfs snapshot + clone
macos-apfs     -> cp -cR clone copy
fallback-copy  -> normal copy or SCM-native workspace creation
```

## Open Questions

- Which SCM is the first target: Plastic SCM / Unity Version Control, Git LFS, or Perforce?
- Should the base workspace include SCM metadata, or should workspaces be initialized after cloning?
- Should generated directories be excluded by default or configured per project?
- Should shared cache directories be symlinked/junctioned into each workspace?
- Should workspace deletion always move to `_trash` first?
- Should physical disk usage be reported per workspace?
- Should Windows ReFS be mandatory for official support?
- Should Linux Btrfs be used for CI/build-worker workspace pooling?

## Recommendation

Start with the Windows ReFS Dev Drive strategy because Windows is the primary target and it preserves a normal directory-based workflow.

Use Linux Btrfs as the reference model for the cleanest snapshot semantics and as a likely build-server/CI strategy.

Use macOS APFS clone copy as a practical secondary strategy, but avoid promising Btrfs/ZFS-like snapshot management on macOS.

Keep VHDX differencing as a Windows fallback for highly isolated experiments rather than the default daily feature-workspace workflow.
