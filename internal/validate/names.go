package validate

import (
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// NamesValidator is a defensive check that service names are unique and match
// their map keys. The engine's single-instance policy already prevents
// duplicates; this guards against bugs and hand-built projects.
type NamesValidator struct{}

func (NamesValidator) Name() string { return "names" }

func (NamesValidator) Validate(p *domain.Project) []error {
	var errList []error
	owner := map[string]string{} // Name -> first key using it
	for _, key := range sortedServiceNames(p) {
		svc := p.Services[key]
		if svc.Name == "" || svc.Name != key {
			errList = append(errList, errs.InvalidRequestError{
				Reason: "service key " + key + " does not match its name " + svc.Name,
			})
		}
		if _, dup := owner[svc.Name]; dup && svc.Name != "" {
			errList = append(errList, errs.DuplicateServiceError{Name: svc.Name})
			continue
		}
		owner[svc.Name] = key
	}
	return errList
}
