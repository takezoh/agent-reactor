package devcontainer

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLoadSpec_ImageField(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"myproject:dev"}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_BuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile","name":"myproject:dev"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_ImagePrecedenceOverBuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"top:v1","build":{"name":"build:v2"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "top:v1" {
		t.Errorf("Image = %q, want top:v1 (image: takes precedence)", spec.Image)
	}
}

func TestLoadSpec_MissingImage_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"containerEnv":{"FOO":"bar"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}

func TestLoadSpec_BuildWithoutName_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}
