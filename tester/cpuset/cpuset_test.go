package cpuset

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAndString(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		"3":             "3",
		"0-3":           "0-3",
		" 8, 0-2 ,1,10": "0-2,8,10",
		"4,5,6,9":       "4-6,9",
	} {
		s, err := Parse(in)
		require.NoError(t, err, in)
		require.Equal(t, want, s.String(), in)
	}
	for _, bad := range []string{"a", "3-1", "-1", "1,,2", "2-x", "99999"} {
		_, err := Parse(bad)
		require.Error(t, err, bad)
	}
}

func TestCheckAllocation(t *testing.T) {
	allowed := Set{0, 1, 2, 3, 4, 5}
	cp, dp, errs := Allocation{CpCpus: "0-1", DpCpus: "2-5"}.Check(allowed)
	require.Empty(t, errs)
	require.Equal(t, Set{0, 1}, cp)
	require.Equal(t, Set{2, 3, 4, 5}, dp)

	_, _, errs = Allocation{}.Check(allowed)
	require.Empty(t, errs, "nothing set: nothing pinned")

	_, _, errs = Allocation{CpCpus: "0-2", DpCpus: "2-7"}.Check(allowed)
	require.Equal(t, []FieldError{{"dpCpus", "CPU 6-7 not available to fru-tester (it may use 0-5)"}}, errs)

	_, _, errs = Allocation{CpCpus: "0-2", DpCpus: "2-5"}.Check(allowed)
	require.Equal(t, []FieldError{{"dpCpus", "CPU 2 is also a control plane CPU"}}, errs)

	_, _, errs = Allocation{CpCpus: "x"}.Check(allowed)
	require.Len(t, errs, 1)
	require.Equal(t, "cpCpus", errs[0].Field)
}

// A pinned goroutine runs only on its CPUs; others are not affected.
func TestPinThreadAndSetProcess(t *testing.T) {
	all := Allowed()
	if len(all) < 2 {
		t.Skip("needs two CPUs")
	}
	one := Set{all[len(all)-1]}
	got := make(chan Set)
	go func() {
		require.NoError(t, PinThread(one))
		s, err := Current()
		require.NoError(t, err)
		got <- s
	}()
	require.Equal(t, one, <-got)

	first := Set{all[0]}
	require.NoError(t, SetProcess(first))
	defer func() { require.NoError(t, SetProcess(all)) }()
	go func() {
		runtime.LockOSThread() // whichever thread this is, it was moved
		s, err := Current()
		require.NoError(t, err)
		got <- s
	}()
	require.Equal(t, first, <-got)
}

func TestStoreKeepsTheAllocation(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "data", "system.json"))
	a, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, Allocation{}, a, "no file yet")
	require.NoError(t, s.Save(Allocation{CpCpus: "0-1", DpCpus: "2-5"}))
	a, err = s.Load()
	require.NoError(t, err)
	require.Equal(t, Allocation{CpCpus: "0-1", DpCpus: "2-5"}, a)
}

func TestReadTopology(t *testing.T) {
	dir := t.TempDir()
	write := func(path, body string) {
		p := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body+"\n"), 0o644))
	}
	write("sys/devices/system/cpu/online", "0-7")
	write("sys/devices/system/node/node0/cpulist", "0-3")
	write("sys/devices/system/node/node1/cpulist", "4-7")
	// a NIC on node 1 with two queues whose IRQs go to CPUs 4 and 5
	write("sys/bus/pci/devices/0000:e1:00.0/numa_node", "1")
	write("sys/bus/pci/devices/0000:e1:00.0/local_cpulist", "4-7")
	write("sys/bus/pci/devices/0000:e1:00.0/msi_irqs/120", "msix")
	write("sys/bus/pci/devices/0000:e1:00.0/msi_irqs/121", "msix")
	write("sys/bus/pci/drivers/i40e/x", "")
	write("proc/irq/120/effective_affinity_list", "4")
	write("proc/irq/121/effective_affinity_list", "5")
	write("sys/class/net/eth1/queues/rx-0/x", "")
	write("sys/class/net/eth1/queues/rx-1/x", "")
	require.NoError(t, os.Symlink(filepath.Join(dir, "sys/bus/pci/devices/0000:e1:00.0"), filepath.Join(dir, "sys/class/net/eth1/device")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "sys/bus/pci/drivers/i40e"), filepath.Join(dir, "sys/bus/pci/devices/0000:e1:00.0/driver")))
	write("sys/class/net/veth0/queues/rx-0/x", "") // virtual: no device

	old := root
	root = dir
	defer func() { root = old }()
	topo := ReadTopology()
	require.Equal(t, "0-7", topo.Online)
	require.Equal(t, []Node{{0, "0-3"}, {1, "4-7"}}, topo.Nodes)
	require.Equal(t, []NicLayout{{Name: "eth1", Driver: "i40e", NumaNode: 1, LocalCpus: "4-7", Queues: 2, IrqCpus: "4-5"}}, topo.Interfaces)
}
