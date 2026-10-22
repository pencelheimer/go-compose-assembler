package source

import (
	"context"
	"errors"
	"sort"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// Catalog implements both ComponentCatalog and PresetCatalog from the domain
// contracts using a Resolver. It is safe for concurrent use.
type Catalog struct {
	resolver *Resolver
}

// NewCatalog creates a Catalog backed by the given resolver.
func NewCatalog(resolver *Resolver) *Catalog {
	return &Catalog{resolver: resolver}
}

// NewDefaultCatalog creates a Catalog over the default resolver (cwd → user config → embedded).
func NewDefaultCatalog() *Catalog {
	return NewCatalog(NewDefaultResolver())
}

// ListComponents returns sorted component summaries. Files that fail to load or
// validate are skipped; their errors are joined into the returned error.
func (c *Catalog) ListComponents(ctx context.Context) ([]domain.ComponentSummary, error) {
	ids, listErr := c.resolver.ListComponents(ctx)

	var summaries []domain.ComponentSummary
	var errs []error

	if listErr != nil {
		errs = append(errs, listErr)
	}

	for _, id := range ids {
		comp, err := c.GetComponent(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		summaries = append(summaries, domain.ComponentSummary{
			ID:          comp.ID,
			Description: comp.Description,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})

	var joined error
	if len(errs) > 0 {
		joined = errors.Join(errs...)
	}
	return summaries, joined
}

// GetComponent resolves and parses a component by identifier.
func (c *Catalog) GetComponent(ctx context.Context, id string) (*domain.ComponentDef, error) {
	data, srcName, err := c.resolver.LoadComponent(ctx, id)
	if err != nil {
		return nil, err
	}
	return ParseComponent(data, srcName+":components/"+id+".yaml", id)
}

// ListPresets returns sorted preset summaries. Files that fail to load or
// validate are skipped; their errors are joined into the returned error.
func (c *Catalog) ListPresets(ctx context.Context) ([]domain.PresetSummary, error) {
	ids, listErr := c.resolver.ListPresets(ctx)

	var summaries []domain.PresetSummary
	var errs []error

	if listErr != nil {
		errs = append(errs, listErr)
	}

	for _, id := range ids {
		preset, err := c.GetPreset(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		summaries = append(summaries, domain.PresetSummary{
			ID:          preset.ID,
			Description: preset.Description,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})

	var joined error
	if len(errs) > 0 {
		joined = errors.Join(errs...)
	}
	return summaries, joined
}

// GetPreset resolves and parses a preset by identifier.
func (c *Catalog) GetPreset(ctx context.Context, id string) (*domain.Preset, error) {
	data, srcName, err := c.resolver.LoadPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	return ParsePreset(data, srcName+":presets/"+id+".yaml", id)
}
