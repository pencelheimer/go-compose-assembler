package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

func TestErrorCategoryMatching(t *testing.T) {
	allSentinels := []error{
		errs.ErrNotFound,
		errs.ErrInvalidDefinition,
		errs.ErrInvalidInput,
		errs.ErrConflict,
	}

	tests := []struct {
		name    string
		err     error
		matches error
	}{
		{
			name:    "PresetNotFoundError",
			err:     errs.PresetNotFoundError{ID: "web", SearchedLocations: []string{"./presets", "embedded"}},
			matches: errs.ErrNotFound,
		},
		{
			name:    "ComponentNotFoundError",
			err:     errs.ComponentNotFoundError{ID: "redis", SearchedLocations: []string{"./components"}},
			matches: errs.ErrNotFound,
		},
		{
			name:    "InvalidPresetError",
			err:     errs.InvalidPresetError{Location: "presets/bad.yaml", Err: fmt.Errorf("parse error")},
			matches: errs.ErrInvalidDefinition,
		},
		{
			name:    "InvalidComponentError",
			err:     errs.InvalidComponentError{Location: "components/bad.yaml", Err: fmt.Errorf("parse error")},
			matches: errs.ErrInvalidDefinition,
		},
		{
			name:    "UnknownTargetError",
			err:     errs.UnknownTargetError{Target: "foobar"},
			matches: errs.ErrInvalidDefinition,
		},
		{
			name:    "MissingRequiredParamError",
			err:     errs.MissingRequiredParamError{Component: "postgres", Param: "POSTGRES_PASSWORD"},
			matches: errs.ErrInvalidInput,
		},
		{
			name:    "UnknownParamError",
			err:     errs.UnknownParamError{Component: "postgres", Param: "UNKNOWN"},
			matches: errs.ErrInvalidInput,
		},
		{
			name:    "ParamTypeMismatchError",
			err:     errs.ParamTypeMismatchError{Component: "postgres", Param: "PORT", ExpectedType: "scalar", ActualType: "list"},
			matches: errs.ErrInvalidInput,
		},
		{
			name:    "InvalidRequestError",
			err:     errs.InvalidRequestError{Reason: "both preset and services specified"},
			matches: errs.ErrInvalidInput,
		},
		{
			name:    "PortConflictError",
			err:     errs.PortConflictError{Port: "8080/tcp", ServiceA: "app", ServiceB: "proxy"},
			matches: errs.ErrConflict,
		},
		{
			name:    "VolumeConflictError",
			err:     errs.VolumeConflictError{Volume: "data", ServiceA: "db1", ServiceB: "db2", Reason: "different drivers"},
			matches: errs.ErrConflict,
		},
		{
			name:    "EnvConflictError",
			err:     errs.EnvConflictError{Key: "PORT", ServiceA: "app", ValueA: "3000", ServiceB: "proxy", ValueB: "8080"},
			matches: errs.ErrConflict,
		},
		{
			name:    "CyclicDependencyError",
			err:     errs.CyclicDependencyError{Chain: []string{"a", "b", "c"}},
			matches: errs.ErrConflict,
		},
		{
			name:    "DuplicateServiceError",
			err:     errs.DuplicateServiceError{Name: "postgres"},
			matches: errs.ErrConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.matches) {
				t.Errorf("errors.Is(%T, %v) = false, want true", tc.err, tc.matches)
			}

			for _, sentinel := range allSentinels {
				if sentinel == tc.matches {
					continue
				}
				if errors.Is(tc.err, sentinel) {
					t.Errorf("errors.Is(%T, %v) = true, want false", tc.err, sentinel)
				}
			}

			if tc.err.Error() == "" {
				t.Errorf("%T.Error() returned empty string", tc.err)
			}
		})
	}
}
