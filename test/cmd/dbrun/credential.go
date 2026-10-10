package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xo/dburl"

	"github.com/xo/dbmeta/container"
	"github.com/xo/dbmeta/hosted"
	"github.com/xo/dbmeta/test/internal/spannerdsn"
)

// A hosted service is reached with a connection string that holds a secret,
// and a person provisions it. dbrun reads it from the first of three places
// that has one, and a service appears in dbrun only while one of them does.
// See D117.
//
//  1. The variable DBMETA_<NAME>_DSN.
//  2. The file <name> in $XDG_CONFIG_HOME/dbmeta/credentials, which only its
//     owner can read.
//  3. The helper dbmeta-credential-<name> on the path, which prints the
//     connection string, the way a git or docker credential helper does.
//
// The connection string can hold no secret at all, when the driver reads one
// by itself, such as a key file named by GOOGLE_APPLICATION_CREDENTIALS.
// hosted.Service.Native says which.

// credential is a resolved connection string and where it came from. The
// source never holds the secret.
type credential struct {
	dsn    string
	source string
}

// helperTimeout is how long a credential helper can take.
const helperTimeout = 10 * time.Second

// credentialDir is the directory of credential files.
func credentialDir() string {
	return filepath.Join(configDir(), "dbmeta", "credentials")
}

// configDir is $XDG_CONFIG_HOME, or ~/.config when it is unset.
func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config"
	}
	return filepath.Join(home, ".config")
}

// credentialEnv names the variable a service's connection string is read
// from.
func credentialEnv(name string) string {
	return "DBMETA_" + strings.ToUpper(name) + "_DSN"
}

// resolveCredential finds the connection string of a service. It returns
// false when no place has one, which is not an error: the service is then
// absent. An error is a place that has one and cannot be used, such as a
// file that others can read.
func resolveCredential(ctx context.Context, name string) (credential, bool, error) {
	env := credentialEnv(name)
	if s := strings.TrimSpace(os.Getenv(env)); s != "" {
		return credential{dsn: s, source: "env " + env}, true, nil
	}
	file := filepath.Join(credentialDir(), name)
	switch info, err := os.Stat(file); {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return credential{}, false, fmt.Errorf("reading %s: %w", file, err)
	case !info.Mode().IsRegular():
		return credential{}, false, fmt.Errorf("%s is not a regular file", file)
	case info.Mode().Perm()&0o077 != 0:
		return credential{}, false, fmt.Errorf("%s can be read by others. Run: chmod 600 %s", file, file)
	default:
		b, err := os.ReadFile(file)
		if err != nil {
			return credential{}, false, fmt.Errorf("reading %s: %w", file, err)
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return credential{}, false, fmt.Errorf("%s is empty", file)
		}
		return credential{dsn: s, source: "file " + file}, true, nil
	}
	helper, err := exec.LookPath("dbmeta-credential-" + name)
	if err != nil {
		return credential{}, false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, helperTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper)
	// What the helper writes to its error stream is not shown, because it
	// can hold the secret.
	out, err := cmd.Output()
	if err != nil {
		return credential{}, false, fmt.Errorf("running %s: %w", helper, err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return credential{}, false, fmt.Errorf("%s printed nothing", helper)
	}
	return credential{dsn: s, source: "helper " + helper}, true, nil
}

// keyFileEnv names the key file of a service for the driver that reads one by
// itself. Each service and each principal has its own file, named for its
// credential file, in $XDG_CONFIG_HOME/dbmeta/gcp. If
// GOOGLE_APPLICATION_CREDENTIALS is set already, or the file is not there,
// the answer is empty, so a person's own setting wins. See D218.
func keyFileEnv(name string) []string {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		return nil
	}
	file := filepath.Join(configDir(), "dbmeta", "gcp", name+".json")
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []string{"GOOGLE_APPLICATION_CREDENTIALS=" + file}
}

// hostedOnce resolves every service once for the life of the command, so
// that a helper runs once however often the list of targets is read.
var hostedOnce = sync.OnceValue(func() []target {
	out, problems := resolveHosted(context.Background(), hosted.All())
	for _, err := range problems {
		fmt.Fprintln(os.Stderr, "dbrun:", err)
	}
	return out
})

// hostedTargets are the hosted services whose connection strings resolve.
func hostedTargets() []target {
	return hostedOnce()
}

// resolveHosted builds a target for each service that has a connection
// string, and returns a problem for each place that has one and cannot be
// used. A service with neither is left out and says nothing.
func resolveHosted(ctx context.Context, services []hosted.Service) ([]target, []error) {
	var (
		out      []target
		problems []error
	)
	for _, s := range services {
		c, ok, err := resolveCredential(ctx, s.Name)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s is not available: %w", s.Name, err))
			continue
		}
		if !ok {
			continue
		}
		masked := maskDSN(c.dsn)
		// CI never runs a hosted service, so a Staged one will be Verified
		// if a model reads it (D120).
		var cadence container.Tier
		if s.Tier == container.Staged {
			cadence = container.Verified
		}
		out = append(out, target{
			Name: s.Name, Product: s.Name, Kind: kindHosted, Tier: s.Tier, Cadence: cadence, Dialect: s.Dialect,
			Env:        "DBMETA_" + strings.ToUpper(s.Name),
			DSN:        masked,
			URL:        masked,
			Credential: c.source,
			secret:     c.dsn,
			driverDSN:  driverDSN(c.dsn),
		})
	}
	return out, problems
}

// driverDSN is what sql.Open takes for a secret URL. The scheme of the URL
// is dburl's, and a driver can read it as part of the user name, such as
// gosnowflake, or refuse it, such as pgx on redshift. If dburl cannot parse
// the URL, the URL itself is returned and the connection reports the fault.
func driverDSN(secret string) string {
	// The Spanner tests open go-sql-spanner, because the driver of dbimp and
	// go-sql-spanner register one name, and Spanner Omni needs the second. The
	// URL of the file becomes the DSN that go-sql-spanner reads. See D229.
	if strings.HasPrefix(secret, "spanner:") {
		if dsn, err := spannerdsn.FromURL(secret); err == nil {
			return dsn
		}
		return secret
	}
	u, err := dburl.Parse(secret)
	if err != nil || u.DSN == "" {
		return secret
	}
	return u.DSN
}

// secretKeys are the query parameters whose values are masked, matched
// without regard to case: a key holds one of these words.
var secretKeys = []string{"password", "secret", "token", "key", "credential"}

// userIsSecret are the schemes whose user name can be the secret. The credential
// of Cosmos DB holds the account key as the password and any word as the user
// (D228), and maskDSN hides the password. A file in the older form, with the key
// as the user and no password, is still hidden whole.
var userIsSecret = map[string]bool{"cosmos": true, "cm": true, "gocosmos": true}

// keyIDIsSecret are the schemes whose user name is the access key id, with the
// secret key as the password. The id is not the secret, and dbrun hides it too,
// because it names the account that owns the key. See D229.
var keyIDIsSecret = map[string]bool{"athena": true}

// masked replaces a secret in what dbrun prints.
const masked = "xxxxx"

// maskDSN hides the secret in a connection string: the password, a user name
// that is a key, and each query parameter that names a secret. A connection
// string that is not a URL is hidden whole, because nothing says which part
// of it is the secret.
func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return masked
	}
	if u.User != nil {
		name := u.User.Username()
		_, hasPassword := u.User.Password()
		switch {
		case hasPassword && keyIDIsSecret[u.Scheme]:
			u.User = url.UserPassword(masked, masked)
		case hasPassword:
			u.User = url.UserPassword(name, masked)
		case userIsSecret[u.Scheme]:
			u.User = url.User(masked)
		}
	}
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			lower := strings.ToLower(k)
			for _, w := range secretKeys {
				if strings.Contains(lower, w) {
					q.Set(k, masked)
					break
				}
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// usqlHosted opens usql on a hosted service without the secret on a command
// line, where a process list shows it. The connection string goes in a
// usql configuration file that only its owner can read, as a named
// connection, and usql is started on the name. The file is removed when usql
// ends.
func usqlHosted(ctx context.Context, t target) error {
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	dir, err := os.MkdirTemp("", "dbrun-usql-")
	if err != nil {
		return fmt.Errorf("making a directory for the usql configuration: %w", err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "config.yaml")
	// A YAML string in double quotes holds any connection string once its
	// backslashes and quotes are escaped.
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(t.secret)
	body := "connections:\n  " + t.Name + ": \"" + quoted + "\"\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		return fmt.Errorf("writing the usql configuration: %w", err)
	}
	cmd := exec.CommandContext(ctx, "usql", "--config", file, t.Name)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), keyFileEnv(t.Name)...)
	return cmd.Run()
}
