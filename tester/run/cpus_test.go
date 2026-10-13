package run

import (
	"sync"
	"testing"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

	"tester/cpuset"
	"tester/dataplane"
)

type memCpus struct {
	mu sync.Mutex
	a  cpuset.Allocation
}

func (m *memCpus) Load() (cpuset.Allocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.a, nil
}
func (m *memCpus) Save(a cpuset.Allocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.a = a
	return nil
}

// cpuController is an e2e controller whose CPU moves are recorded, on a
// host with CPUs 0-7.
func cpuController(store *memCpus) (*Controller, *fakeDataplane, func() []cpuset.Set) {
	var mu sync.Mutex
	var moves []cpuset.Set
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	dp := &fakeDataplane{ues: map[int]fakeFlow{}}
	c := NewController(Deps{Addrs: &fakeAddrs{}, Dialer: amfDialer{amf: newFakeAMF(nil)}, Log: lg.WithTags("TEST"),
		NewID: func() string { return "cpu" }, Cpus: store,
		NewDataplane:   func(cfg dataplane.Config) Dataplane { dp.cfg = cfg; return dp },
		AllowedCpus:    func() cpuset.Set { return cpuset.Set{0, 1, 2, 3, 4, 5, 6, 7} },
		SetProcessCpus: func(s cpuset.Set) error { mu.Lock(); moves = append(moves, s); mu.Unlock(); return nil }})
	return c, dp, func() []cpuset.Set { mu.Lock(); defer mu.Unlock(); return append([]cpuset.Set(nil), moves...) }
}

func TestSetCpusChecksAndStores(t *testing.T) {
	store := &memCpus{}
	c, _, _ := cpuController(store)
	a, err := c.SetCpus(cpuset.Allocation{CpCpus: "1,0", DpCpus: "2,3,4,5"})
	require.NoError(t, err)
	require.Equal(t, cpuset.Allocation{CpCpus: "0-1", DpCpus: "2-5"}, a, "stored in canonical form")
	require.Equal(t, a, store.a)

	_, err = c.SetCpus(cpuset.Allocation{CpCpus: "0-3", DpCpus: "3-9"})
	var aerr *CpuAllocationError
	require.ErrorAs(t, err, &aerr)
	require.Equal(t, "dpCpus", aerr.Errors[0].Field)
	require.Equal(t, cpuset.Allocation{CpCpus: "0-1", DpCpus: "2-5"}, store.a, "unchanged")

	info, err := c.System()
	require.NoError(t, err)
	require.Equal(t, store.a, info.Cpus)
	require.NotEmpty(t, info.Topology.Online)
}

// A run keeps the process on the control plane CPUs and gives the data
// plane its CPUs (one sender and one reader each); after it, the process
// gets every CPU back. The allocation cannot change during the run.
func TestRunUsesTheCpuAllocation(t *testing.T) {
	store := &memCpus{a: cpuset.Allocation{CpCpus: "0-1", DpCpus: "2-5"}}
	c, dp, moves := cpuController(store)
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Equal(t, store.a, snap.Cpus)
	require.Equal(t, cpuset.Set{2, 3, 4, 5}, dp.cfg.Cpus)
	require.Equal(t, 4, dp.cfg.Senders)
	require.Equal(t, 4, dp.cfg.Receivers)
	require.Equal(t, []cpuset.Set{{0, 1}}, moves())

	_, err = c.SetCpus(cpuset.Allocation{CpCpus: "0"})
	require.ErrorIs(t, err, ErrRunActive)

	stopAndWait(t, c)
	require.Equal(t, []cpuset.Set{{0, 1}, {0, 1, 2, 3, 4, 5, 6, 7}}, moves())
}

func TestNoAllocationMeansNoPinning(t *testing.T) {
	c, dp, moves := cpuController(&memCpus{})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Empty(t, dp.cfg.Cpus)
	require.Zero(t, dp.cfg.Senders, "the engine's default: one per CPU")
	stopAndWait(t, c)
	require.Empty(t, moves())
}

// CPUs the process may no longer use (e.g. a smaller container cpuset)
// stop the run from starting, with the reason.
func TestStaleAllocationRefusesToStart(t *testing.T) {
	c, _, _ := cpuController(&memCpus{a: cpuset.Allocation{CpCpus: "0", DpCpus: "8-9"}})
	_, err := c.Start(e2eProfile())
	var aerr *CpuAllocationError
	require.ErrorAs(t, err, &aerr)
	require.Contains(t, err.Error(), "dpCpus")
}
