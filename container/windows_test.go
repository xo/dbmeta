package container_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// The Windows machine list and the scripts that provision from it. Two copies
// drift, the same way the Linux list and the CI workflow do, so the same kind
// of test holds them together.

// TestEveryWindowsVMIsUsable checks the parts a provisioner needs.
func TestEveryWindowsVMIsUsable(t *testing.T) {
	t.Parallel()
	ports := map[int]string{}
	for _, s := range container.All() {
		// A machine must not collide with a Linux container. Those publish
		// from 55000 up, and these are in the 51000s, and nothing enforced it
		// until now.
		ports[s.Port] = s.Name()
	}
	seen := map[string]bool{}
	for _, v := range container.WindowsVMs {
		if seen[v.Name()] {
			t.Errorf("%s is listed twice", v.Name())
		}
		seen[v.Name()] = true
		if v.Release == "" || v.Windows == "" || v.Image == "" ||
			v.Installer == "" || v.RegistryKey == "" {
			t.Errorf("%s is missing something it needs: %+v", v.Name(), v)
		}
		// A virtual machine cannot run in CI, so it is Verified and never
		// Tested. Saying otherwise would put an untestable release in the
		// table beside one CI runs. See D54.
		if v.Tier != container.Verified {
			t.Errorf("%s is %s, and a machine can only be %s",
				v.Name(), v.Tier, container.Verified)
		}
		for _, p := range []int{v.Port, v.Viewer} {
			if other, ok := ports[p]; ok {
				t.Errorf("%s wants port %d, which %s already uses", v.Name(), p, other)
			}
			ports[p] = v.Name()
		}
		if !strings.HasPrefix(v.Installer, "https://") {
			t.Errorf("%s: the installer is not an https URL: %q", v.Name(), v.Installer)
		}
		if f := v.InstallerFile(); !strings.HasSuffix(f, ".exe") || strings.Contains(f, "/") {
			t.Errorf("%s: %q is not a file name", v.Name(), f)
		}
		if !strings.Contains(v.DSN(), strconv.Itoa(v.Port)) {
			t.Errorf("%s: the port is missing from %s", v.Name(), v.DSN())
		}
	}
}

// TestTheRegistryKeyMatchesTheRelease checks the one value that is silently
// wrong when it is wrong.
//
// It names the instance key where the listening port is set. A key for another
// release writes the port into a key nothing reads, setup reports success, and
// the machine is simply unreachable with no error anywhere.
func TestTheRegistryKeyMatchesTheRelease(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"2008R2": "MSSQL10_50", // SQL Server 2008 R2 is 10.50
		"2012":   "MSSQL11",
		"2014":   "MSSQL12",
		"2016":   "MSSQL13",
	}
	for _, v := range container.WindowsVMs {
		prefix, ok := want[v.Release]
		if !ok {
			t.Errorf("%s has no expected registry key. Add it here when you add the release.",
				v.Release)
			continue
		}
		if !strings.HasPrefix(v.RegistryKey, prefix+".") {
			t.Errorf("%s: expected the key to start %s., got %s",
				v.Release, prefix, v.RegistryKey)
		}
	}
}

// TestOnly2008R2RefusesTheLicenseFlag pins the one per release difference in
// the setup command line. /IACCEPTSQLSERVERLICENSETERMS arrived in 2012 and
// 2008 R2 fails when it is given one.
func TestOnly2008R2RefusesTheLicenseFlag(t *testing.T) {
	t.Parallel()
	for _, v := range container.WindowsVMs {
		want := v.Release != "2008R2"
		if v.LicenseFlag != want {
			t.Errorf("%s: expected LicenseFlag %v, got %v", v.Release, want, v.LicenseFlag)
		}
	}
}

// TestTheProvisioningScriptsMatchTheList checks that the scripts fill in every
// placeholder the template has, and no more.
func TestTheProvisioningScriptsMatchTheList(t *testing.T) {
	t.Parallel()
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "test", "vm", path))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		return string(body)
	}
	tmpl, script := read(filepath.Join("oem", "install.bat")), read("provision.sh")

	placeholder := regexp.MustCompile(`@@[A-Z_]+@@`)
	found := map[string]bool{}
	for _, m := range placeholder.FindAllString(tmpl, -1) {
		found[m] = true
		if !strings.Contains(script, m) {
			t.Errorf("oem/install.bat has %s and provision.sh never fills it in", m)
		}
	}
	if len(found) == 0 {
		t.Error("oem/install.bat has no placeholders, so this test guards nothing")
	}
	for _, m := range placeholder.FindAllString(script, -1) {
		if !found[m] {
			t.Errorf("provision.sh fills in %s and oem/install.bat does not have it", m)
		}
	}

	// The reader and the writer have to agree on the separator, and a tab
	// here would silently shift every field after the empty one.
	if !strings.Contains(script, `IFS=$'\x1f'`) {
		t.Error("provision.sh must split on the unit separator, not on a tab. " +
			"2008R2 has an empty field and bash collapses a run of tabs.")
	}
}
