package processor

import (
	"backend/model"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var (
	reCoreName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,31}$`)
	reDnn      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,62}$`)
	reIfName   = regexp.MustCompile(`^[A-Za-z0-9@._:-]{1,15}$`)
)

func isIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// normalizeCoreNetwork trims n and says what is malformed, if anything.
func normalizeCoreNetwork(n *model.CoreNetwork) *model.ErrorDetail {
	bad := func(format string, args ...any) *model.ErrorDetail {
		return &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: fmt.Sprintf(format, args...)}
	}
	n.Name = strings.TrimSpace(n.Name)
	n.N2Ip, n.N3Ip = strings.TrimSpace(n.N2Ip), strings.TrimSpace(n.N3Ip)
	n.N2Interface, n.N3Interface = strings.TrimSpace(n.N2Interface), strings.TrimSpace(n.N3Interface)
	switch {
	case !reCoreName.MatchString(n.Name):
		return bad("name must be 1-32 letters, digits, spaces, dots, dashes or underscores, starting with a letter or digit")
	case !isIPv4(n.N2Ip):
		return bad("n2Ip must be an IPv4 address")
	case !isIPv4(n.N3Ip):
		return bad("n3Ip must be an IPv4 address")
	case n.N2Interface != "" && !reIfName.MatchString(n.N2Interface):
		return bad("n2Interface is not an interface name")
	case n.N3Interface != "" && !reIfName.MatchString(n.N3Interface):
		return bad("n3Interface is not an interface name")
	case len(n.Dns) == 0:
		return bad("add at least one data network")
	}
	seen := map[string]bool{}
	for i := range n.Dns {
		dn := &n.Dns[i]
		dn.Name, dn.Cidr = strings.TrimSpace(dn.Name), strings.TrimSpace(dn.Cidr)
		if !reDnn.MatchString(dn.Name) {
			return bad("data network %d: the DNN must be letters, digits, dots or dashes", i+1)
		}
		if seen[strings.ToLower(dn.Name)] {
			return bad("data network %s is listed twice", dn.Name)
		}
		seen[strings.ToLower(dn.Name)] = true
		if dn.Cidr != "" {
			if _, _, err := net.ParseCIDR(dn.Cidr); err != nil {
				return bad("data network %s: %s is not an address range like 10.60.0.0/16", dn.Name, dn.Cidr)
			}
		}
	}
	return nil
}

// coreNameTaken says whether name clashes with the lab core or another
// saved core network (names are compared ignoring case).
func coreNameTaken(name, id string, others []model.CoreNetwork) string {
	if strings.EqualFold(name, model.LabCoreName) {
		return fmt.Sprintf("%s is the lab core's name", model.LabCoreName)
	}
	for _, o := range others {
		if o.ID != id && strings.EqualFold(o.Name, name) {
			return fmt.Sprintf("another core network is already named %s", o.Name)
		}
	}
	return ""
}

func (p *Processor) ListCoreNetworks() ([]model.CoreNetwork, *model.ErrorDetail) {
	list, err := p.FlContext.ListCoreNetworks()
	if err != nil {
		return nil, internal(err)
	}
	return list, nil
}

func (p *Processor) SaveCoreNetwork(id string, n *model.CoreNetwork) (*model.CoreNetwork, *model.ErrorDetail) {
	if id != "" {
		existing, err := p.FlContext.GetCoreNetwork(id)
		if err != nil {
			return nil, internal(err)
		}
		if existing == nil {
			return nil, notFound("core network", id)
		}
	}
	n.ID = id
	if detail := normalizeCoreNetwork(n); detail != nil {
		return nil, detail
	}
	others, err := p.FlContext.ListCoreNetworks()
	if err != nil {
		return nil, internal(err)
	}
	if msg := coreNameTaken(n.Name, n.ID, others); msg != "" {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: msg}
	}
	if err := p.FlContext.SaveCoreNetwork(n); err != nil {
		return nil, internal(err)
	}
	return n, nil
}

func (p *Processor) DeleteCoreNetwork(id string) *model.ErrorDetail {
	if err := p.FlContext.DeleteCoreNetwork(id); err != nil {
		return internal(err)
	}
	return nil
}
