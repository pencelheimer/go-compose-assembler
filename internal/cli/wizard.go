package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// paramAnswer is one parameter as shown to and edited by the user.
type paramAnswer struct {
	Name     string
	Type     string
	Required bool
	Prefill  string // effective value before editing: preset value, else default
	Value    string // what the user ended up with
}

type componentAnswers struct {
	ID     string
	Params []paramAnswer
}

// buildWizardRequest converts the user's answers into a request. In preset
// mode only values that differ from what the preset/defaults already give
// become overrides; in ad-hoc mode every non-default value is a param.
func buildWizardRequest(preset string, comps []componentAnswers) domain.GenerateRequest {
	value := func(p paramAnswer) any {
		if p.Type == "list" {
			items := []any{}
			for _, it := range strings.Split(p.Value, ",") {
				if it = strings.TrimSpace(it); it != "" {
					items = append(items, it)
				}
			}
			return items
		}
		return p.Value
	}
	changed := func(p paramAnswer) bool {
		if p.Type == "list" {
			return normalizeList(p.Value) != normalizeList(p.Prefill)
		}
		return p.Value != p.Prefill
	}

	req := domain.GenerateRequest{}
	if preset != "" {
		req.Preset = preset
		req.Overrides = map[string]map[string]any{}
	}
	for _, c := range comps {
		params := map[string]any{}
		for _, p := range c.Params {
			if strings.TrimSpace(p.Value) == "" && p.Type == "list" {
				if preset != "" && changed(p) {
					params[p.Name] = value(p) // user cleared the preset's list
				}
				continue
			}
			if p.Type != "list" && strings.TrimSpace(p.Value) == "" {
				continue // an emptied scalar falls back to the preset value or default
			}
			if changed(p) || (preset == "" && p.Required) {
				params[p.Name] = value(p)
			}
		}
		if preset != "" {
			if len(params) > 0 {
				req.Overrides[c.ID] = params
			}
			continue
		}
		req.Services = append(req.Services, domain.ServiceSelection{Component: c.ID, Params: nilIfEmpty(params)})
	}
	return req
}

func nilIfEmpty(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}

func normalizeList(s string) string {
	var items []string
	for _, it := range strings.Split(s, ",") {
		if it = strings.TrimSpace(it); it != "" {
			items = append(items, it)
		}
	}
	return strings.Join(items, ",")
}

func prefillFor(p domain.ComponentParam, preset map[string]any) string {
	if v, ok := preset[p.Name]; ok && v != nil {
		if l, ok := v.([]any); ok {
			parts := make([]string, len(l))
			for i, e := range l {
				parts[i] = fmt.Sprint(e)
			}
			return strings.Join(parts, ",")
		}
		return fmt.Sprint(v)
	}
	if p.Type == "list" {
		return ""
	}
	return p.Default
}

func isSecret(name string) bool {
	n := strings.ToUpper(name)
	return strings.Contains(n, "PASSWORD") || strings.Contains(n, "SECRET") || strings.Contains(n, "TOKEN")
}

func paramDescription(def *domain.ComponentDef, name string) string {
	for _, p := range def.Params {
		if p.Name == name {
			return p.Description
		}
	}
	return ""
}

// StdTerminal reports whether stdin and stdout are terminals.
func StdTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
