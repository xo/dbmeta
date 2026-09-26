package container_test

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// The Windows machine list and the scripts that provision from it. Two copies
// drift, the same way the Linux list and the CI workflow do, so the same kind
// of test holds them together.

// TestEveryWindowsMachineIsUsable checks the parts a Windows install needs.
// TestEveryMachineIsUsable checks what every machine needs, whichever kind.
func TestEveryWindowsMachineIsUsable(t *testing.T) {
	t.Parallel()
	for _, m := range windowsMachines() {
		w := m.Windows
		if w.Windows == "" || w.Image == "" || w.Installer == "" || w.RegistryKey == "" {
			t.Errorf("%s is missing something it needs: %+v", m.Name(), *w)
		}
		// A URL, and one a downloader will accept. One installer path has a
		// space in it, and curl refuses the raw character rather than
		// encoding it, so the escape belongs in the list.
		u, err := url.Parse(w.Installer)
		switch {
		case err != nil:
			t.Errorf("%s: the installer is not a URL: %v", m.Name(), err)
		case u.Scheme != "https":
			t.Errorf("%s: the installer is not https: %q", m.Name(), w.Installer)
		case strings.ContainsAny(w.Installer, " \t"):
			t.Errorf("%s: the installer URL has a raw space, which curl refuses. "+
				"Write it %%20: %q", m.Name(), w.Installer)
		}
		if f := w.InstallerFile(); !strings.HasSuffix(f, ".exe") || strings.Contains(f, "/") {
			t.Errorf("%s: %q is not a file name", m.Name(), f)
		}
	}
}

// windowsMachines returns the machines that install into Windows.
func windowsMachines() []container.Machine {
	var out []container.Machine
	for _, m := range container.Machines() {
		if m.Windows != nil {
			out = append(out, m)
		}
	}
	return out
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
	for _, v := range windowsMachines() {
		prefix, ok := want[v.Release]
		if !ok {
			t.Errorf("%s has no expected registry key. Add it here when you add the release.",
				v.Release)
			continue
		}
		if !strings.HasPrefix(v.Windows.RegistryKey, prefix+".") {
			t.Errorf("%s: expected the key to start %s., got %s",
				v.Release, prefix, v.Windows.RegistryKey)
		}
	}
}

// TestEveryReleaseTakesTheLicenseFlag pins a fact that was recorded wrong.
//
// The list said 2008 R2 refused /IACCEPTSQLSERVERLICENSETERMS, on a review
// that said the flag arrived in 2012. Its setup then refused to install
// without it. Unattended setup requires it on every release here, and a future
// release that does not can flip its own field and this test with it.
func TestEveryReleaseTakesTheLicenseFlag(t *testing.T) {
	t.Parallel()
	for _, v := range windowsMachines() {
		if !v.Windows.LicenseFlag {
			t.Errorf("%s: setup will refuse to run without the license flag. "+
				"If a release really does reject it, say so here and why.", v.Release)
		}
	}
}

// TestProvisionFillsEveryPlaceholder checks that dbrun fills in every
// placeholder the template has, and no more.
//
// dbrun refuses at runtime when one is left behind, which is 40 minutes into
// an install. This is the same question asked at build time.
func TestProvisionFillsEveryPlaceholder(t *testing.T) {
	t.Parallel()
	read := func(path ...string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{"..", "test", "cmd", "dbrun"}, path...)...))
		if err != nil {
			t.Fatalf("reading %s: %v", filepath.Join(path...), err)
		}
		return string(body)
	}
	tmpl, source := read("oem", "install.bat"), read("provision.go")

	placeholder := regexp.MustCompile(`@@[A-Z_]+@@`)
	found := map[string]bool{}
	for _, m := range placeholder.FindAllString(tmpl, -1) {
		found[m] = true
		if !strings.Contains(source, m) {
			t.Errorf("oem/install.bat has %s and provision.go never fills it in", m)
		}
	}
	if len(found) == 0 {
		t.Error("oem/install.bat has no placeholders, so this test guards nothing")
	}
	for _, m := range placeholder.FindAllString(source, -1) {
		if !found[m] {
			t.Errorf("provision.go fills in %s and oem/install.bat does not have it", m)
		}
	}
}
