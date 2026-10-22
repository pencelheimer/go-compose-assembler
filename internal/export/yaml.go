package export

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// YAMLExporter renders compose.yaml and .env.
type YAMLExporter struct{}

var _ Exporter = YAMLExporter{}

// composeService is the on-disk shape of a service. domain.Service is not
// marshaled directly because Compose rejects a service-level "name" key.
type composeService struct {
	Image       string                              `yaml:"image"`
	Restart     string                              `yaml:"restart,omitempty"`
	Ports       []string                            `yaml:"ports,omitempty"`
	Environment map[string]string                   `yaml:"environment,omitempty"`
	DependsOn   map[string]domain.ServiceDependency `yaml:"depends_on,omitempty"`
	Volumes     []string                            `yaml:"volumes,omitempty"`
	Networks    []string                            `yaml:"networks,omitempty"`
	HealthCheck *domain.HealthCheck                 `yaml:"healthcheck,omitempty"`
	Extra       map[string]any                      `yaml:",inline"`
}

// composeResource is a top-level volume or network. The name is emitted only
// when it differs from the map key, because an explicit name disables Compose's
// project-name prefixing.
type composeResource struct {
	Name          string            `yaml:"name,omitempty"`
	Driver        string            `yaml:"driver,omitempty"`
	DriverOptions map[string]string `yaml:"driver_opts,omitempty"`
	External      bool              `yaml:"external,omitempty"`
}

type composeFile struct {
	Services map[string]composeService  `yaml:"services"`
	Volumes  map[string]composeResource `yaml:"volumes,omitempty"`
	Networks map[string]composeResource `yaml:"networks,omitempty"`
}

func (YAMLExporter) Export(res *domain.GenerateResult) ([]byte, []byte, error) {
	if res == nil {
		return nil, nil, fmt.Errorf("export: nil result")
	}
	doc := composeFile{Services: map[string]composeService{}}
	for key, s := range res.Project.Services {
		doc.Services[key] = composeService{
			Image: s.Image, Restart: s.Restart, Ports: s.Ports, Environment: s.Environment,
			DependsOn: s.DependsOn, Volumes: s.Volumes, Networks: s.Networks,
			HealthCheck: s.HealthCheck, Extra: s.Extra,
		}
	}
	if len(res.Project.Volumes) > 0 {
		doc.Volumes = map[string]composeResource{}
		for key, v := range res.Project.Volumes {
			doc.Volumes[key] = composeResource{Name: nameIfDiffers(key, v.Name), Driver: v.Driver, DriverOptions: v.DriverOptions, External: v.External}
		}
	}
	if len(res.Project.Networks) > 0 {
		doc.Networks = map[string]composeResource{}
		for key, n := range res.Project.Networks {
			doc.Networks[key] = composeResource{Name: nameIfDiffers(key, n.Name), Driver: n.Driver, DriverOptions: n.DriverOptions, External: n.External}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, nil, fmt.Errorf("export compose: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, nil, fmt.Errorf("export compose: %w", err)
	}
	return buf.Bytes(), renderEnv(res.EnvMap), nil
}

func nameIfDiffers(key, name string) string {
	if name == key {
		return ""
	}
	return name
}

var plainEnvValue = regexp.MustCompile(`^[A-Za-z0-9_./:@%+,=-]*$`)

// renderEnv writes sorted KEY=value lines, quoting anything Compose could
// re-interpret (spaces, '#', '$', quotes, newlines).
func renderEnv(env map[string]string) []byte {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + quoteEnv(env[k]) + "\n")
	}
	return []byte(b.String())
}

func quoteEnv(v string) string {
	if plainEnvValue.MatchString(v) {
		return v
	}
	if !strings.ContainsAny(v, "'\n\r") {
		return "'" + v + "'" // single quotes: fully literal, no interpolation
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "$", `\$`)
	return `"` + r.Replace(v) + `"`
}
