package assemble

import (
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

// expand resolves ${NAME} and the modifier forms ${NAME:-d} ${NAME-d}
// ${NAME:?m} ${NAME?m} ${NAME:+alt} ${NAME+alt} against env and unescapes "$$".
// It exists only so validators can reason about concrete values (e.g. ports);
// the compose output keeps the ${...} references and Compose itself does the
// real substitution. Required-variable checks are not enforced here.
func expand(s string, env map[string]string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			b.WriteByte('$')
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			if end := strings.IndexByte(s[i:], '}'); end > 0 {
				b.WriteString(expandExpr(s[i+2:i+end], env))
				i += end
				continue
			}
		}
		b.WriteByte('$')
	}
	return b.String()
}

func expandExpr(expr string, env map[string]string) string {
	n := 0
	for n < len(expr) && (expr[n] == '_' || expr[n] >= 'A' && expr[n] <= 'Z' || expr[n] >= 'a' && expr[n] <= 'z' || n > 0 && expr[n] >= '0' && expr[n] <= '9') {
		n++
	}
	name, rest := expr[:n], expr[n:]
	val, set := env[name]
	switch {
	case rest == "":
		return val
	case strings.HasPrefix(rest, ":-"):
		if !set || val == "" {
			return rest[2:]
		}
	case strings.HasPrefix(rest, "-"):
		if !set {
			return rest[1:]
		}
	case strings.HasPrefix(rest, ":+"):
		if set && val != "" {
			return rest[2:]
		}
		return ""
	case strings.HasPrefix(rest, "+"):
		if set {
			return rest[1:]
		}
		return ""
	}
	return val // ":?" and "?" fall through: use the value
}

func expandAll(in []string, env map[string]string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = expand(s, env)
	}
	return out
}

// resolvedCopy returns a deep-enough copy of p with env values substituted
// into the fields validators inspect.
func resolvedCopy(p *domain.Project, env map[string]string) *domain.Project {
	out := &domain.Project{
		Services: make(map[string]domain.Service, len(p.Services)),
		Volumes:  p.Volumes,
		Networks: p.Networks,
	}
	for name, svc := range p.Services {
		svc.Image = expand(svc.Image, env)
		svc.Ports = expandAll(svc.Ports, env)
		svc.Volumes = expandAll(svc.Volumes, env)
		out.Services[name] = svc
	}
	return out
}
