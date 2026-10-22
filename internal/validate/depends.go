package validate

import (
	"sort"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// DependsValidator detects cycles in depends_on. Dependencies on services that
// are not part of the project are not its concern (the engine warns about them).
type DependsValidator struct{}

func (DependsValidator) Name() string { return "depends" }

func (DependsValidator) Validate(p *domain.Project) []error {
	const (
		unvisited = iota
		inStack
		done
	)
	state := map[string]int{}
	var (
		stack   []string
		errList []error
		visit   func(string)
	)
	visit = func(n string) {
		state[n] = inStack
		stack = append(stack, n)
		deps := make([]string, 0, len(p.Services[n].DependsOn))
		for d := range p.Services[n].DependsOn {
			if _, ok := p.Services[d]; ok {
				deps = append(deps, d)
			}
		}
		sort.Strings(deps)
		for _, d := range deps {
			switch state[d] {
			case unvisited:
				visit(d)
			case inStack:
				start := 0
				for i, s := range stack {
					if s == d {
						start = i
						break
					}
				}
				chain := append(append([]string{}, stack[start:]...), d)
				errList = append(errList, errs.CyclicDependencyError{Chain: chain})
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
	}
	for _, n := range sortedServiceNames(p) {
		if state[n] == unvisited {
			visit(n)
		}
	}
	return errList
}
