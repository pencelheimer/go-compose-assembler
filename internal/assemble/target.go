package assemble

import (
	"fmt"
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

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

// SwapRegistry empties the registry and returns a function that restores the
// previous contents. Intended for testing only: it keeps the built-in mergers
// registered from init() intact for other tests in the same binary.
func SwapRegistry() (restore func()) {
	saved := registry
	registry = make(map[string]TargetMerger)
	return func() { registry = saved }
}

// stringList converts a decoded list value into strings. Elements must be
// non-empty strings.
func stringList(target string, value any) ([]string, error) {
	var out []string
	switch v := value.(type) {
	case []string:
		out = v
	case []any:
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil, errs.InvalidRequestError{Reason: fmt.Sprintf("%s: list elements must be strings, got %T", target, e)}
			}
			out = append(out, s)
		}
	default:
		return nil, errs.InvalidRequestError{Reason: fmt.Sprintf("%s: expected a list of strings, got %T", target, value)}
	}
	for _, s := range out {
		if strings.TrimSpace(s) == "" {
			return nil, errs.InvalidRequestError{Reason: target + ": empty list element"}
		}
	}
	return out, nil
}

// appendUnique appends items that are not already present, preserving order.
func appendUnique(dst []string, items []string) []string {
	have := make(map[string]bool, len(dst))
	for _, d := range dst {
		have[d] = true
	}
	for _, it := range items {
		if !have[it] {
			dst = append(dst, it)
			have[it] = true
		}
	}
	return dst
}
