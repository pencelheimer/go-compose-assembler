package errs

import (
	"errors"
	"fmt"
	"strings"
)

// Category sentinels.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidDefinition = errors.New("invalid definition")
	ErrConflict          = errors.New("conflict")
)

// PresetNotFoundError indicates that a preset could not be found in any source.
type PresetNotFoundError struct {
	ID                string
	SearchedLocations []string
}

func (e PresetNotFoundError) Error() string {
	return fmt.Sprintf("preset not found: %q (searched: %s)", e.ID, strings.Join(e.SearchedLocations, ", "))
}

func (e PresetNotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// ComponentNotFoundError indicates that a component could not be found in any source.
type ComponentNotFoundError struct {
	ID                string
	SearchedLocations []string
}

func (e ComponentNotFoundError) Error() string {
	return fmt.Sprintf("component not found: %q (searched: %s)", e.ID, strings.Join(e.SearchedLocations, ", "))
}

func (e ComponentNotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// InvalidPresetError indicates a preset definition failed to parse or validate.
type InvalidPresetError struct {
	Location string
	Err      error
}

func (e InvalidPresetError) Error() string {
	return fmt.Sprintf("invalid preset at %s: %s", e.Location, e.Err)
}

func (e InvalidPresetError) Is(target error) bool {
	return target == ErrInvalidDefinition
}

func (e InvalidPresetError) Unwrap() error {
	return e.Err
}

// InvalidComponentError indicates a component definition failed to parse or validate.
type InvalidComponentError struct {
	Location string
	Err      error
}

func (e InvalidComponentError) Error() string {
	return fmt.Sprintf("invalid component at %s: %s", e.Location, e.Err)
}

func (e InvalidComponentError) Is(target error) bool {
	return target == ErrInvalidDefinition
}

func (e InvalidComponentError) Unwrap() error {
	return e.Err
}

// UnknownTargetError indicates a component parameter references a target merger
// that is not registered.
type UnknownTargetError struct {
	Target string
}

func (e UnknownTargetError) Error() string {
	return fmt.Sprintf("unknown target merger: %q", e.Target)
}

func (e UnknownTargetError) Is(target error) bool {
	return target == ErrInvalidDefinition
}

// MissingRequiredParamError indicates a required parameter was not supplied.
type MissingRequiredParamError struct {
	Component string
	Param     string
}

func (e MissingRequiredParamError) Error() string {
	return fmt.Sprintf("component %q: required parameter %q is missing", e.Component, e.Param)
}

func (e MissingRequiredParamError) Is(target error) bool {
	return target == ErrInvalidInput
}

// UnknownParamError indicates a parameter was supplied that the component does
// not declare.
type UnknownParamError struct {
	Component string
	Param     string
}

func (e UnknownParamError) Error() string {
	return fmt.Sprintf("component %q: unknown parameter %q", e.Component, e.Param)
}

func (e UnknownParamError) Is(target error) bool {
	return target == ErrInvalidInput
}

// ParamTypeMismatchError indicates a parameter value does not match the
// expected type.
type ParamTypeMismatchError struct {
	Component    string
	Param        string
	ExpectedType string
	ActualType   string
}

func (e ParamTypeMismatchError) Error() string {
	return fmt.Sprintf("component %q: parameter %q expects %s, got %s", e.Component, e.Param, e.ExpectedType, e.ActualType)
}

func (e ParamTypeMismatchError) Is(target error) bool {
	return target == ErrInvalidInput
}

// InvalidRequestError indicates a malformed or invalid generate request.
type InvalidRequestError struct {
	Reason string
}

func (e InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request: %s", e.Reason)
}

func (e InvalidRequestError) Is(target error) bool {
	return target == ErrInvalidInput
}

// PortConflictError indicates two services bind the same port.
type PortConflictError struct {
	Port     string
	ServiceA string
	ServiceB string
}

func (e PortConflictError) Error() string {
	return fmt.Sprintf("port conflict: %s is used by both %q and %q", e.Port, e.ServiceA, e.ServiceB)
}

func (e PortConflictError) Is(target error) bool {
	return target == ErrConflict
}

// VolumeConflictError indicates a volume conflict between two services.
type VolumeConflictError struct {
	Volume   string
	ServiceA string
	ServiceB string
	Reason   string
}

func (e VolumeConflictError) Error() string {
	return fmt.Sprintf("volume conflict: %q between %q and %q: %s", e.Volume, e.ServiceA, e.ServiceB, e.Reason)
}

func (e VolumeConflictError) Is(target error) bool {
	return target == ErrConflict
}

// EnvConflictError indicates two services resolve the same environment key to
// different values.
type EnvConflictError struct {
	Key      string
	ServiceA string
	ValueA   string
	ServiceB string
	ValueB   string
}

func (e EnvConflictError) Error() string {
	return fmt.Sprintf("environment conflict: key %q has value %q in %q but %q in %q", e.Key, e.ValueA, e.ServiceA, e.ValueB, e.ServiceB)
}

func (e EnvConflictError) Is(target error) bool {
	return target == ErrConflict
}

// CyclicDependencyError indicates a dependency cycle was detected.
type CyclicDependencyError struct {
	Chain []string
}

func (e CyclicDependencyError) Error() string {
	closed := append(e.Chain, e.Chain[0])
	return fmt.Sprintf("cyclic dependency: %s", strings.Join(closed, " -> "))
}

func (e CyclicDependencyError) Is(target error) bool {
	return target == ErrConflict
}

// DuplicateServiceError indicates a service name appears more than once.
type DuplicateServiceError struct {
	Name string
}

func (e DuplicateServiceError) Error() string {
	return fmt.Sprintf("duplicate service: %q", e.Name)
}

func (e DuplicateServiceError) Is(target error) bool {
	return target == ErrConflict
}
