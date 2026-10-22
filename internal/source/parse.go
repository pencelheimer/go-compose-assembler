package source

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
	"gopkg.in/yaml.v3"
)

// scalarNamePattern validates scalar parameter names: uppercase letters, digits
// and underscores, not starting with a digit.
var scalarNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// interpRefPattern matches ${VAR}, ${VAR:-default}, ${VAR:+alt}, ${VAR:?err}.
// It skips escaped dollar signs ($$). Capture group 1 is the variable name.
var interpRefPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::[?+\-][^}]*)?\}`)

// rawComponent mirrors ComponentDef for strict top-level YAML decoding.
// The service field is decoded separately to allow unknown keys (catch-all).
type rawComponent struct {
	ID          string                    `yaml:"id"`
	Description string                    `yaml:"description"`
	Params      []domain.ComponentParam   `yaml:"params"`
	Service     yaml.Node                 `yaml:"service"`
	Volumes     []domain.ComponentVolume  `yaml:"volumes"`
	Networks    []domain.ComponentNetwork `yaml:"networks"`
}

// rawPreset mirrors Preset for strict top-level YAML decoding.
type rawPreset struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Services    []domain.PresetService `yaml:"services"`
}

// ParseComponent decodes and validates a component definition from raw YAML bytes.
// location is used in error messages; expectedID is the file stem that id must match.
func ParseComponent(data []byte, location string, expectedID string) (*domain.ComponentDef, error) {
	// Strict top-level decoding.
	var raw rawComponent
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, errs.InvalidComponentError{Location: location, Err: fmt.Errorf("YAML decode: %w", err)}
	}

	wrap := func(msg string) error {
		return errs.InvalidComponentError{Location: location, Err: fmt.Errorf("%s", msg)}
	}

	// Validate ID.
	if !ValidIdentifier(raw.ID) {
		return nil, wrap(fmt.Sprintf("id %q does not match identifier pattern", raw.ID))
	}
	if raw.ID != expectedID {
		return nil, wrap(fmt.Sprintf("id %q does not match file stem %q", raw.ID, expectedID))
	}

	// Build the set of declared scalar parameter names for interpolation checking.
	scalarParams := make(map[string]bool)
	paramNames := make(map[string]bool)

	for _, p := range raw.Params {
		// Non-empty name.
		if p.Name == "" {
			return nil, wrap("parameter name must not be empty")
		}
		// Unique name.
		if paramNames[p.Name] {
			return nil, wrap(fmt.Sprintf("duplicate parameter name %q", p.Name))
		}
		paramNames[p.Name] = true

		// Type must be "scalar" or "list".
		if p.Type != "scalar" && p.Type != "list" {
			return nil, wrap(fmt.Sprintf("parameter %q: type must be \"scalar\" or \"list\", got %q", p.Name, p.Type))
		}

		if p.Type == "scalar" {
			// Scalar name must match env-var pattern.
			if !scalarNamePattern.MatchString(p.Name) {
				return nil, wrap(fmt.Sprintf("scalar parameter %q: name must match uppercase env-var pattern", p.Name))
			}
			// Scalar must not declare a target.
			if p.Target != "" {
				return nil, wrap(fmt.Sprintf("scalar parameter %q must not declare a target", p.Name))
			}
			// Required must not have default.
			if p.Required && p.Default != "" {
				return nil, wrap(fmt.Sprintf("required parameter %q must not declare a default", p.Name))
			}
			// Non-required scalar must have default.
			if !p.Required && p.Default == "" {
				return nil, wrap(fmt.Sprintf("non-required scalar parameter %q must declare a default", p.Name))
			}
			scalarParams[p.Name] = true
		}

		if p.Type == "list" {
			// List must have target.
			if p.Target == "" {
				return nil, wrap(fmt.Sprintf("list parameter %q must declare a target", p.Name))
			}
			// List must not have default.
			if p.Default != "" {
				return nil, wrap(fmt.Sprintf("list parameter %q must not declare a default", p.Name))
			}
		}
	}

	// Decode the service template. We do NOT use KnownFields here so that
	// unknown keys land in the Extra catch-all map.
	var svc domain.ServiceTemplate
	if raw.Service.Kind == 0 {
		return nil, wrap("service template is required")
	}
	if err := raw.Service.Decode(&svc); err != nil {
		return nil, errs.InvalidComponentError{Location: location, Err: fmt.Errorf("service decode: %w", err)}
	}

	// Service image must be non-empty.
	if svc.Image == "" {
		return nil, wrap("service image must not be empty")
	}

	// Collect all interpolation references from typed service fields and verify
	// each refers to a declared scalar parameter.
	refs := collectServiceRefs(&svc)
	for _, ref := range refs {
		if !scalarParams[ref] {
			return nil, wrap(fmt.Sprintf("service references undeclared scalar parameter ${%s}", ref))
		}
	}

	// Validate volume references: named volumes in short-form entries must be
	// declared in the component's top-level volumes.
	declaredVolumes := make(map[string]bool)
	for _, v := range raw.Volumes {
		if declaredVolumes[v.Name] {
			return nil, wrap(fmt.Sprintf("duplicate top-level volume name %q", v.Name))
		}
		declaredVolumes[v.Name] = true
	}
	for _, entry := range svc.Volumes {
		src := volumeSource(entry)
		if src == "" {
			continue // anonymous volume
		}
		if isBindMountOrInterp(src) {
			continue
		}
		if !declaredVolumes[src] {
			return nil, wrap(fmt.Sprintf("service volume source %q is not declared in top-level volumes", src))
		}
	}

	// Validate network references: service networks must be declared in
	// top-level networks (except "default").
	declaredNetworks := make(map[string]bool)
	for _, n := range raw.Networks {
		if declaredNetworks[n.Name] {
			return nil, wrap(fmt.Sprintf("duplicate top-level network name %q", n.Name))
		}
		declaredNetworks[n.Name] = true
	}
	for _, net := range svc.Networks {
		if net == "default" {
			continue
		}
		if !declaredNetworks[net] {
			return nil, wrap(fmt.Sprintf("service network %q is not declared in top-level networks", net))
		}
	}

	comp := &domain.ComponentDef{
		ID:          raw.ID,
		Description: raw.Description,
		Params:      raw.Params,
		Service:     svc,
		Volumes:     raw.Volumes,
		Networks:    raw.Networks,
	}
	return comp, nil
}

// ParsePreset decodes and validates a preset definition from raw YAML bytes.
// location is used in error messages; expectedID is the file stem that the name must match.
func ParsePreset(data []byte, location string, expectedID string) (*domain.Preset, error) {
	var raw rawPreset
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, errs.InvalidPresetError{Location: location, Err: fmt.Errorf("YAML decode: %w", err)}
	}

	wrap := func(msg string) error {
		return errs.InvalidPresetError{Location: location, Err: fmt.Errorf("%s", msg)}
	}

	// Validate name (serves as the preset ID).
	if !ValidIdentifier(raw.Name) {
		return nil, wrap(fmt.Sprintf("name %q does not match identifier pattern", raw.Name))
	}
	if raw.Name != expectedID {
		return nil, wrap(fmt.Sprintf("name %q does not match file stem %q", raw.Name, expectedID))
	}

	// At least one service.
	if len(raw.Services) == 0 {
		return nil, wrap("preset must have at least one service")
	}

	// No duplicate component IDs; every component matches identifier pattern.
	seen := make(map[string]bool)
	for _, svc := range raw.Services {
		if !ValidIdentifier(svc.Component) {
			return nil, wrap(fmt.Sprintf("service component %q does not match identifier pattern", svc.Component))
		}
		if seen[svc.Component] {
			return nil, wrap(fmt.Sprintf("duplicate component %q in preset", svc.Component))
		}
		seen[svc.Component] = true
	}

	preset := &domain.Preset{
		ID:          raw.Name,
		Description: raw.Description,
		Services:    raw.Services,
	}
	return preset, nil
}

// collectServiceRefs extracts all ${VAR} references from the typed fields of
// a service template. Escaped $$ are ignored.
func collectServiceRefs(svc *domain.ServiceTemplate) []string {
	var refs []string
	seen := make(map[string]bool)

	addRefs := func(s string) {
		// Remove escaped dollars first.
		cleaned := strings.ReplaceAll(s, "$$", "")
		matches := interpRefPattern.FindAllStringSubmatch(cleaned, -1)
		for _, m := range matches {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				refs = append(refs, name)
			}
		}
	}

	addRefs(svc.Image)

	for _, p := range svc.Ports {
		addRefs(p)
	}

	for _, v := range svc.Environment {
		addRefs(v)
	}

	for _, v := range svc.Volumes {
		addRefs(v)
	}

	// Networks are plain names, not interpolated, but check anyway.
	for _, n := range svc.Networks {
		addRefs(n)
	}

	for _, dep := range svc.DependsOn {
		addRefs(dep.Condition)
	}

	if svc.HealthCheck != nil {
		for _, t := range svc.HealthCheck.Test {
			addRefs(t)
		}
		addRefs(svc.HealthCheck.Interval)
		addRefs(svc.HealthCheck.Timeout)
		addRefs(svc.HealthCheck.StartPeriod)
	}

	addRefs(svc.Restart)

	return refs
}

// volumeSource extracts the source part from a short-form volume entry
// ("source:target[:opts]"). Returns "" for anonymous volumes (no colon).
func volumeSource(entry string) string {
	parts := strings.SplitN(entry, ":", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

// isBindMountOrInterp reports whether a volume source looks like a bind mount
// (absolute path, relative path, home shortcut) or contains an interpolation
// reference. These are skipped during volume reference validation.
func isBindMountOrInterp(src string) bool {
	if strings.HasPrefix(src, "/") ||
		strings.HasPrefix(src, "./") ||
		strings.HasPrefix(src, "../") ||
		strings.HasPrefix(src, "~") {
		return true
	}
	if strings.Contains(src, "${") {
		return true
	}
	return false
}
