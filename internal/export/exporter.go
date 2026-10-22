// Package export renders a generation result into Compose and .env files.
package export

import "github.com/pencelheimer/go-compose-assembler/internal/domain"

// Exporter renders the compose document and the .env file.
type Exporter interface {
	Export(res *domain.GenerateResult) ([]byte, []byte, error) // compose document bytes, .env bytes, error
}
