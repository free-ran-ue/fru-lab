package dataplane

import (
	"net/netip"
	"time"
)

// Runner is either engine, as a run uses it.
type Runner interface {
	Start() error
	AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort)
	Stop(drain time.Duration)
	Snapshot() Snapshot
}

// Select builds the engine cfg.Engine asks for. EngineAuto uses AF_XDP
// when the N3 and N6 interfaces are NICs and AF_XDP starts on them, and
// the socket engine otherwise, saying why on the Run page.
func Select(cfg Config) Runner {
	switch cfg.Engine {
	case EngineAFXDP:
		return NewXDP(cfg)
	case EngineAuto:
		return &autoEngine{cfg: cfg}
	default:
		return New(cfg)
	}
}

type autoEngine struct {
	cfg Config
	Runner
}

func (a *autoEngine) Start() error {
	reason := ""
	for _, name := range []string{a.cfg.N3Interface, a.cfg.N6Interface} {
		if ok, why := isPhysical(name); !ok && reason == "" {
			reason = why
		}
	}
	if reason == "" {
		x := NewXDP(a.cfg)
		err := x.Start()
		if err == nil {
			a.Runner = x
			return nil
		}
		reason = "AF_XDP did not start: " + err.Error()
	}
	s := New(a.cfg)
	s.engine = "socket (auto: " + reason + ")"
	a.Runner = s
	return s.Start()
}
