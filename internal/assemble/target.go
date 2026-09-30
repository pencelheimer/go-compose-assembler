package assemble

import "github.com/pencelheimer/go-compose-assembler/internal/domain"

// TargetMerger handles merging list parameter values into a service.
type TargetMerger interface {
	// Target returns the string identifier this merger handles (e.g., "ports", "volumes", "environment").
	Target() string
	// Merge applies the given value to the service. The value arrives decoded from JSON/YAML as a scalar or slice.
	Merge(svc *domain.Service, value any) error
}

// registry holds registered target mergers keyed by target name.
var registry = make(map[string]TargetMerger)

// RegisterTarget registers a TargetMerger. It panics if a merger is already registered for the same target.
// This is called from init() functions in merger implementation files.
func RegisterTarget(m TargetMerger) {
	key := m.Target()
	if _, exists := registry[key]; exists {
		panic("assembler: duplicate target merger registration for target: " + key)
	}
	registry[key] = m
}

// GetTarget returns the TargetMerger registered for the given target name, or nil if none is registered.
func GetTarget(target string) TargetMerger {
	return registry[target]
}

// ResetRegistry clears the merger registry. Intended for testing only.
func ResetRegistry() {
	registry = make(map[string]TargetMerger)
}
