package domain

// PresetSummary is a lightweight representation for listing presets.
type PresetSummary struct {
	ID          string `json:"id" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// PresetService maps a component to its preset parameter values.
type PresetService struct {
	Component string         `json:"component" yaml:"component"`
	Params    map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// Preset is a named collection of services with their parameters.
type Preset struct {
	ID          string          `json:"id" yaml:"name"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Services    []PresetService `json:"services" yaml:"services"`
}
