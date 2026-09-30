package domain

// ServiceSelection represents an ad-hoc service in a generate request.
type ServiceSelection struct {
	Component string         `json:"component" yaml:"component"`
	Params    map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// GenerateRequest is the input to the generation engine.
type GenerateRequest struct {
	Preset    string                    `json:"preset,omitempty"`
	Overrides map[string]map[string]any `json:"overrides,omitempty"`
	Services  []ServiceSelection        `json:"services,omitempty"`
}

// GenerateResult is the output of a successful generation.
type GenerateResult struct {
	Project  Project           `json:"project"`
	EnvMap   map[string]string `json:"env"`
	Warnings []string          `json:"warnings,omitempty"`
}
