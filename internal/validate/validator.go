// Package validate checks an assembled project for internal conflicts.
package validate

import (
	"fmt"
	"sort"
	"sync"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// Validator answers "does the already-assembled project conflict with itself".
type Validator interface {
	Name() string
	Validate(p *domain.Project) []error
}

// Default returns the built-in validators.
func Default() []Validator {
	return []Validator{PortsValidator{}, NamesValidator{}, DependsValidator{}, VolumesValidator{}}
}

// RunAll runs every validator in its own goroutine and returns the collected
// errors sorted by message so the output is deterministic. A panicking
// validator is reported as an error instead of crashing the process.
func RunAll(p *domain.Project, validators []Validator) []error {
	var wg sync.WaitGroup
	results := make(chan []error, len(validators))
	for _, v := range validators {
		wg.Add(1)
		go func(v Validator) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results <- []error{fmt.Errorf("validator %q panicked: %v", v.Name(), r)}
				}
			}()
			results <- v.Validate(p)
		}(v)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for batch := range results {
		errs = append(errs, batch...)
	}
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

func sortedServiceNames(p *domain.Project) []string {
	names := make([]string, 0, len(p.Services))
	for n := range p.Services {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
