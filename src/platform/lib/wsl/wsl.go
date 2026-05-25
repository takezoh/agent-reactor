// Package wsl provides helpers for detecting and working with WSL2 (Windows
// Subsystem for Linux). All functions return safe defaults on non-Linux hosts.
package wsl

import "os"

// IsWSL reports whether the current process is running inside WSL2.
func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}
