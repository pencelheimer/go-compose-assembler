package validate

import (
	"strconv"
	"strings"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// PortsValidator reports host ports published by more than one binding.
type PortsValidator struct{}

func (PortsValidator) Name() string { return "ports" }

type binding struct {
	service string
	ip      string
}

func (PortsValidator) Validate(p *domain.Project) []error {
	var errList []error
	seen := map[string][]binding{} // "8080/tcp" -> bindings
	for _, name := range sortedServiceNames(p) {
		for _, spec := range p.Services[name].Ports {
			ip, hostPorts, proto, ok := parsePort(spec)
			if !ok {
				errList = append(errList, errs.InvalidRequestError{
					Reason: "service " + name + ": unparseable port mapping " + strconv.Quote(spec),
				})
				continue
			}
			for _, hp := range hostPorts {
				key := strconv.Itoa(hp) + "/" + proto
				for _, other := range seen[key] {
					if ipsOverlap(other.ip, ip) {
						errList = append(errList, errs.PortConflictError{Port: key, ServiceA: other.service, ServiceB: name})
						break
					}
				}
				seen[key] = append(seen[key], binding{service: name, ip: ip})
			}
		}
	}
	return errList
}

func ipsOverlap(a, b string) bool {
	wild := func(s string) bool { return s == "" || s == "0.0.0.0" || s == "::" }
	return wild(a) || wild(b) || a == b
}

// parsePort understands the short syntax "[ip:][host[-end]:]container[-end][/proto]".
// A mapping without a host part publishes an ephemeral port and yields no host ports.
func parsePort(spec string) (ip string, hostPorts []int, proto string, ok bool) {
	proto = "tcp"
	if i := strings.LastIndex(spec, "/"); i >= 0 {
		proto = strings.ToLower(spec[i+1:])
		spec = spec[:i]
		if proto == "" {
			return "", nil, "", false
		}
	}
	if strings.HasPrefix(spec, "[") {
		end := strings.Index(spec, "]")
		if end < 0 || len(spec) <= end+1 || spec[end+1] != ':' {
			return "", nil, "", false
		}
		ip = spec[1:end]
		spec = spec[end+2:]
	}
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case 1:
		_, _, ok := portRange(parts[0])
		return "", nil, proto, ok
	case 2:
	case 3:
		if ip != "" {
			return "", nil, "", false
		}
		ip, parts = parts[0], parts[1:]
	default:
		return "", nil, "", false
	}
	lo, hi, ok := portRange(parts[0])
	if !ok {
		return "", nil, "", false
	}
	if _, _, ok := portRange(parts[1]); !ok {
		return "", nil, "", false
	}
	for hp := lo; hp <= hi; hp++ {
		hostPorts = append(hostPorts, hp)
	}
	return ip, hostPorts, proto, true
}

func portRange(s string) (lo, hi int, ok bool) {
	loS, hiS, isRange := strings.Cut(s, "-")
	lo, err := strconv.Atoi(loS)
	if err != nil || lo < 1 || lo > 65535 {
		return 0, 0, false
	}
	if !isRange {
		return lo, lo, true
	}
	hi, err = strconv.Atoi(hiS)
	if err != nil || hi < lo || hi > 65535 {
		return 0, 0, false
	}
	return lo, hi, true
}
