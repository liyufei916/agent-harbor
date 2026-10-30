package engine

import "context"

// Status is a coarse, UI-safe engine probe result (no secrets in Message).
type Status struct {
	Ready      bool   `json:"ready"`
	Source     string `json:"source,omitempty"`     // harbor-colima | system-docker | ""
	Message    string `json:"message,omitempty"`    // fixed template for UI
	DockerHost string `json:"dockerHost,omitempty"` // unix://… when known
}

// Engine manages a Docker-API-compatible daemon for P2 Runtime (B→C).
// Implementations must fail closed: Ready=false when daemon/image prerequisites are unmet.
type Engine interface {
	// Ensure installs or verifies pinned CLI/Colima under Application Support (idempotent).
	Ensure(ctx context.Context) error
	// Start brings the managed daemon up (no-op if already running and healthy).
	Start(ctx context.Context) error
	// Stop shuts down the managed daemon when Harbor owns it (not system Docker).
	Stop(ctx context.Context) error
	// Health reports whether docker can talk to the chosen daemon.
	Health(ctx context.Context) (Status, error)
}

// Source identifiers for Status.Source / prefs.
const (
	SourceHarborColima = "harbor-colima"
	SourceSystemDocker = "system-docker"
)
