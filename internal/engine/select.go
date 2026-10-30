package engine

import "strings"

// NewSelected picks the engine implementation.
// harbor / colima / managed / "" (default product path) → Colima stub.
// system / docker / desktop → SystemDocker.
func NewSelected(source string) Engine {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceSystemDocker, "system", "docker", "desktop":
		return NewSystemDocker()
	default:
		return NewColima()
	}
}
