package web

import (
	"errors"
	"net/http"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// Error kinds mirror the category sentinels in errs.
const (
	kindInvalidInput      = "invalid_input"
	kindNotFound          = "not_found"
	kindInvalidDefinition = "invalid_definition"
	kindConflict          = "conflict"
	kindInternal          = "internal"
)

// apiError is the body of every non-2xx API response.
type apiError struct {
	Kind    string        `json:"kind"`
	Message string        `json:"message"`
	Details []errorDetail `json:"details"`
}

// errorDetail describes one underlying error with the fields the UI needs to
// place it next to the right service or parameter.
type errorDetail struct {
	Type      string   `json:"type"`
	Message   string   `json:"message"`
	Component string   `json:"component,omitempty"`
	Param     string   `json:"param,omitempty"`
	Services  []string `json:"services,omitempty"`
	Value     string   `json:"value,omitempty"` // port, env key or volume
	Chain     []string `json:"chain,omitempty"`
	Location  string   `json:"location,omitempty"`
	Expected  string   `json:"expected,omitempty"`
	Actual    string   `json:"actual,omitempty"`
}

func writeError(w http.ResponseWriter, err error) {
	kind, status := classify(err)
	details := []errorDetail{}
	for _, e := range flatten(err) {
		details = append(details, describe(e))
	}
	writeJSON(w, status, apiError{Kind: kind, Message: err.Error(), Details: details})
}

// classify maps an error to its kind and HTTP status via the category sentinels.
func classify(err error) (string, int) {
	switch {
	case errors.Is(err, errs.ErrInvalidInput):
		return kindInvalidInput, http.StatusBadRequest
	case errors.Is(err, errs.ErrNotFound):
		return kindNotFound, http.StatusNotFound
	case errors.Is(err, errs.ErrInvalidDefinition):
		return kindInvalidDefinition, http.StatusInternalServerError
	case errors.Is(err, errs.ErrConflict):
		return kindConflict, http.StatusConflict
	}
	return kindInternal, http.StatusInternalServerError
}

// flatten splits joined errors (errors.Join, validate.RunAll) into leaves.
func flatten(err error) []error {
	if err == nil {
		return nil
	}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range j.Unwrap() {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return []error{err}
}

func describe(err error) errorDetail {
	d := errorDetail{Type: "error", Message: err.Error()}

	var (
		missing  errs.MissingRequiredParamError
		unknown  errs.UnknownParamError
		mismatch errs.ParamTypeMismatchError
		request  errs.InvalidRequestError
		compNF   errs.ComponentNotFoundError
		presetNF errs.PresetNotFoundError
		badComp  errs.InvalidComponentError
		badPre   errs.InvalidPresetError
		target   errs.UnknownTargetError
		port     errs.PortConflictError
		volume   errs.VolumeConflictError
		env      errs.EnvConflictError
		cycle    errs.CyclicDependencyError
		dup      errs.DuplicateServiceError
	)
	switch {
	case errors.As(err, &missing):
		d.Type, d.Component, d.Param = "missing_param", missing.Component, missing.Param
	case errors.As(err, &unknown):
		d.Type, d.Component, d.Param = "unknown_param", unknown.Component, unknown.Param
	case errors.As(err, &mismatch):
		d.Type, d.Component, d.Param = "param_type", mismatch.Component, mismatch.Param
		d.Expected, d.Actual = mismatch.ExpectedType, mismatch.ActualType
	case errors.As(err, &request):
		d.Type = "invalid_request"
	case errors.As(err, &compNF):
		d.Type, d.Component = "component_not_found", compNF.ID
	case errors.As(err, &presetNF):
		d.Type, d.Value = "preset_not_found", presetNF.ID
	case errors.As(err, &badComp):
		d.Type, d.Location = "invalid_component", badComp.Location
	case errors.As(err, &badPre):
		d.Type, d.Location = "invalid_preset", badPre.Location
	case errors.As(err, &target):
		d.Type, d.Value = "unknown_target", target.Target
	case errors.As(err, &port):
		d.Type, d.Value, d.Services = "port_conflict", port.Port, []string{port.ServiceA, port.ServiceB}
	case errors.As(err, &volume):
		d.Type, d.Value, d.Services = "volume_conflict", volume.Volume, []string{volume.ServiceA, volume.ServiceB}
	case errors.As(err, &env):
		d.Type, d.Value, d.Services = "env_conflict", env.Key, []string{env.ServiceA, env.ServiceB}
	case errors.As(err, &cycle):
		d.Type, d.Chain = "cyclic_dependency", cycle.Chain
	case errors.As(err, &dup):
		d.Type, d.Component = "duplicate_service", dup.Name
	}
	return d
}
