package data

import (
	"fmt"
	"strconv"
	"strings"
)

// ProxyTypes are the portproxy types netsh and the reference app understand.
var ProxyTypes = []string{"v4tov4", "v4tov6", "v6tov4", "v6tov6"}

// Rule is one port forwarding rule, matching the "rules" table of
// PortProxyGUI's config.db exactly.
type Rule struct {
	Id          string
	Type        string
	ListenOn    string
	ListenPort  int
	ConnectTo   string
	ConnectPort int
	Comment     string
	Group       string
}

func (r Rule) Valid() bool { return r.ListenPort > 0 && r.ConnectPort > 0 }

// EqualsKeys compares the natural key of a rule: Type + ListenOn + ListenPort.
func (r Rule) EqualsKeys(o Rule) bool {
	return r.Type == o.Type && r.ListenOn == o.ListenOn && r.ListenPort == o.ListenPort
}

func (r Rule) Equal(o Rule) bool {
	return r.Id == o.Id && r.EqualsKeys(o) && r.ConnectTo == o.ConnectTo &&
		r.ConnectPort == o.ConnectPort && r.Comment == o.Comment && r.Group == o.Group
}

func IsProxyType(s string) bool {
	for _, t := range ProxyTypes {
		if t == s {
			return true
		}
	}
	return false
}

// ParsePort accepts the same port strings as PortProxyGUI.Rule.ParsePort:
// an integer in (0, 65536).
func ParsePort(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 || n >= 65536 {
		return 0, fmt.Errorf("invalid port string (%s)", s)
	}
	return n, nil
}
