// Package assemble turns a GenerateRequest into an assembled project.
package assemble

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
	"github.com/pencelheimer/go-compose-assembler/internal/validate"
)

// ComponentGetter loads component definitions.
type ComponentGetter interface {
	GetComponent(ctx context.Context, id string) (*domain.ComponentDef, error)
}

// PresetGetter loads preset definitions.
type PresetGetter interface {
	GetPreset(ctx context.Context, id string) (*domain.Preset, error)
}

// Engine resolves parameters, assembles services and validates the result.
type Engine struct {
	components ComponentGetter
	presets    PresetGetter
	validators []validate.Validator
}

// NewEngine builds an engine. A nil validators slice means validate.Default().
func NewEngine(c ComponentGetter, p PresetGetter, validators []validate.Validator) *Engine {
	if validators == nil {
		validators = validate.Default()
	}
	return &Engine{components: c, presets: p, validators: validators}
}

// selection is one requested service with its layered parameter values.
type selection struct {
	component string
	base      map[string]any // preset values or ad-hoc params
	overrides map[string]any
}

// Generate follows the data flow in AGENTS.md: select, resolve, merge, validate.
func (e *Engine) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResult, error) {
	sels, err := e.selections(ctx, req)
	if err != nil {
		return nil, err
	}

	project := domain.Project{
		Services: map[string]domain.Service{},
		Volumes:  map[string]domain.Volume{},
		Networks: map[string]domain.Network{},
	}
	envMap := map[string]string{}
	type envOwner struct{ service, value string }
	owners := map[string]envOwner{}

	for _, sel := range sels {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		def, err := e.components.GetComponent(ctx, sel.component)
		if err != nil {
			return nil, err
		}
		scalars, lists, err := resolveParams(def, sel)
		if err != nil {
			return nil, err
		}
		for _, p := range def.Params { // declaration order keeps conflicts stable
			val, ok := scalars[p.Name]
			if !ok {
				continue
			}
			if prev, dup := owners[p.Name]; dup && prev.value != val {
				return nil, errs.EnvConflictError{
					Key: p.Name, ServiceA: prev.service, ValueA: prev.value, ServiceB: def.ID, ValueB: val,
				}
			} else if !dup {
				owners[p.Name] = envOwner{def.ID, val}
				envMap[p.Name] = val
			}
		}

		svc := buildService(def)
		for _, p := range def.Params {
			val, ok := lists[p.Name]
			if !ok {
				continue
			}
			m := GetTarget(p.Target)
			if m == nil {
				return nil, errs.UnknownTargetError{Target: p.Target}
			}
			if err := m.Merge(&svc, val); err != nil {
				if !errors.Is(err, errs.ErrInvalidInput) && !errors.Is(err, errs.ErrInvalidDefinition) {
					err = errs.InvalidRequestError{Reason: err.Error()}
				}
				return nil, fmt.Errorf("component %q param %q: %w", def.ID, p.Name, err)
			}
		}
		project.Services[svc.Name] = svc
		for _, v := range def.Volumes {
			if _, ok := project.Volumes[v.Name]; !ok {
				project.Volumes[v.Name] = domain.Volume{Name: v.Name}
			}
		}
		for _, n := range def.Networks {
			if _, ok := project.Networks[n.Name]; !ok {
				project.Networks[n.Name] = domain.Network{Name: n.Name}
			}
		}
	}

	if verrs := validate.RunAll(resolvedCopy(&project, envMap), e.validators); len(verrs) > 0 {
		return nil, errors.Join(verrs...)
	}
	return &domain.GenerateResult{Project: project, EnvMap: envMap, Warnings: danglingDependencies(&project)}, nil
}

// selections validates the request shape and expands it into ordered selections.
func (e *Engine) selections(ctx context.Context, req domain.GenerateRequest) ([]selection, error) {
	overrides := map[string]map[string]any{}
	for k, v := range req.Overrides {
		if len(v) > 0 {
			overrides[k] = v
		}
	}
	invalid := func(reason string) ([]selection, error) {
		return nil, errs.InvalidRequestError{Reason: reason}
	}

	switch {
	case req.Preset != "" && len(req.Services) > 0:
		return invalid("specify either a preset or a services list, not both")
	case req.Preset == "" && len(req.Services) == 0:
		return invalid("specify a preset or a services list")
	case req.Preset == "":
		if len(overrides) > 0 {
			return invalid("overrides are only allowed together with a preset")
		}
		seen := map[string]bool{}
		sels := make([]selection, 0, len(req.Services))
		for _, s := range req.Services {
			if s.Component == "" {
				return invalid("service selection without a component")
			}
			if seen[s.Component] {
				return nil, errs.DuplicateServiceError{Name: s.Component}
			}
			seen[s.Component] = true
			sels = append(sels, selection{component: s.Component, base: s.Params})
		}
		return sels, nil
	}

	preset, err := e.presets.GetPreset(ctx, req.Preset)
	if err != nil {
		return nil, err
	}
	inPreset := map[string]bool{}
	sels := make([]selection, 0, len(preset.Services))
	for _, s := range preset.Services {
		if inPreset[s.Component] {
			return nil, errs.DuplicateServiceError{Name: s.Component}
		}
		inPreset[s.Component] = true
		sels = append(sels, selection{component: s.Component, base: s.Params, overrides: overrides[s.Component]})
	}
	for _, k := range slices.Sorted(maps.Keys(overrides)) {
		if !inPreset[k] {
			return invalid(fmt.Sprintf("override targets component %q which is not in preset %q", k, preset.ID))
		}
	}
	return sels, nil
}

// resolveParams applies override > preset/ad-hoc > default, with nil falling
// through, and returns normalized scalars plus raw list values.
func resolveParams(def *domain.ComponentDef, sel selection) (scalars map[string]string, lists map[string]any, err error) {
	declared := make(map[string]bool, len(def.Params))
	for _, p := range def.Params {
		declared[p.Name] = true
	}
	for _, layer := range []map[string]any{sel.base, sel.overrides} {
		for _, name := range slices.Sorted(maps.Keys(layer)) {
			if !declared[name] {
				return nil, nil, errs.UnknownParamError{Component: def.ID, Param: name}
			}
		}
	}

	scalars = map[string]string{}
	lists = map[string]any{}
	for _, p := range def.Params {
		var val any
		if v := sel.overrides[p.Name]; v != nil {
			val = v
		} else if v := sel.base[p.Name]; v != nil {
			val = v
		}
		switch p.Type {
		case "list":
			if val == nil {
				if p.Required {
					return nil, nil, errs.MissingRequiredParamError{Component: def.ID, Param: p.Name}
				}
				continue
			}
			if err := checkList(def.ID, p.Name, val); err != nil {
				return nil, nil, err
			}
			lists[p.Name] = val
		default:
			if val == nil {
				if p.Required {
					return nil, nil, errs.MissingRequiredParamError{Component: def.ID, Param: p.Name}
				}
				scalars[p.Name] = p.Default
				continue
			}
			s, err := normalizeScalar(def.ID, p.Name, val)
			if err != nil {
				return nil, nil, err
			}
			scalars[p.Name] = s
		}
	}
	return scalars, lists, nil
}

// buildService deep-copies the template so definitions shared by a catalog are
// never mutated.
func buildService(def *domain.ComponentDef) domain.Service {
	t := def.Service
	svc := domain.Service{
		Name:        def.ID,
		Image:       t.Image,
		Environment: maps.Clone(t.Environment),
		DependsOn:   maps.Clone(t.DependsOn),
		Ports:       slices.Clone(t.Ports),
		Volumes:     slices.Clone(t.Volumes),
		Networks:    slices.Clone(t.Networks),
		Restart:     t.Restart,
		Extra:       deepCopyMap(t.Extra),
	}
	if t.HealthCheck != nil {
		hc := *t.HealthCheck
		hc.Test = slices.Clone(hc.Test)
		svc.HealthCheck = &hc
	}
	return svc
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCopyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = deepCopyValue(e)
		}
		return out
	default:
		return v
	}
}

// danglingDependencies warns about depends_on entries that name a service
// outside the project; Compose will reject such a file.
func danglingDependencies(p *domain.Project) []string {
	var warns []string
	for name, svc := range p.Services {
		for dep := range svc.DependsOn {
			if _, ok := p.Services[dep]; !ok {
				warns = append(warns, fmt.Sprintf("service %q depends on %q, which is not part of this project", name, dep))
			}
		}
	}
	sort.Strings(warns)
	return warns
}
