// Package cpuset keeps the control plane and the data plane on CPUs the
// user chose (the Status page): the data plane's long-lived goroutines
// each lock an OS thread pinned to the data plane CPUs, and the rest of
// the process is held to the control plane CPUs for the run.
package cpuset

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// maxCPUs is how many CPUs unix.CPUSet holds.
const maxCPUs = len(unix.CPUSet{}) * 64

// Set is a sorted list of CPU numbers without duplicates.
type Set []int

// Parse reads a Linux CPU list such as "0-3,8,10-11"; "" is the empty set.
func Parse(s string) (Set, error) {
	var out Set
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil || a < 0 {
			return nil, fmt.Errorf("%q is not a CPU number or range", part)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil || b < a {
				return nil, fmt.Errorf("%q is not a CPU range", part)
			}
		}
		if b >= maxCPUs {
			return nil, fmt.Errorf("CPU %d is out of range", b)
		}
		for c := a; c <= b; c++ {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// String is the set in Linux CPU list form, e.g. "0-3,8".
func (s Set) String() string {
	var parts []string
	for i := 0; i < len(s); {
		j := i
		for j+1 < len(s) && s[j+1] == s[j]+1 {
			j++
		}
		if j == i {
			parts = append(parts, strconv.Itoa(s[i]))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", s[i], s[j]))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// Minus is the CPUs of s not in o.
func (s Set) Minus(o Set) Set {
	var out Set
	for _, c := range s {
		if !slices.Contains(o, c) {
			out = append(out, c)
		}
	}
	return out
}

// Overlap is the CPUs in both s and o.
func (s Set) Overlap(o Set) Set {
	var out Set
	for _, c := range s {
		if slices.Contains(o, c) {
			out = append(out, c)
		}
	}
	return out
}

func (s Set) mask() *unix.CPUSet {
	var m unix.CPUSet
	for _, c := range s {
		m.Set(c)
	}
	return &m
}

func fromMask(m *unix.CPUSet) Set {
	var out Set
	for c := range maxCPUs {
		if m.IsSet(c) {
			out = append(out, c)
		}
	}
	return out
}

// startup is the CPUs the process could use when it started, before any
// run narrowed it (in a container, its cpuset).
var startup = func() Set {
	var m unix.CPUSet
	if err := unix.SchedGetaffinity(0, &m); err != nil {
		return nil
	}
	return fromMask(&m)
}()

// Allowed is the CPUs this process may be given.
func Allowed() Set { return slices.Clone(startup) }

// Current is the calling thread's CPUs.
func Current() (Set, error) {
	var m unix.CPUSet
	if err := unix.SchedGetaffinity(0, &m); err != nil {
		return nil, err
	}
	return fromMask(&m), nil
}

// PinThread locks the calling goroutine to its OS thread and keeps that
// thread on s. It never unlocks: when the goroutine returns, Go ends the
// thread, so the pinning cannot leak to other goroutines.
func PinThread(s Set) error {
	if len(s) == 0 {
		return nil
	}
	runtime.LockOSThread()
	return unix.SchedSetaffinity(0, s.mask())
}

// SetProcess moves every thread of the process to s; threads started
// later inherit it. Threads that exit meanwhile are skipped.
func SetProcess(s Set) error {
	if len(s) == 0 {
		return nil
	}
	tasks, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return err
	}
	var errs []error
	for _, t := range tasks {
		tid, err := strconv.Atoi(t.Name())
		if err != nil {
			continue
		}
		if err := unix.SchedSetaffinity(tid, s.mask()); err != nil && !errors.Is(err, unix.ESRCH) {
			errs = append(errs, fmt.Errorf("thread %d: %w", tid, err))
		}
	}
	return errors.Join(errs...)
}

// Allocation is where the control plane and the data plane run; an empty
// list leaves that side unrestricted.
type Allocation struct {
	CpCpus string `json:"cpCpus"`
	DpCpus string `json:"dpCpus"`
}

// FieldError is a problem with one Allocation field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Check parses a and checks it against allowed: every CPU must be one the
// process may use, and the two sides must not share any.
func (a Allocation) Check(allowed Set) (cp, dp Set, errs []FieldError) {
	parse := func(field, v string) Set {
		s, err := Parse(v)
		if err != nil {
			errs = append(errs, FieldError{field, err.Error()})
			return nil
		}
		if bad := s.Minus(allowed); len(bad) > 0 {
			errs = append(errs, FieldError{field, fmt.Sprintf("CPU %s not available to fru-tester (it may use %s)", bad, allowed)})
		}
		return s
	}
	cp, dp = parse("cpCpus", a.CpCpus), parse("dpCpus", a.DpCpus)
	if both := cp.Overlap(dp); len(errs) == 0 && len(both) > 0 {
		errs = append(errs, FieldError{"dpCpus", fmt.Sprintf("CPU %s is also a control plane CPU", both)})
	}
	return cp, dp, errs
}

// Canonical rewrites both lists in canonical form.
func (a Allocation) Canonical() Allocation {
	cp, _ := Parse(a.CpCpus)
	dp, _ := Parse(a.DpCpus)
	return Allocation{CpCpus: cp.String(), DpCpus: dp.String()}
}
