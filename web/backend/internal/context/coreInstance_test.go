package context

import (
	"backend/constant"
	"backend/model"
	"context"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestIsCoreInstance(t *testing.T) {
	for target, want := range map[string]bool{
		"core-3f2a9c1e5b7d":  true,
		"core-3F2A9C1E5B7D":  false,
		"core-":              false,
		"core-../../etc/xyz": false,
		"free5gc":            false,
		"gnb-3f2a9c1e5b7d":   false,
	} {
		if IsCoreInstance(target) != want {
			t.Errorf("%s: want %v", target, want)
		}
	}
}

var (
	reAnyContainerName = regexp.MustCompile(`(?m)^\s*container_name:\s*(\S+)`)
	reAnyBridgeName    = regexp.MustCompile(`com\.docker\.network\.bridge\.name\s*[:=]\s*([A-Za-z0-9_.-]+)`)
	reLabAddress       = regexp.MustCompile(`(^|[^0-9])10\.0\.(1|10)\.`)
)

// Every file of both templates, moved into a slot, keeps nothing that is
// global to the host from the lab free5GC: no lab address, every
// container name and bridge name carries the slot.
func TestEveryTemplateFileMovesIntoTheSlot(t *testing.T) {
	for _, variant := range []string{constant.FREE5GC_TEMPLATE_BASIC, constant.FREE5GC_TEMPLATE_ULCL} {
		tmpl := defaultTargets()[constant.DEPLOY_TARGET_FREE5GC].template(variant)
		dir := t.TempDir()
		if err := materializeCoreFiles(dir, tmpl, coreCnRanPool("abcdefabcdef", 9), coreSbiPool("abcdefabcdef", 9)); err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
		files := 0
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files++
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(raw)
			if reLabAddress.MatchString(s) {
				t.Errorf("%s %s: a lab address is left", variant, path)
			}
			if strings.Contains(s, "frulab-cn-ran\n") || strings.Contains(s, "frulab-cn-ran ") {
				t.Errorf("%s %s: the lab cn-ran network is left", variant, path)
			}
			for _, m := range reAnyContainerName.FindAllStringSubmatch(s, -1) {
				if !strings.HasPrefix(m[1], "c9-") {
					t.Errorf("%s %s: container %s", variant, path, m[1])
				}
			}
			for _, m := range reAnyBridgeName.FindAllStringSubmatch(s, -1) {
				if !strings.HasSuffix(m[1], "9") || len(m[1]) > 15 {
					t.Errorf("%s %s: bridge %s", variant, path, m[1])
				}
			}
			if filepath.Ext(path) == ".sh" {
				if info, _ := d.Info(); info.Mode()&0100 == 0 {
					t.Errorf("%s %s: not executable", variant, path)
				}
			}
			return nil
		})
		if err != nil || files < 30 {
			t.Fatalf("%s: %v, %d files", variant, err, files)
		}
	}
}

func TestUpCoreInstance(t *testing.T) {
	for _, variant := range []string{constant.FREE5GC_TEMPLATE_BASIC, constant.FREE5GC_TEMPLATE_ULCL} {
		fake := &fakeCompose{}
		c := newTestCompose(t, fake)
		var created []sharedNetwork
		c.createNetworkFn = func(_ context.Context, n sharedNetwork) error {
			created = append(created, n)
			return nil
		}
		const id = "abcdefabcdef"
		if err := c.UpCoreInstance(context.Background(), id, 2, 5002, variant); err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
		if len(created) != 1 || created[0] != (sharedNetwork{name: "frulab-cn-ran-2", subnet: "10.0.102.0/24", gateway: "10.0.102.1", bridgeName: "docker-cnran2"}) {
			t.Fatalf("%s: networks %+v", variant, created)
		}
		p := fake.upped
		if p.Name != "frulab-core-abcdefabcdef" {
			t.Fatalf("%s: project %s", variant, p.Name)
		}
		for name, svc := range p.Services {
			if !strings.HasPrefix(svc.ContainerName, "c2-") {
				t.Errorf("%s: service %s container %s", variant, name, svc.ContainerName)
			}
		}
		if n := p.Services["amf"].Networks["cn-ran"]; n == nil || n.Ipv4Address != "10.0.102.3" {
			t.Fatalf("%s: amf cn-ran %+v", variant, n)
		}
		upf := "upf"
		if variant == constant.FREE5GC_TEMPLATE_ULCL {
			upf = "i-upf"
		}
		if n := p.Services[upf].Networks["cn-ran"]; n == nil || n.Ipv4Address != "10.0.102.5" {
			t.Fatalf("%s: upf cn-ran %+v", variant, n)
		}
		if p.Networks["cn-ran"].Name != "frulab-cn-ran-2" || !bool(p.Networks["cn-ran"].External) {
			t.Fatalf("%s: cn-ran %+v", variant, p.Networks["cn-ran"])
		}
		sbi := p.Networks["sbi"]
		if sbi.Ipam.Config[0].Subnet != "10.0.112.0/24" || sbi.DriverOpts["com.docker.network.bridge.name"] != "docker-sbi2" {
			t.Fatalf("%s: sbi %+v", variant, sbi)
		}
		wc := p.Services["webconsole"]
		if len(wc.Ports) != 1 || wc.Ports[0].Published != "5002" || wc.Ports[0].Target != 5000 {
			t.Fatalf("%s: webconsole ports %+v", variant, wc.Ports)
		}
		amfcfg, _ := os.ReadFile(filepath.Join(c.workDir, "core-"+id, "config", "amfcfg.yaml"))
		smfcfg, _ := os.ReadFile(filepath.Join(c.workDir, "core-"+id, "config", "smfcfg.yaml"))
		if !strings.Contains(string(amfcfg), "10.0.102.3") || !strings.Contains(string(smfcfg), "10.0.102.5") {
			t.Fatalf("%s: configs not moved:\n%s\n%s", variant, amfcfg, smfcfg)
		}
		if got := c.CoreInstanceTemplate(id); got != variant {
			t.Fatalf("%s: recorded template %q", variant, got)
		}

		// loaded again (status / logs / down) it keeps its webconsole port
		again, err := c.loadProject(context.Background(), "core-"+id, "", false)
		if err != nil || again.Services["webconsole"].Ports[0].Published != "5002" {
			t.Fatalf("%s: reload %v", variant, err)
		}
		// down stops it and leaves its files (listed as stopped)
		if err := c.Down(context.Background(), "core-"+id); err != nil {
			t.Fatal(err)
		}
		if len(fake.downs) != 1 || fake.downs[0] != "frulab-core-abcdefabcdef" || !c.coreInstanceMaterialized("core-"+id) {
			t.Fatalf("%s: downs %v", variant, fake.downs)
		}
		c.RemoveCoreInstanceDir(id)
		if c.coreInstanceMaterialized("core-" + id) {
			t.Fatal("working dir left after remove")
		}
	}
}

func TestCoreInstanceNeverDeployedIsStopped(t *testing.T) {
	c := newTestCompose(t, &fakeCompose{})
	res, err := c.Status(context.Background(), "core-abcdefabcdef")
	if err != nil || len(res.Services) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if c.CoreInstanceTemplate("abcdefabcdef") != "" {
		t.Fatal("a template for a core never deployed")
	}
	if err := c.UpCoreInstance(context.Background(), "abcdefabcdef", 10, 5010, "basic"); err == nil {
		t.Fatal("slot 10 accepted")
	}
}

func TestCoreCnRanNetworks(t *testing.T) {
	got := coreCnRanNetworks([]string{"frulab-cn-ran", "frulab-cn-ran-3", "frulab-cn-ran-10", "frulab-cn-ran-x", "frulab-cn-ran-1", "frulab-cn-ran-3", "other"})
	if len(got) != 2 || got[0].name != "frulab-cn-ran-3" || got[0].bridgeName != "docker-cnran3" || got[1].name != "frulab-cn-ran-1" {
		t.Fatalf("got %+v", got)
	}
}

// A lab-type gNB on a free5gc core joins that core's cn-ran network at an
// assigned address and dials that core's AMF / UPF; the next one gets the
// next address.
func TestUpGnbInstanceOnCore(t *testing.T) {
	fake := &fakeCompose{}
	c := newTestCompose(t, fake)
	c.inspectNetworkFn = func(_ context.Context, name string) (map[netip.Addr]string, error) {
		if name == "frulab-cn-ran-2" {
			return map[netip.Addr]string{netip.MustParseAddr("10.0.102.3"): "c2-amf", netip.MustParseAddr("10.0.102.20"): "squatter"}, nil
		}
		return map[netip.Addr]string{}, nil
	}
	on := CoreCnRan("abcdefabcdef", 2)
	p := DefaultGnbProfile()
	p.ID, p.CoreNetwork, p.N2Ip = "aaaaaaaaaaaa", "abcdefabcdef", "10.0.1.99"
	d, err := c.UpGnbInstance(context.Background(), &p, &on)
	if err != nil {
		t.Fatal(err)
	}
	if d.N2Ip != "10.0.102.21" || d.AmfIp != "10.0.102.3" || d.CoreNetwork != "abcdefabcdef" || d.RanIp != "10.0.2.200" || d.Core != model.GnbCoreLab {
		t.Fatalf("deployment %+v", d)
	}
	if p.N2Ip != "10.0.1.99" {
		t.Fatal("the caller's profile was changed")
	}
	svc := fake.upped.Services["gnb"]
	if n := svc.Networks["cn-ran"]; n == nil || n.Ipv4Address != "10.0.102.21" {
		t.Fatalf("cn-ran %+v", svc.Networks["cn-ran"])
	}
	if fake.upped.Networks["cn-ran"].Name != "frulab-cn-ran-2" || fake.upped.Networks["ran-ue"].Name != RanUeNetwork {
		t.Fatalf("networks %+v", fake.upped.Networks)
	}
	cfg, _ := os.ReadFile(filepath.Join(c.workDir, "gnb-aaaaaaaaaaaa", "config", "gnbcfg.yaml"))
	for _, want := range []string{`amfN2Ip: "10.0.102.3"`, `upfN3Ip: "10.0.102.5"`, `ranN2Ip: "10.0.102.21"`} {
		if !strings.Contains(string(cfg), want) {
			t.Fatalf("missing %s in\n%s", want, cfg)
		}
	}

	q := DefaultGnbProfile()
	q.ID, q.GnbID = "bbbbbbbbbbbb", "000315"
	d2, err := c.UpGnbInstance(context.Background(), &q, &on)
	if err != nil || d2.N2Ip != "10.0.102.22" || d2.RanIp != "10.0.2.201" {
		t.Fatalf("second gNB %+v %v", d2, err)
	}

	// on the lab free5GC (no cn-ran given) it is the lab's network
	r := DefaultGnbProfile()
	r.ID, r.GnbID = "cccccccccccc", "000316"
	d3, err := c.UpGnbInstance(context.Background(), &r, nil)
	if err != nil || d3.N2Ip != "10.0.1.20" || d3.AmfIp != "10.0.1.3" || d3.CoreNetwork != "" || fake.upped.Networks["cn-ran"].Name != CnRanNetwork {
		t.Fatalf("lab gNB %+v %v", d3, err)
	}

	// stopped and deployed again, a gNB gets the same addresses back, even
	// with lower ones free (the squatter left)
	if err := c.DownGnbInstance(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	c.inspectNetworkFn = func(context.Context, string) (map[netip.Addr]string, error) { return map[netip.Addr]string{}, nil }
	d4, err := c.UpGnbInstance(context.Background(), &q, &on)
	if err != nil || d4.N2Ip != "10.0.102.22" || d4.RanIp != "10.0.2.201" {
		t.Fatalf("redeploy %+v %v", d4, err)
	}
	// moved to the lab core it keeps its ran-ue address and gets a lab N2
	// one; its lease on the core is given up
	d5, err := c.UpGnbInstance(context.Background(), &q, nil)
	if err != nil || d5.N2Ip != "10.0.1.21" || d5.RanIp != "10.0.2.201" {
		t.Fatalf("moved %+v %v", d5, err)
	}
	if l, ok, _ := c.ipam.Lookup("gnb-bbbbbbbbbbbb", LeaseRoleN2); !ok || l.Pool != PoolCnRan {
		t.Fatalf("lease %+v", l)
	}
}
