package cpuset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Topology is what the System page shows to help choose CPUs.
type Topology struct {
	Online     string      `json:"online"`  // CPUs the kernel runs
	Allowed    string      `json:"allowed"` // of those, the ones fru-tester may use
	Nodes      []Node      `json:"nodes"`
	Interfaces []NicLayout `json:"interfaces"`
}

// Node is one NUMA node.
type Node struct {
	Node int    `json:"node"`
	Cpus string `json:"cpus"`
}

// NicLayout places one physical NIC: its NUMA node (-1 when the kernel
// does not say), the CPUs close to it, how many receive queues it has,
// and which CPUs its interrupts are delivered to.
type NicLayout struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	NumaNode  int    `json:"numaNode"`
	LocalCpus string `json:"localCpus"`
	Queues    int    `json:"queues"`
	IrqCpus   string `json:"irqCpus"`
}

// root is where /sys and /proc are read from; tests point it elsewhere.
var root = "/"

func readTrim(path string) string {
	b, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ReadTopology reads the host's CPU and NIC layout from /sys and /proc.
func ReadTopology() Topology {
	t := Topology{Online: readTrim("sys/devices/system/cpu/online"), Allowed: Allowed().String(),
		Nodes: []Node{}, Interfaces: []NicLayout{}}
	nodes, _ := filepath.Glob(filepath.Join(root, "sys/devices/system/node/node[0-9]*"))
	for _, n := range nodes {
		id, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(n), "node"))
		if err != nil {
			continue
		}
		cpus, _ := Parse(readTrim(filepath.Join(strings.TrimPrefix(n, root), "cpulist")))
		t.Nodes = append(t.Nodes, Node{Node: id, Cpus: cpus.String()})
	}
	sort.Slice(t.Nodes, func(i, j int) bool { return t.Nodes[i].Node < t.Nodes[j].Node })

	nics, _ := os.ReadDir(filepath.Join(root, "sys/class/net"))
	for _, e := range nics {
		dev := filepath.Join("sys/class/net", e.Name(), "device")
		if _, err := os.Stat(filepath.Join(root, dev)); err != nil {
			continue // virtual: veth, bridge, lo
		}
		l := NicLayout{Name: e.Name(), NumaNode: -1}
		if drv, err := os.Readlink(filepath.Join(root, dev, "driver")); err == nil {
			l.Driver = filepath.Base(drv)
		}
		if n, err := strconv.Atoi(readTrim(filepath.Join(dev, "numa_node"))); err == nil {
			l.NumaNode = n
		}
		local, _ := Parse(readTrim(filepath.Join(dev, "local_cpulist")))
		l.LocalCpus = local.String()
		queues, _ := filepath.Glob(filepath.Join(root, "sys/class/net", e.Name(), "queues", "rx-*"))
		l.Queues = len(queues)
		l.IrqCpus = irqCpus(dev).String()
		t.Interfaces = append(t.Interfaces, l)
	}
	return t
}

// irqCpus is the union of the CPUs a device's MSI interrupts go to.
func irqCpus(dev string) Set {
	irqs, _ := os.ReadDir(filepath.Join(root, dev, "msi_irqs"))
	var all Set
	for _, irq := range irqs {
		list := readTrim(filepath.Join("proc/irq", irq.Name(), "effective_affinity_list"))
		if list == "" {
			list = readTrim(filepath.Join("proc/irq", irq.Name(), "smp_affinity_list"))
		}
		s, _ := Parse(list)
		all = append(all, s...)
	}
	all, _ = Parse(all.String()) // sort and dedupe
	return all
}

// Store keeps the allocation in a JSON file on the tester host, so it
// survives restarts; a missing file means no allocation.
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() (Allocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var a Allocation
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return a, nil
	}
	if err != nil {
		return a, err
	}
	return a, json.Unmarshal(b, &a)
}

func (s *Store) Save(a Allocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.MarshalIndent(a, "", "  ")
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
