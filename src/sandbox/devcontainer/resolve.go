package devcontainer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ResolveImage resolves the roost image and devcontainer config dir for projectPath.
// Priority: roost-proj-<hash> (project-scope) → roost-user (user-scope).
// Returns an error if neither image exists.
func ResolveImage(ctx context.Context, projectPath string) (image, dcDir string, err error) {
	hash := projectHash(projectPath)

	projImage := ProjectScopeImage(hash)
	ok, err := ImageExists(ctx, projImage)
	if err != nil {
		return "", "", fmt.Errorf("check project image: %w", err)
	}
	if ok {
		return projImage, filepath.Join(projectPath, devcontainerSubdir), nil
	}

	userImage := UserScopeImage()
	ok, err = ImageExists(ctx, userImage)
	if err != nil {
		return "", "", fmt.Errorf("check user image: %w", err)
	}
	if ok {
		home, _ := os.UserHomeDir()
		return userImage, filepath.Join(home, devcontainerSubdir), nil
	}

	return "", "", fmt.Errorf("no roost image found for %s; run 'roost build %s' or 'roost build --user' first", projectPath, projectPath)
}
