package container

import (
	"slices"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// The database releases that run on a virtual machine rather than in a
// container.
//
// This holds data and nothing else, the same way the rest of this package
// does. It starts no virtual machine and knows nothing about podman. The
// provisioning lives in test/cmd/dbrun, which reads this list from here so
// that the list has one copy.
//
// # Two kinds, one list
//
// A machine is built one of two ways, and each entry says which by setting
// exactly one of [Machine.Windows] and [Machine.Appliance].
//
// A Windows machine installs a database into an evaluation Windows, without a
// person watching, over about an hour. The old SQL Servers are these, because
// SQL Server on Linux begins at 2017. See D57 and windows.go.
//
// An appliance is a disk image a vendor ships with the database already in
// it. It is imported once from a file a person downloaded, and nothing is
// installed. The Exasol Community Edition is this. See D85.
//
// Everything after that is the same for both. dbrun starts, stops, probes and
// keeps a machine the same way whichever kind it is, and only provisioning
// differs. So they are one list, with the part that differs held beside the
// part that does not, and there is no interface: nothing dispatches on the
// kind, it reads fields. See D86.

// Machine is one database release on a virtual machine.
type Machine struct {
	// Dialect selects the model that reads this machine.
	Dialect dbmeta.Dialect
	// Product is the name the server reports for itself, as it is for a
	// [Server].
	Product string
	// Release is the database release, written the way the vendor names it.
	Release string
	// Tier is how thoroughly this release is tested. It is always Verified:
	// a virtual machine needs KVM, so CI cannot run one.
	Tier Tier

	// Port is the host port this machine publishes the database on. It is
	// fixed per machine rather than derived from a position in a list,
	// because a machine is kept, and a port that moves when a release is
	// added strands one that took an hour to build.
	Port int
	// Viewer is the host port for the web console, which is how a person
	// watches a boot or an install that has gone wrong.
	Viewer int

	// Provision is how long building this machine is allowed, from nothing
	// to a database that answers.
	Provision time.Duration
	// Startup is how long starting a machine that is already built is
	// allowed, before its database answers.
	Startup time.Duration

	// Windows describes a machine that installs its database into Windows.
	Windows *WindowsSpec
	// Appliance describes a machine imported from a vendor's disk image.
	Appliance *ApplianceSpec

	// dsn builds the connection string for this machine on the host, at its
	// own port. It is a function for the same reason [Server]'s is: the form
	// is the product's, and an appliance's credentials are the vendor's.
	dsn func(port int) string
	// url is the dburl style URL a person types, where that differs from the
	// DSN the driver takes. Nil means they are the same string.
	url func(port int) string
}

// Name returns a short name, such as "sqlserver-2012". It is safe as a
// container name and as a directory name.
func (m Machine) Name() string {
	return strings.ToLower(m.Product) + "-" + m.Release
}

// DSN returns a connection string for this machine on the host.
func (m Machine) DSN() string { return m.dsn(m.Port) }

// URL returns the dburl style URL for this machine, which is what usql takes.
// It is the DSN unless the driver takes a form that is not a URL.
func (m Machine) URL() string {
	if m.url != nil {
		return m.url(m.Port)
	}
	return m.dsn(m.Port)
}

// GuestPort returns the port the database listens on inside the machine.
func (m Machine) GuestPort() int {
	if m.Appliance != nil {
		return m.Appliance.Port
	}
	// Every Windows machine here is SQL Server, pinned to its usual port by
	// the registry key.
	return 1433
}

// ApplianceSpec is a machine a vendor ships as a virtual machine image with
// the database already installed in it.
type ApplianceSpec struct {
	// File is the image's file name as the vendor publishes it. dbrun looks
	// for it by this name.
	File string
	// SHA256 is the digest of the whole file as it was downloaded. The file
	// sits behind a signup form, so there is no URL to fetch it from and
	// nothing else to check it against.
	SHA256 string
	// Page is where a person downloads the file.
	Page string

	// Disks are the disk images inside the file, in the order the machine
	// attaches them. The first is the one it boots.
	Disks []Disk
	// Memory is the memory the machine is given, in the form qemux/qemu
	// takes, such as "8G". It is what the vendor's descriptor declares.
	Memory string
	// CPUs is the number of processors the machine is given, from the same
	// descriptor.
	CPUs int

	// Port is the port the database listens on inside the machine.
	Port int
}

// Disk is one disk image inside an appliance's file.
type Disk struct {
	// Member is the file's name inside the archive.
	Member string
	// Size is the disk's virtual size, in the form qemux/qemu takes, such as
	// "100G". It is given so that the machine does not try to resize it.
	Size string
}

// Machines returns every release that runs on a virtual machine.
func Machines() []Machine {
	return slices.Concat(SQLServerMachines, ExasolMachines)
}

// MachineByName returns the machine with this name, such as "sqlserver-2012".
func MachineByName(name string) (Machine, bool) {
	for _, m := range Machines() {
		if strings.EqualFold(m.Name(), name) {
			return m, true
		}
	}
	return Machine{}, false
}
