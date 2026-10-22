package assemble

import (
	"fmt"
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

func init() { RegisterTarget(EnvironmentMerger{}) }

// EnvironmentMerger adds KEY=VALUE entries to a service's environment,
// overriding existing keys.
type EnvironmentMerger struct{}

func (EnvironmentMerger) Target() string { return "environment" }

func (EnvironmentMerger) Merge(svc *domain.Service, value any) error {
	items, err := stringList("environment", value)
	if err != nil {
		return err
	}
	for _, it := range items {
		k, v, ok := strings.Cut(it, "=")
		if !ok || k == "" {
			return errs.InvalidRequestError{Reason: fmt.Sprintf("environment: %q is not KEY=VALUE", it)}
		}
		if svc.Environment == nil {
			svc.Environment = map[string]string{}
		}
		svc.Environment[k] = v
	}
	return nil
}
