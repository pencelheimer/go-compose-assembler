package domain

// ComponentSummary is a lightweight representation for listing.
type ComponentSummary struct {
	ID          string `json:"id" yaml:"id"`
	Description string `json:"description" yaml:"description"`
}

// ComponentParam describes a single parameter of a component.
type ComponentParam struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"` // "scalar" or "list"
	Required    bool   `json:"required" yaml:"required"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Target      string `json:"target,omitempty" yaml:"target,omitempty"` // target merger name for list params
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ComponentVolume is a top-level volume declared by a component.
type ComponentVolume struct {
	Name string `json:"name" yaml:"name"`
}

// ComponentNetwork is a top-level network declared by a component.
type ComponentNetwork struct {
	Name string `json:"name" yaml:"name"`
}

// ComponentDef is the full specification of a component loaded from YAML.
type ComponentDef struct {
	ID          string             `json:"id" yaml:"id"`
	Description string             `json:"description" yaml:"description"`
	Params      []ComponentParam   `json:"params" yaml:"params"`
	Service     ServiceTemplate    `json:"service" yaml:"service"`
	Volumes     []ComponentVolume  `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Networks    []ComponentNetwork `json:"networks,omitempty" yaml:"networks,omitempty"`
}

// ServiceTemplate is the service section within a component definition YAML.
// It uses inline YAML mapping to capture arbitrary compose fields.
type ServiceTemplate struct {
	Image       string                       `json:"image,omitempty" yaml:"image,omitempty"`
	Ports       []string                     `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment map[string]string            `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes     []string                     `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Networks    []string                     `json:"networks,omitempty" yaml:"networks,omitempty"`
	DependsOn   map[string]ServiceDependency `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Restart     string                       `json:"restart,omitempty" yaml:"restart,omitempty"`
	HealthCheck *HealthCheck                 `json:"healthcheck,omitempty" yaml:"healthcheck,omitempty"`
	Extra       map[string]any               `json:"extra,omitempty" yaml:",inline"`
}

// ServiceDependency represents a depends_on entry with a condition.
type ServiceDependency struct {
	Condition string `json:"condition" yaml:"condition"`
}
