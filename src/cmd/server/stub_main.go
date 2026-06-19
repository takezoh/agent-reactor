//go:build !legacy_session

// Package main is a build stub for the server command. The production
// implementation (main.go) requires the legacy_session build tag until the
// session service is migrated to the daemon-client model (see mux.go task).
package main

func main() {}
