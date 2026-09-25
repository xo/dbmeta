package container

import (
	"fmt"
	"net/url"
	"strings"
)

// The SQL Server releases that need a Windows virtual machine, and the Windows
// that hosts each one.
//
// SQL Server on Linux begins at 2017, so there is no container for 2016 or
// earlier and nothing older can run in CI. D54 says those releases are
// Archived, which means nothing is claimed for them. A virtual machine is how
// that changes, and D57 says what may be claimed once one has run.
//
// This holds data and nothing else, the same way the rest of this package
// does. It starts no virtual machine and knows nothing about podman. The
// provisioning lives in test/vm, which reads this list through
// test/tool/vms so that the list has one copy.
//
// # Why one machine per release
//
// SQL Server installs side by side, so four releases could share fewer
// machines. Both reviews said not to, for the same two reasons. A second
// release on a host has to be a named instance, which means a dynamic port and
// the SQL Server Browser rather than a fixed 1433. And the releases disagree
// about prerequisites, because 2008 R2 and 2012 want .NET Framework 3.5 where
// 2016 wants 4.6, so a shared host is a host where one of them is unusual.
//
// A machine also runs one at a time rather than all four at once. Each is
// started, tested and stopped.

// WindowsVM is one SQL Server release, the Windows it runs on, and what is
// needed to install it without a person watching.
type WindowsVM struct {
	// Release is the SQL Server release, written the way Microsoft names it.
	Release string
	// Tier is how thoroughly this release is tested. It is always Verified:
	// a virtual machine needs KVM and twenty minutes, so CI cannot run one.
	Tier Tier

	// Windows is the Windows Server release that hosts it, chosen to match the
	// era rather than to be current. Every one is a Microsoft evaluation
	// edition, which is free for 180 days of testing and needs no activation.
	Windows string
	// Image is the value dockurr/windows takes as VERSION to select it.
	Image string

	// Installer is the SQL Server Express package, downloaded on the Linux
	// host rather than inside the machine. Windows Server 2008 R2 has no TLS
	// 1.2 and cannot reach Microsoft's download servers at all.
	Installer string
	// Bootstrapper says the installer downloads the media when it runs rather
	// than carrying it. Only 2016 does, and it needs the machine to have a
	// working connection at install time.
	Bootstrapper bool

	// RegistryKey is the instance key under
	// HKLM\SOFTWARE\Microsoft\Microsoft SQL Server. It names the release and
	// it is where the listening port is set, because Express installs with a
	// dynamic port whatever the configuration file says.
	RegistryKey string
	// LicenseFlag says setup takes /IACCEPTSQLSERVERLICENSETERMS. Every
	// release here requires it, and the field stays because a release that
	// does not is exactly the kind of thing this list should be able to say.
	LicenseFlag bool

	// Port is the host port this machine publishes SQL Server on. One per
	// machine, so two can run at once when somebody wants to compare them.
	Port int
	// Viewer is the host port for the dockur web console, which is how a
	// person watches an install that has gone wrong.
	Viewer int
}

// Name returns a short name, such as "sqlserver-2012". It is safe as a
// container name and as a directory name.
func (v WindowsVM) Name() string { return "sqlserver-" + v.Release }

// DSN returns a connection string for this machine on the host.
func (v WindowsVM) DSN() string {
	return fmt.Sprintf("sqlserver://sa:%s@127.0.0.1:%d?database=master&encrypt=disable",
		url.QueryEscape(SQLServerPassword), v.Port)
}

// InstallerFile returns the file name the installer is saved as.
func (v WindowsVM) InstallerFile() string {
	if i := strings.LastIndex(v.Installer, "/"); i >= 0 {
		return v.Installer[i+1:]
	}
	return v.Installer
}

// WindowsVMs is every SQL Server release that needs a virtual machine.
//
// 2008 R2 is the floor because it is the oldest release whose Express
// installer Microsoft still publishes. The plain 2008 page is gone, and every
// 2012 service pack page is still there although the release page is not.
var WindowsVMs = []WindowsVM{
	{
		Release: "2008R2", Tier: Verified,
		Windows: "Windows Server 2008 R2", Image: "2008r2",
		// SQL Server 2008 R2 SP2 Express.
		Installer:   "https://download.microsoft.com/download/0/4/b/04be03cd-eaf3-4797-9d8d-2e08e316c998/SQLEXPR_x64_ENU.exe",
		RegistryKey: "MSSQL10_50.MSSQLSERVER",
		// This was written false, on a review that said the flag arrived in
		// 2012 and 2008 R2 refuses it. The opposite is true for SP2 Express,
		// and the machine said so:
		//
		//	The /IAcceptSQLServerLicenseTerms command line parameter is
		//	missing or has not been set to true. It is a required parameter
		//	for the setup action you are running.
		LicenseFlag: true,
		Port:        51433, Viewer: 8106,
	},
	{
		Release: "2012", Tier: Verified,
		Windows: "Windows Server 2012 R2", Image: "2012r2",
		// SQL Server 2012 SP4 Express. The service pack matters: 2012 RTM
		// does not install on Server 2012 R2 at all.
		Installer:   "https://download.microsoft.com/download/b/d/e/bde8fad6-33e5-44f6-b714-348f73e602b6/SQLEXPR_x64_ENU.exe",
		RegistryKey: "MSSQL11.MSSQLSERVER",
		LicenseFlag: true,
		Port:        51434, Viewer: 8107,
	},
	{
		Release: "2014", Tier: Verified,
		Windows: "Windows Server 2012 R2", Image: "2012r2",
		// SQL Server 2014 Express. The path has a space in it, and it is
		// written %20 here because it has to be: curl rejects the raw
		// character outright with "malformed input to a URL function",
		// whatever the shell does with the quoting.
		Installer:   "https://download.microsoft.com/download/e/a/e/eae6f7fc-767a-4038-a954-49b8b05d04eb/Express%2064BIT/SQLEXPR_x64_ENU.exe",
		RegistryKey: "MSSQL12.MSSQLSERVER",
		LicenseFlag: true,
		Port:        51435, Viewer: 8108,
	},
	{
		Release: "2016", Tier: Verified,
		Windows: "Windows Server 2016", Image: "2016",
		// SQL Server 2016 SP2 Express, and the only one of the four that is a
		// bootstrapper rather than the package. It downloads the media when it
		// runs, which Server 2016 can do and Server 2008 R2 could not.
		Installer:    "https://download.microsoft.com/download/3/7/6/3767d272-76a1-4f31-8849-260bd37924e4/SQLServer2016-SSEI-Expr.exe",
		Bootstrapper: true,
		RegistryKey:  "MSSQL13.MSSQLSERVER",
		LicenseFlag:  true,
		Port:         51436, Viewer: 8109,
	},
}

// WindowsVMByRelease returns the machine for a SQL Server release.
func WindowsVMByRelease(release string) (WindowsVM, bool) {
	for _, v := range WindowsVMs {
		if strings.EqualFold(v.Release, release) {
			return v, true
		}
	}
	return WindowsVM{}, false
}
