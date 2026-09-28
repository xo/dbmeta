package container_test

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// TestEveryMachineIsUsable checks what dbrun needs from every machine,
// whichever kind it is.
func TestEveryMachineIsUsable(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	ports := map[int]string{}
	for _, s := range container.All() {
		// A machine and a container share one name space, which is how
		// sqlserver-2016 and sqlserver-2017 sit in one list. A name in both
		// would make a selector mean two things.
		names[s.Name()] = true
	}
	// A machine must not collide with a container either. dbrun publishes
	// the container at index i on port 55000 + i, its basePort, and nothing
	// but this keeps the machines out of that range. The port inside a
	// container is not on the host, so Typesense's 8108 and the viewer of
	// sqlserver-2014 on 8108 do not collide.
	const basePort = 55000
	published := func(p int) bool { return p >= basePort && p < basePort+len(container.All()) }
	for _, m := range container.Machines() {
		if names[m.Name()] {
			t.Errorf("%s is listed twice, or is both a machine and a container", m.Name())
		}
		names[m.Name()] = true
		if m.Dialect == "" || m.Product == "" || m.Release == "" {
			t.Errorf("%s is missing its dialect, product or release: %+v", m.Name(), m)
		}
		// A virtual machine cannot run in CI, so it is Verified and never
		// Tested. Saying otherwise would put an untestable release in the
		// table beside one CI runs. See D54.
		if m.Tier != container.Verified {
			t.Errorf("%s is %s, and a machine can only be %s",
				m.Name(), m.Tier, container.Verified)
		}
		for _, p := range []int{m.Port, m.Viewer} {
			if p == 0 {
				t.Errorf("%s has no port for the database or the viewer", m.Name())
				continue
			}
			if published(p) {
				t.Errorf("%s wants port %d, which dbrun publishes a container on", m.Name(), p)
			}
			if other, ok := ports[p]; ok {
				t.Errorf("%s wants port %d, which %s already uses", m.Name(), p, other)
			}
			ports[p] = m.Name()
		}
		if m.Provision <= 0 || m.Startup <= 0 {
			t.Errorf("%s has no time budget for provisioning or starting", m.Name())
		}
		if !strings.Contains(m.DSN(), ":"+strconv.Itoa(m.Port)) {
			t.Errorf("%s: the port is missing from %s", m.Name(), m.DSN())
		}
		// Exactly one kind. Both set would leave provision guessing, and
		// neither would leave it nothing to do.
		if (m.Windows == nil) == (m.Appliance == nil) {
			t.Errorf("%s must set exactly one of Windows and Appliance", m.Name())
		}
		if m.Appliance != nil {
			checkAppliance(t, m)
		}
	}
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// checkAppliance checks the parts an import needs.
func checkAppliance(t *testing.T, m container.Machine) {
	t.Helper()
	a := m.Appliance
	if !strings.HasSuffix(a.File, ".ova") || strings.Contains(a.File, "/") {
		t.Errorf("%s: %q is not the file name of an OVA", m.Name(), a.File)
	}
	// The digest is the only check there is, because the file cannot be
	// fetched again to compare. An empty one would import anything.
	if !sha256Hex.MatchString(a.SHA256) {
		t.Errorf("%s: %q is not a lower case SHA256", m.Name(), a.SHA256)
	}
	if u, err := url.Parse(a.Page); err != nil || u.Scheme != "https" {
		t.Errorf("%s: the download page is not an https URL: %q", m.Name(), a.Page)
	}
	// qemux/qemu attaches six disks at most.
	if len(a.Disks) == 0 || len(a.Disks) > 6 {
		t.Errorf("%s has %d disks, and a machine takes one to six", m.Name(), len(a.Disks))
	}
	size := regexp.MustCompile(`^[1-9][0-9]*[GT]$`)
	for _, d := range a.Disks {
		if !strings.HasSuffix(d.Member, ".vmdk") || !size.MatchString(d.Size) {
			t.Errorf("%s: a disk needs a .vmdk member and a size such as 100G: %+v", m.Name(), d)
		}
	}
	if !size.MatchString(a.Memory) || a.CPUs <= 0 || a.Port <= 0 {
		t.Errorf("%s needs memory such as 8G, a processor count and a port", m.Name())
	}
	if m.GuestPort() != a.Port {
		t.Errorf("%s: the guest port is %d and the appliance says %d",
			m.Name(), m.GuestPort(), a.Port)
	}
}
