package errs_test

import (
	"errors"
	"fmt"
	"strings"
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

func TestErrorsAsValueReceivers(t *testing.T) {
	tests := []struct {
		name string
		err  error
		asFn func(error) bool
	}{
		{
			name: "PresetNotFoundError",
			err:  errs.PresetNotFoundError{ID: "web", SearchedLocations: []string{"./presets"}},
			asFn: func(e error) bool { var target errs.PresetNotFoundError; return errors.As(e, &target) },
		},
		{
			name: "ComponentNotFoundError",
			err:  errs.ComponentNotFoundError{ID: "redis", SearchedLocations: []string{"./components"}},
			asFn: func(e error) bool { var target errs.ComponentNotFoundError; return errors.As(e, &target) },
		},
		{
			name: "InvalidPresetError",
			err:  errs.InvalidPresetError{Location: "presets/bad.yaml", Err: fmt.Errorf("parse error")},
			asFn: func(e error) bool { var target errs.InvalidPresetError; return errors.As(e, &target) },
		},
		{
			name: "InvalidComponentError",
			err:  errs.InvalidComponentError{Location: "components/bad.yaml", Err: fmt.Errorf("parse error")},
			asFn: func(e error) bool { var target errs.InvalidComponentError; return errors.As(e, &target) },
		},
		{
			name: "UnknownTargetError",
			err:  errs.UnknownTargetError{Target: "foobar"},
			asFn: func(e error) bool { var target errs.UnknownTargetError; return errors.As(e, &target) },
		},
		{
			name: "MissingRequiredParamError",
			err:  errs.MissingRequiredParamError{Component: "postgres", Param: "POSTGRES_PASSWORD"},
			asFn: func(e error) bool { var target errs.MissingRequiredParamError; return errors.As(e, &target) },
		},
		{
			name: "UnknownParamError",
			err:  errs.UnknownParamError{Component: "postgres", Param: "UNKNOWN"},
			asFn: func(e error) bool { var target errs.UnknownParamError; return errors.As(e, &target) },
		},
		{
			name: "ParamTypeMismatchError",
			err:  errs.ParamTypeMismatchError{Component: "postgres", Param: "PORT", ExpectedType: "scalar", ActualType: "list"},
			asFn: func(e error) bool { var target errs.ParamTypeMismatchError; return errors.As(e, &target) },
		},
		{
			name: "InvalidRequestError",
			err:  errs.InvalidRequestError{Reason: "both preset and services specified"},
			asFn: func(e error) bool { var target errs.InvalidRequestError; return errors.As(e, &target) },
		},
		{
			name: "PortConflictError",
			err:  errs.PortConflictError{Port: "8080/tcp", ServiceA: "app", ServiceB: "proxy"},
			asFn: func(e error) bool { var target errs.PortConflictError; return errors.As(e, &target) },
		},
		{
			name: "VolumeConflictError",
			err:  errs.VolumeConflictError{Volume: "data", ServiceA: "db1", ServiceB: "db2", Reason: "different drivers"},
			asFn: func(e error) bool { var target errs.VolumeConflictError; return errors.As(e, &target) },
		},
		{
			name: "EnvConflictError",
			err:  errs.EnvConflictError{Key: "PORT", ServiceA: "app", ValueA: "3000", ServiceB: "proxy", ValueB: "8080"},
			asFn: func(e error) bool { var target errs.EnvConflictError; return errors.As(e, &target) },
		},
		{
			name: "CyclicDependencyError",
			err:  errs.CyclicDependencyError{Chain: []string{"a", "b", "c"}},
			asFn: func(e error) bool { var target errs.CyclicDependencyError; return errors.As(e, &target) },
		},
		{
			name: "DuplicateServiceError",
			err:  errs.DuplicateServiceError{Name: "postgres"},
			asFn: func(e error) bool { var target errs.DuplicateServiceError; return errors.As(e, &target) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.asFn(tc.err) {
				t.Errorf("errors.As(%T) with value target returned false, want true", tc.err)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	inner := fmt.Errorf("bad yaml")

	t.Run("InvalidPresetError", func(t *testing.T) {
		err := errs.InvalidPresetError{Location: "test.yaml", Err: inner}
		if !errors.Is(err, inner) {
			t.Error("InvalidPresetError should unwrap to inner error")
		}
	})

	t.Run("InvalidComponentError", func(t *testing.T) {
		err := errs.InvalidComponentError{Location: "test.yaml", Err: inner}
		if !errors.Is(err, inner) {
			t.Error("InvalidComponentError should unwrap to inner error")
		}
	})
}

func TestErrorMessages(t *testing.T) {
	t.Run("PresetNotFoundError", func(t *testing.T) {
		err := errs.PresetNotFoundError{ID: "web", SearchedLocations: []string{"a", "b"}}
		msg := err.Error()
		if !strings.Contains(msg, "web") {
			t.Errorf("expected message to contain %q, got %q", "web", msg)
		}
		if !strings.Contains(msg, "a, b") {
			t.Errorf("expected message to contain %q, got %q", "a, b", msg)
		}
	})

	t.Run("CyclicDependencyError", func(t *testing.T) {
		err := errs.CyclicDependencyError{Chain: []string{"x", "y", "z"}}
		msg := err.Error()
		if !strings.Contains(msg, "x -> y -> z -> x") {
			t.Errorf("expected message to contain %q, got %q", "x -> y -> z -> x", msg)
		}
	})

	t.Run("EnvConflictError", func(t *testing.T) {
		err := errs.EnvConflictError{Key: "K", ServiceA: "s1", ValueA: "v1", ServiceB: "s2", ValueB: "v2"}
		msg := err.Error()
		for _, substr := range []string{"K", "v1", "v2"} {
			if !strings.Contains(msg, substr) {
				t.Errorf("expected message to contain %q, got %q", substr, msg)
			}
		}
	})
}
