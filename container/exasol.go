package container

import (
	"fmt"
	"time"

	"github.com/xo/dbmeta"
)

// The Exasol releases dbmeta is tested against.
//
// Exasol is tested two ways, because it ships two ways, and D85 is the
// decision. The current release is the nano container here. An older one is
// the Community Edition appliance, which is a frozen virtual machine further
// down this file.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides the container and it gives a floor of one. exasol/nano
// carries one release line, 2026.2.0, published as nano.1 through nano.5.
// nano.5 was pushed on 2026-09-24, three days before this was written, for
// amd64 and arm64. Each nano build replaces the one before it rather than
// being a release of its own, so the line is one release and the tag pins the
// newest build.
//
// exasol/docker-db, the older image, is not used. D77 recorded that it does
// not initialize under rootless podman.
//
// A single release cannot exercise a version gate, which is why the
// Community Edition is here as well. It is Exasol 8, build 2025.2.1, a
// release line behind. It needs KVM, so it is Verified, the same as the SQL
// Server machines. See D85 and D86.
var exasol = product{
	dialect: dbmeta.Exasol,
	name:    "exasol",
	image:   "docker.io/exasol/nano",
	// The release is the line and the suffix is the build. There is no
	// bare 2026.2.0 tag.
	tagSuffix: "-nano.5",
	port:      8563,
	// The image has no shell and no client, so nothing can be run inside
	// it to ask whether it is up. An empty readiness command tells dbrun to
	// ask from the host with the driver instead.
	ready: nil,
	// Both are the vendor's recommendation for the image. The database
	// warns at start when shared memory is the default 64 MB, and it caps
	// its own connection count by the process limit.
	runFlags: []string{"--shm-size=512m", "--pids-limit=-1"},
	dsn:      exasolDSN("exasol"),
	url:      exasolURL("exasol"),
}

// Exasol is every Exasol container release dbmeta is tested against.
var Exasol = list{}.add(exasol, Tested, "2026.2.0")

// ExasolMachines is the Exasol release that runs on a virtual machine.
//
// The Community Edition is published only as a VirtualBox or VMware image
// behind a signup form, so there is no URL to fetch it from. dbrun imports
// the file a person downloaded. The VirtualBox one was measured and is the
// one pinned. D85 has the measurement.
var ExasolMachines = []Machine{
	{
		Dialect: dbmeta.Exasol, Product: "exasol", Release: "2025.2.1",
		Tier: Verified,
		// Ports in the range the SQL Server machines use, above them.
		Port: 51437, Viewer: 8110,
		// Converting the disks is most of it. Booting took about a minute.
		Provision: 30 * time.Minute,
		Startup:   10 * time.Minute,
		Appliance: &ApplianceSpec{
			File:   "Exasol_Community_Edition_v8_202521_virtualbox.ova",
			SHA256: "ce17dac5a3d36caa2bfbd422286022e7474275b786be8b205c3a33f371d214a5",
			Page:   "https://www.exasol.com/free-signup-community-edition/",
			Disks: []Disk{
				// The system disk, 10 GiB used of 100.
				{Member: "Exasol_Community_Edition_v8_202521-disk001.vmdk", Size: "100G"},
				// The data disk, 41 MiB used of 500.
				{Member: "Exasol_Community_Edition_v8_202521-disk002.vmdk", Size: "500G"},
			},
			// What the image's own descriptor declares.
			Memory: "8G", CPUs: 4,
			Port: 8563,
		},
		dsn: exasolDSN("exasol"),
		url: exasolURL("exasol"),
	},
}

// exasolDSN is the connection string the Exasol driver takes.
//
// The password is the vendor's default rather than [Password], on both. The
// appliance is frozen and cannot be handed one at start. The nano image reads
// its first password from a file mounted into it, and a [Server] carries
// arguments and environment rather than files, so it keeps its default too.
//
// Certificate validation is off. The appliance's certificate is issued for
// exacluster.local and nano's for localhost, and neither is signed by
// anything a host trusts, so validation fails whatever the host name.
func exasolDSN(password string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("exa:127.0.0.1:%d;user=sys;password=%s;validateservercertificate=0",
			port, password)
	}
}

// exasolURL is the dburl style URL a person types into usql.
func exasolURL(password string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("exasol://sys:%s@127.0.0.1:%d?validateservercertificate=0",
			password, port)
	}
}
