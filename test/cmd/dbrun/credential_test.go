package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	"github.com/xo/dbmeta/hosted"
)

// The resolution of a hosted service's connection string is tested with no
// real credential. Each test names a service that does not exist, and points
// $XDG_CONFIG_HOME and $PATH at a temporary directory. See D117.

// credentialHome points the credential directory and the path at a new
// temporary directory, and returns the directory of credential files and the
// directory on the path.
func credentialHome(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	files := filepath.Join(root, "dbmeta", "credentials")
	if err := os.MkdirAll(files, 0o700); err != nil {
		t.Fatal(err)
	}
	return files, bin
}

func TestACredentialIsAbsentWhenNothingHasOne(t *testing.T) {
	credentialHome(t)
	t.Setenv("DBMETA_NOTHING_DSN", "")
	if _, ok, err := resolveCredential(t.Context(), "nothing"); ok || err != nil {
		t.Errorf("got ok %v and %v, and expected no credential and no error", ok, err)
	}
}

func TestTheEnvironmentComesFirst(t *testing.T) {
	files, _ := credentialHome(t)
	if err := os.WriteFile(filepath.Join(files, "svc"), []byte("svc://file"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBMETA_SVC_DSN", "svc://env")
	c, ok, err := resolveCredential(t.Context(), "svc")
	if err != nil || !ok || c.dsn != "svc://env" || c.source != "env DBMETA_SVC_DSN" {
		t.Errorf("got %+v, %v, %v", c, ok, err)
	}
}

func TestAFileIsReadOnlyWhenOnlyItsOwnerCanRead(t *testing.T) {
	files, _ := credentialHome(t)
	t.Setenv("DBMETA_SVC_DSN", "")
	file := filepath.Join(files, "svc")
	if err := os.WriteFile(file, []byte("svc://user:secret@host\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, ok, err := resolveCredential(t.Context(), "svc")
	if err != nil || !ok || c.dsn != "svc://user:secret@host" || c.source != "file "+file {
		t.Errorf("got %+v, %v, %v", c, ok, err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := resolveCredential(t.Context(), "svc"); ok || err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("a file others can read was used: ok %v, %v", ok, err)
	}
}

func TestAHelperComesLast(t *testing.T) {
	_, bin := credentialHome(t)
	t.Setenv("DBMETA_SVC_DSN", "")
	helper := filepath.Join(bin, "dbmeta-credential-svc")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\necho svc://from-helper\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	c, ok, err := resolveCredential(t.Context(), "svc")
	if err != nil || !ok || c.dsn != "svc://from-helper" || c.source != "helper "+helper {
		t.Errorf("got %+v, %v, %v", c, ok, err)
	}
}

func TestAHelperThatFailsIsAProblem(t *testing.T) {
	_, bin := credentialHome(t)
	t.Setenv("DBMETA_SVC_DSN", "")
	helper := filepath.Join(bin, "dbmeta-credential-svc")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\necho the secret >&2\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, ok, err := resolveCredential(t.Context(), "svc")
	if ok || err == nil {
		t.Fatalf("got ok %v and %v, and expected a problem", ok, err)
	}
	if strings.Contains(err.Error(), "the secret") {
		t.Errorf("the problem carries what the helper wrote to its error stream: %v", err)
	}
}

// TestAServiceAppearsOnlyWithACredential holds what Ken asked for: a hosted
// service is a target only while its connection string resolves, and what
// dbrun prints is masked.
func TestAServiceAppearsOnlyWithACredential(t *testing.T) {
	credentialHome(t)
	services := []hosted.Service{
		{Name: "have", Dialect: dbmeta.PostgreSQL, Tier: container.Verified},
		{Name: "lack", Dialect: dbmeta.MySQL, Tier: container.Verified},
	}
	t.Setenv("DBMETA_HAVE_DSN", "postgres://user:s3cret@db.example.com/app")
	t.Setenv("DBMETA_LACK_DSN", "")
	got, problems := resolveHosted(t.Context(), services)
	if len(problems) != 0 || len(got) != 1 {
		t.Fatalf("got %d targets and %v, and expected one target", len(got), problems)
	}
	h := got[0]
	switch {
	case h.Name != "have" || h.Kind != kindHosted || h.Env != "DBMETA_HAVE":
		t.Errorf("got %+v", h)
	case strings.Contains(h.DSN+h.URL+h.Credential, "s3cret"):
		t.Errorf("the secret is in what dbrun prints: %q %q %q", h.DSN, h.URL, h.Credential)
	case h.connectDSN() != "postgres://user:s3cret@db.example.com/app":
		t.Errorf("connects with %q", h.connectDSN())
	case h.env()[0] != "DBMETA_HAVE=postgres://user:s3cret@db.example.com/app":
		t.Errorf("the test variable is %q", h.env()[0])
	}
}

func TestMaskDSN(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ in, want string }{
		{"postgres://user:pw@host/db", "postgres://user:xxxxx@host/db"},
		{"cosmos://c2VjcmV0@host:443/db", "cosmos://xxxxx@host:443/db"},
		{"snowflake://me@acct/db?role=r&privateKey=abc", "snowflake://me@acct/db?privateKey=xxxxx&role=r"},
		{"awsathena://bucket/p?region=r&secretAccessKey=s&accessID=a", "awsathena://bucket/p?accessID=a&region=r&secretAccessKey=xxxxx"},
		{"bigquery://project/us/data", "bigquery://project/us/data"},
		{"Server=host;Password=pw", "xxxxx"},
	} {
		if got := maskDSN(test.in); got != test.want {
			t.Errorf("maskDSN(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// TestNoServiceRunsInCI holds that CI never runs a hosted service, because it
// costs money and needs a secret. hosted_test.go holds which of the two tiers
// outside CI each one is in.
func TestNoServiceRunsInCI(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, s := range hosted.All() {
		if s.Tier == container.Tested || s.Tier == container.Nightly {
			t.Errorf("%s is %s, and CI never runs a hosted service", s.Name, s.Tier)
		}
		if seen[s.Name] || s.Name == "" || s.Product == "" || s.Form == "" || s.Dialect == "" {
			t.Errorf("%+v is missing a field or repeats a name", s)
		}
		seen[s.Name] = true
	}
}

func TestKeyFileEnvNamesTheFileOfTheService(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	dir := filepath.Join(cfg, "dbmeta", "gcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := keyFileEnv("spanner"); len(got) != 0 {
		t.Errorf("a service with no key file: got %v, expected nothing", got)
	}
	file := filepath.Join(dir, "spanner.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := keyFileEnv("spanner")
	if len(got) != 1 || got[0] != "GOOGLE_APPLICATION_CREDENTIALS="+file {
		t.Errorf("got %v, expected the key file %s", got, file)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/elsewhere.json")
	if got := keyFileEnv("spanner"); len(got) != 0 {
		t.Errorf("a variable that is set already: got %v, expected nothing", got)
	}
}
