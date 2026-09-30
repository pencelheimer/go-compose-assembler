package domain

// Project represents the fully assembled domain state.
type Project struct {
	Services map[string]Service `json:"services" yaml:"services"`
	Volumes  map[string]Volume  `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Networks map[string]Network `json:"networks,omitempty" yaml:"networks,omitempty"`
}

// Service represents a single composed service.
type Service struct {
	Name        string                       `json:"name" yaml:"name"`
	Image       string                       `json:"image" yaml:"image"`
	Environment map[string]string            `json:"environment,omitempty" yaml:"environment,omitempty"`
	DependsOn   map[string]ServiceDependency `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Ports       []string                     `json:"ports,omitempty" yaml:"ports,omitempty"`
	Volumes     []string                     `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Networks    []string                     `json:"networks,omitempty" yaml:"networks,omitempty"`
	Restart     string                       `json:"restart,omitempty" yaml:"restart,omitempty"`
	HealthCheck *HealthCheck                 `json:"healthcheck,omitempty" yaml:"healthcheck,omitempty"`
	Extra       map[string]any               `json:"extra,omitempty" yaml:",inline"`
}

// Volume represents a top-level named volume declaration.
type Volume struct {
	Name          string            `json:"name" yaml:"name"`
	Driver        string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	DriverOptions map[string]string `json:"driver_opts,omitempty" yaml:"driver_opts,omitempty"`
	External      bool              `json:"external,omitempty" yaml:"external,omitempty"`
}

// Network represents a top-level named network declaration.
type Network struct {
	Name          string            `json:"name" yaml:"name"`
	Driver        string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	DriverOptions map[string]string `json:"driver_opts,omitempty" yaml:"driver_opts,omitempty"`
	External      bool              `json:"external,omitempty" yaml:"external,omitempty"`
}

// HealthCheck represents a service health check configuration.
// Durations are strings (e.g., "10s", "1m") for consistent JSON/YAML marshaling.
type HealthCheck struct {
	Test        []string `json:"test" yaml:"test"`
	Interval    string   `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout     string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries     int      `json:"retries,omitempty" yaml:"retries,omitempty"`
	StartPeriod string   `json:"start_period,omitempty" yaml:"start_period,omitempty"`
}
