package assemble

import (
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

func init() { RegisterTarget(VolumesMerger{}) }

// VolumesMerger appends volume mounts to a service.
type VolumesMerger struct{}

func (VolumesMerger) Target() string { return "volumes" }

func (VolumesMerger) Merge(svc *domain.Service, value any) error {
	items, err := stringList("volumes", value)
	if err != nil {
		return err
	}
	svc.Volumes = appendUnique(svc.Volumes, items)
	return nil
}
