package export

import (
	"encoding/json"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// Bundle is the JSON payload returned by the web API.
type Bundle struct {
	Compose  string   `json:"compose"`
	Env      string   `json:"env"`
	Warnings []string `json:"warnings,omitempty"`
}

// BundleJSON renders res with exp and wraps the files into a JSON document.
func BundleJSON(exp Exporter, res *domain.GenerateResult) ([]byte, error) {
	compose, env, err := exp.Export(res)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Bundle{Compose: string(compose), Env: string(env), Warnings: res.Warnings})
}
