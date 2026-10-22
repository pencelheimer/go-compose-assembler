package validate

import (
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// VolumesValidator catches volume name collisions: a named volume shared by
// several services (usually two components that picked the same name), two
// mounts on one container path, and mounts of undeclared named volumes.
// Names are never auto-prefixed, so this is the only protection.
type VolumesValidator struct{}

func (VolumesValidator) Name() string { return "volumes" }

func (VolumesValidator) Validate(p *domain.Project) []error {
	var errList []error
	users := map[string]string{} // named volume -> first service mounting it
	for _, svcName := range sortedServiceNames(p) {
		paths := map[string]string{} // container path -> source
		for _, spec := range p.Services[svcName].Volumes {
			parts := strings.Split(spec, ":")
			src := parts[0]
			if len(parts) > 1 {
				if prev, dup := paths[parts[1]]; dup {
					errList = append(errList, errs.VolumeConflictError{
						Volume: src, ServiceA: svcName, ServiceB: svcName,
						Reason: "container path " + parts[1] + " is already mounted from " + prev,
					})
				}
				paths[parts[1]] = src
			}
			if src == "" || len(parts) < 2 || strings.ContainsAny(src[:1], "/.~") || strings.Contains(src, "$") {
				continue // bind mount, anonymous volume, or unresolved reference
			}
			if _, ok := p.Volumes[src]; !ok {
				errList = append(errList, errs.VolumeConflictError{
					Volume: src, ServiceA: svcName, ServiceB: svcName,
					Reason: "named volume is not declared at top level",
				})
			}
			if first, shared := users[src]; shared && first != svcName {
				errList = append(errList, errs.VolumeConflictError{
					Volume: src, ServiceA: first, ServiceB: svcName,
					Reason: "named volume is mounted by more than one service",
				})
			} else if !shared {
				users[src] = svcName
			}
		}
	}
	return errList
}
