package run

import (
	"errors"
	"fmt"

	"tester/cpuset"
)

// SystemInfo is the Status page's CPU part: the CPU allocation and the host layout
// that helps choose it.
type SystemInfo struct {
	Cpus     cpuset.Allocation `json:"cpus"`
	Topology cpuset.Topology   `json:"topology"`
}

// CpuAllocationError lists what is wrong with an allocation.
type CpuAllocationError struct{ Errors []cpuset.FieldError }

func (e *CpuAllocationError) Error() string {
	return fmt.Sprintf("invalid CPU allocation: %s: %s", e.Errors[0].Field, e.Errors[0].Message)
}

// ErrNoCpuStore means this controller was built without a place to keep the
// allocation.
var ErrNoCpuStore = errors.New("CPU allocation is not supported here")

// System returns the stored allocation and the host's CPU layout.
func (c *Controller) System() (SystemInfo, error) {
	info := SystemInfo{Topology: cpuset.ReadTopology()}
	if c.deps.Cpus == nil {
		return info, nil
	}
	a, err := c.deps.Cpus.Load()
	info.Cpus = a
	return info, err
}

// SetCpus checks and stores the allocation the next run uses; it is
// refused while a run is active (design: allocation is fixed before a
// run, never changed during one).
func (c *Controller) SetCpus(a cpuset.Allocation) (cpuset.Allocation, error) {
	if c.deps.Cpus == nil {
		return a, ErrNoCpuStore
	}
	c.mu.Lock()
	active := c.current != nil && !c.current.state().Finished()
	c.mu.Unlock()
	if active {
		return a, ErrRunActive
	}
	if _, _, errs := a.Check(c.deps.AllowedCpus()); len(errs) > 0 {
		return a, &CpuAllocationError{Errors: errs}
	}
	a = a.Canonical()
	return a, c.deps.Cpus.Save(a)
}

// allocation is the stored allocation, checked again: the CPUs the
// process may use can change between saving and running (a container's
// cpuset, CPUs taken offline).
func (c *Controller) allocation() (cpuset.Allocation, cpuset.Set, cpuset.Set, error) {
	if c.deps.Cpus == nil {
		return cpuset.Allocation{}, nil, nil, nil
	}
	a, err := c.deps.Cpus.Load()
	if err != nil {
		return a, nil, nil, fmt.Errorf("read the CPU allocation: %w", err)
	}
	cp, dp, errs := a.Check(c.deps.AllowedCpus())
	if len(errs) > 0 {
		return a, nil, nil, &CpuAllocationError{Errors: errs}
	}
	return a, cp, dp, nil
}
