package assemble

import (
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

func init() { RegisterTarget(PortsMerger{}) }

// PortsMerger appends port mappings to a service.
type PortsMerger struct{}

func (PortsMerger) Target() string { return "ports" }

func (PortsMerger) Merge(svc *domain.Service, value any) error {
	items, err := stringList("ports", value)
	if err != nil {
		return err
	}
	svc.Ports = appendUnique(svc.Ports, items)
	return nil
}
