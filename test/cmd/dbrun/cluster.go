package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A cluster is a bare Kubernetes cluster that kind runs for dbfwd, whose tests
// need one. dbrun starts it and removes it, and it installs nothing inside.
// See D269.
//
// It differs from a server in three ways. Nobody shares one, because each test
// run installs operators and their CRDs across the whole cluster, so the name
// of a cluster holds its owner. kind cannot put a label on its node
// container, so the owner is in a file beside the kubeconfig. And a cluster
// counts against a limit of its own, because one is several processes and one
// to two GB of memory.

const (
	// clusterLimit is how many clusters can be running at once. It is not
	// part of the eight servers, because maxRunning counts only the
	// containers of the servers.
	clusterLimit = 2
	// clusterTimeout is how long a cluster gets to become ready.
	clusterTimeout = 5 * time.Minute
	// clusterWait is how long kind waits for the control plane. The cluster
	// check that follows it is the one that decides.
	clusterWait = "5m"
	// clusterPids is the process limit of the node under podman, whose own
	// default of 2048 is too small for a cluster that runs several operators.
	// See D271.
	clusterPids = 16384
)

// clusterState is what dbrun records about a cluster, in state.json beside its
// kubeconfig. The owner is here because kind cannot label its node container.
type clusterState struct {
	// Owner and OwnerName are what the owner label and the friendly name label
	// hold for a server. See D98 and D115.
	Owner     string `json:"owner"`
	OwnerName string `json:"ownerName,omitempty"`

	Release string    `json:"release"`
	Image   string    `json:"image"`
	Created time.Time `json:"created"`
}

// cluster is one cluster on this machine, whoever owns it.
type cluster struct {
	name  string
	dir   string
	state clusterState
}

// kubeconfig is the path of the file that holds the credentials.
func (c cluster) kubeconfig() string { return filepath.Join(c.dir, "kubeconfig") }

// node is the name of the container kind runs the single node in.
func (c cluster) node() string { return nodeOf(c.name) }

// nodeOf is the name kind gives the container of the control plane.
func nodeOf(name string) string { return name + "-control-plane" }

// clusterRoot is the directory that holds one directory for each cluster.
func clusterRoot() string { return stateDir("DBMETA_CLUSTER_STATE", "clusters") }

// ownerID shortens an owner into the part of a cluster name that tells two
// owners apart. It is the first eight lower case letters and digits after the
// kind of owner, so that a session is its first eight characters and a
// person is the login name. The result is safe in a container name.
func ownerID(owner string) string {
	if _, id, ok := strings.Cut(owner, ":"); ok {
		owner = id
	}
	var b strings.Builder
	for _, r := range strings.ToLower(owner) {
		if b.Len() == 8 {
			break
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

// clusterNameFor is the name of the cluster of one owner for one release, such
// as k8s-1.37-05124815.
func clusterNameFor(release, owner string) string {
	return release + "-" + ownerID(owner)
}

// withCluster fills the fields of a cluster target that depend on the caller:
// the name of its cluster, the kube context and the kubeconfig. Nothing needs
// to exist yet, as dsn says for a server that is not running.
func withCluster(t target, owner string) target {
	t.Cluster = clusterNameFor(t.Name, owner)
	t.Context = "kind-" + t.Cluster
	t.Kubeconfig = filepath.Join(clusterRoot(), t.Cluster, "kubeconfig")
	return t
}

// readCluster reads the directory of one cluster, and is false when there is
// no cluster of that name or its state cannot be read.
func readCluster(name string) (cluster, bool) {
	dir := filepath.Join(clusterRoot(), name)
	body, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return cluster{}, false
	}
	var s clusterState
	if err := json.Unmarshal(body, &s); err != nil {
		return cluster{}, false
	}
	return cluster{name: name, dir: dir, state: s}, true
}

// writeCluster records a cluster, with the owner and the image, and makes the
// directory that only the caller can read, because the kubeconfig in it holds
// the credentials of the cluster.
func writeCluster(c cluster) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("making %s: %w", c.dir, err)
	}
	body, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the state of %s: %w", c.name, err)
	}
	if err := os.WriteFile(filepath.Join(c.dir, "state.json"), body, 0o600); err != nil {
		return fmt.Errorf("writing the state of %s: %w", c.name, err)
	}
	return nil
}

// clustersOf lists every cluster of one release on this machine, whoever owns
// it, the oldest first. An empty release lists every cluster.
func clustersOf(release string) []cluster {
	entries, err := os.ReadDir(clusterRoot())
	if err != nil {
		return nil
	}
	var out []cluster
	for _, e := range entries {
		if !e.IsDir() || release != "" && !strings.HasPrefix(e.Name(), release+"-") {
			continue
		}
		if c, ok := readCluster(e.Name()); ok {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b cluster) int { return a.state.Created.Compare(b.state.Created) })
	return out
}

// who says who owns a cluster, for a person to read. It is runner.who for a
// state file.
func (c cluster) who() string {
	if c.state.OwnerName != "" && c.state.Owner != "" {
		return c.state.OwnerName + " (" + showOwner(c.state.Owner) + ")"
	}
	return showOwner(c.state.Owner)
}

// kind runs the kind command. It is told to use podman when the runner is
// podman, because kind picks docker unless it is told. The command is run
// with the kubeconfig named on its own command line and never with the
// default one, so that ~/.kube/config is not touched.
func (r runner) kind(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Env = os.Environ()
	if r.podman() {
		cmd.Env = append(cmd.Env, "KIND_EXPERIMENTAL_PROVIDER=podman")
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// changed drops what is remembered about a container, after kind changed it.
// A runner command does this itself, and kind is not one.
func (r runner) changed(name string) { r.forget([]string{"kind", name}) }

// clusterRunning reports whether the node of a cluster is running.
func (r runner) clusterRunning(ctx context.Context, c cluster) bool {
	return r.running(ctx, c.node())
}

// runningClusters lists the clusters whose node is running, whoever owns them.
func (r runner) runningClusters(ctx context.Context) []cluster {
	var out []cluster
	for _, c := range clustersOf("") {
		if r.clusterRunning(ctx, c) {
			out = append(out, c)
		}
	}
	return out
}

// makeRoomForCluster refuses to start a cluster when clusterLimit are already
// running. It never stops one, because a cluster holds the operators and the
// fixtures of a test run and takes minutes to build again. With --force it
// removes the oldest running cluster of any owner.
func makeRoomForCluster(ctx context.Context, r runner, keep string, o options) error {
	up := slices.DeleteFunc(r.runningClusters(ctx), func(c cluster) bool { return c.name == keep })
	for len(up) >= clusterLimit {
		if !o.force {
			held := make([]string, len(up))
			for i, c := range up {
				held[i] = c.name + " (" + c.who() + ")"
			}
			return fmt.Errorf("%d clusters are running and the limit is %d: %s."+
				" Remove one of yours, ask the owner of one, or pass --force to remove the oldest",
				len(up), clusterLimit, strings.Join(held, ", "))
		}
		if err := removeCluster(ctx, r, up[0]); err != nil {
			return err
		}
		fmt.Printf("  %-*s removed to stay within %d clusters\n", nameWidth(), up[0].name, clusterLimit)
		up = up[1:]
	}
	return nil
}

// removeCluster deletes a cluster and its directory.
func removeCluster(ctx context.Context, r runner, c cluster) error {
	out, err := r.kind(ctx, "delete", "cluster", "--name", c.name, "--kubeconfig", c.kubeconfig())
	r.changed(c.node())
	if err != nil {
		return fmt.Errorf("removing %s: %w\n%s", c.name, err, lastLines(out, 10))
	}
	if err := os.RemoveAll(c.dir); err != nil {
		return fmt.Errorf("removing %s: %w", c.dir, err)
	}
	return nil
}

// kubectlArgs is the command that runs kubectl inside the node, as the
// administrator of the cluster. The node image holds a kubectl, so the host
// needs none for this.
func kubectlArgs(node string, args ...string) []string {
	return append([]string{"exec", node, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf"}, args...)
}

// nodesReady reads what kubectl get nodes says, one condition status for each
// node, and reports whether there is a node and each one is Ready.
func nodesReady(out string) bool {
	fields := strings.Fields(out)
	return len(fields) > 0 && !slices.ContainsFunc(fields, func(s string) bool { return s != "True" })
}

// raisePids lifts the process limit of the node of a cluster to clusterPids.
// Podman gives a container 2048, and a cluster that runs several operators
// and their databases uses all of it: ClickHouse stops for want of threads
// and probes fail with "resource temporarily unavailable". Docker has no such
// default, so nothing is done for it. See D271.
func raisePids(ctx context.Context, r runner, c cluster) error {
	if !r.podman() {
		return nil
	}
	if out, err := r.output(ctx, "update", "--pids-limit", strconv.Itoa(clusterPids), c.node()); err != nil {
		return fmt.Errorf("raising the process limit of %s: %w\n%s", c.node(), err, lastLines(out, 10))
	}
	return nil
}

// waitClusterReady waits until the node is Ready and the API answers.
func waitClusterReady(ctx context.Context, r runner, c cluster, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var last string
	for {
		out, err := r.output(ctx, kubectlArgs(c.node(), "get", "--raw=/readyz")...)
		if err == nil {
			out, err = r.output(ctx, kubectlArgs(c.node(), "get", "nodes", "-o",
				`jsonpath={range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}`)...)
			if err == nil && nodesReady(out) {
				return nil
			}
		}
		last = out
		select {
		case <-ctx.Done():
			return fmt.Errorf("the cluster was not ready in %s: %s", timeout, lastLines(last, 3))
		case <-time.After(2 * time.Second):
		}
	}
}

// clusterHint explains the two faults of a rootless kind that a person meets
// first, from what kind said. The modules are the four that the rootless
// guide of kind asks for, and the log driver is the other fault it names.
func clusterHint(out string, modulesLoaded func(string) bool) string {
	var hints []string
	var missing []string
	for _, m := range []string{"ip6_tables", "ip6table_nat", "ip_tables", "iptable_nat"} {
		if !modulesLoaded(m) {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		hints = append(hints, "these kernel modules are not loaded: "+strings.Join(missing, ", ")+
			". Rootless kind needs them, and loading them needs root:"+
			" printf 'ip6_tables\\nip6table_nat\\nip_tables\\niptable_nat\\n' | sudo tee /etc/modules-load.d/iptables.conf"+
			" && sudo systemctl restart systemd-modules-load.service")
	}
	if strings.Contains(out, "could not find a log line that matches") {
		hints = append(hints, `podman needs log_driver = "k8s-file" under [containers] in ~/.config/containers/containers.conf`)
	}
	return strings.Join(hints, "\n  ")
}

// moduleLoaded reports whether a kernel module is loaded or built in. A
// built in module that has parameters is under /sys/module, and a loaded one
// is under it and in /proc/modules.
func moduleLoaded(name string) bool {
	if _, err := os.Stat(filepath.Join("/sys/module", name)); err == nil {
		return true
	}
	body, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	for line := range strings.Lines(string(body)) {
		if strings.HasPrefix(line, name+" ") {
			return true
		}
	}
	return false
}

// startLineCluster is what start says about a cluster: its state, the
// kubeconfig and the kube context, which is what kubectl takes.
func startLineCluster(t target, state string) {
	fmt.Printf("  %-*s %s: KUBECONFIG=%s  context: %s\n", nameWidth(), t.Name, state, t.Kubeconfig, t.Context)
}

// doStartCluster brings the cluster of the caller up for one release, and
// leaves it there. A cluster that is stopped is started again with its node
// kept, and a cluster that is not there is created.
func doStartCluster(ctx context.Context, r runner, t target, o options) error {
	if _, err := exec.LookPath("kind"); err != nil {
		return errors.New("kind is not on the path")
	}
	t = withCluster(t, currentOwner())
	c := cluster{name: t.Cluster, dir: filepath.Dir(t.Kubeconfig)}
	if got, ok := readCluster(c.name); ok {
		c = got
	}
	if r.clusterRunning(ctx, c) {
		if err := raisePids(ctx, r, c); err != nil {
			return err
		}
		if err := waitClusterReady(ctx, r, c, t.timeout(o)); err != nil {
			return fmt.Errorf("it is running and not ready: %w", err)
		}
		startLineCluster(t, "already up")
		return nil
	}
	if err := makeRoomForCluster(ctx, r, c.name, o); err != nil {
		return err
	}
	if r.exists(ctx, c.node()) {
		return restartCluster(ctx, r, t, c, o)
	}
	c.state = clusterState{
		Owner: currentOwner(), OwnerName: currentOwnerName(),
		Release: t.Release, Image: t.Image, Created: time.Now().UTC(),
	}
	if err := writeCluster(c); err != nil {
		return err
	}
	out, err := r.kind(ctx, "create", "cluster", "--name", c.name, "--image", t.Image,
		"--kubeconfig", c.kubeconfig(), "--wait", clusterWait)
	r.changed(c.node())
	if err != nil {
		// What kind made is removed, so that a failure leaves nothing to
		// count against the limit and nothing to explain.
		_, _ = r.kind(ctx, "delete", "cluster", "--name", c.name, "--kubeconfig", c.kubeconfig())
		_ = os.RemoveAll(c.dir)
		msg := fmt.Sprintf("creating the cluster: %v\n%s", err, lastLines(out, 10))
		if hint := clusterHint(out, moduleLoaded); hint != "" {
			msg += "\n  " + hint
		}
		return errors.New(msg)
	}
	if err := raisePids(ctx, r, c); err != nil {
		return err
	}
	if err := waitClusterReady(ctx, r, c, t.timeout(o)); err != nil {
		return err
	}
	startLineCluster(t, "up")
	return nil
}

// restartCluster starts the node of a stopped cluster, writes the kubeconfig
// again because the port of the API server can differ after a restart, and
// waits until the cluster is ready.
func restartCluster(ctx context.Context, r runner, t target, c cluster, o options) error {
	if out, err := r.output(ctx, "start", c.node()); err != nil {
		return fmt.Errorf("starting the node: %w\n%s", err, lastLines(out, 10))
	}
	if out, err := r.kind(ctx, "export", "kubeconfig", "--name", c.name, "--kubeconfig", c.kubeconfig()); err != nil {
		return fmt.Errorf("writing the kubeconfig: %w\n%s", err, lastLines(out, 10))
	}
	if err := raisePids(ctx, r, c); err != nil {
		return err
	}
	if err := waitClusterReady(ctx, r, c, t.timeout(o)); err != nil {
		return err
	}
	startLineCluster(t, "up again")
	return nil
}

// clustersFor lists the clusters that a command on one target acts on. They
// are the clusters of its release, and a command that changes one acts only
// on the caller's own unless --force says otherwise.
func clustersFor(t target) []cluster {
	return clustersOf(t.Name)
}

// doStopCluster stops the node of the cluster of the caller and keeps it, so
// that start brings the same cluster back.
func doStopCluster(ctx context.Context, r runner, t target, o options) error {
	for _, c := range clustersFor(t) {
		if !mayTouch(c.state.Owner, currentOwner(), o.force) {
			if c.name == clusterNameFor(t.Name, currentOwner()) {
				return notYoursCluster(c)
			}
			continue
		}
		if !r.clusterRunning(ctx, c) {
			continue
		}
		if out, err := r.output(ctx, "stop", c.node()); err != nil {
			return fmt.Errorf("stopping %s: %w\n%s", c.name, err, lastLines(out, 10))
		}
		fmt.Printf("  stopped %s\n", c.name)
	}
	return nil
}

// notYoursCluster is notYours for a cluster.
func notYoursCluster(c cluster) error { return notYours(c.who()) }

// doRemoveCluster removes the cluster of the caller. A stopped cluster of
// another owner is removed too, as a stopped container is, because its owner
// can be a session that ended. A running cluster of another owner is not,
// unless --force says so.
func doRemoveCluster(ctx context.Context, r runner, t target, o options) error {
	if _, err := exec.LookPath("kind"); err != nil {
		return errors.New("kind is not on the path")
	}
	for _, c := range clustersFor(t) {
		if running := r.clusterRunning(ctx, c); !mayTouch(c.state.Owner, currentOwner(), o.force) && running {
			if c.name == clusterNameFor(t.Name, currentOwner()) {
				return notYoursCluster(c)
			}
			continue
		}
		if err := removeCluster(ctx, r, c); err != nil {
			return err
		}
		fmt.Printf("  removed %s\n", c.name)
	}
	return nil
}

// doStatusCluster prints the clusters of one release, with their owners. A
// stopped cluster is shown only with --all, as a stopped server is.
func doStatusCluster(ctx context.Context, r runner, t target, o options) error {
	me := currentOwner()
	for _, c := range clustersFor(t) {
		whoseIs := "  (" + c.who() + ")"
		if c.state.Owner == me {
			whoseIs = "  (yours)"
		}
		switch {
		case r.clusterRunning(ctx, c):
			fmt.Printf("  %-*s KUBECONFIG=%s  context: kind-%s%s\n", nameWidth(), c.name, c.kubeconfig(), c.name, whoseIs)
		case o.all && r.exists(ctx, c.node()):
			fmt.Printf("  %-*s KUBECONFIG=%s  (stopped, %s)\n", nameWidth(), c.name, c.kubeconfig(), c.who())
		}
	}
	return nil
}

// clusterEntries is statusJSON for the clusters of one release: one entry for
// each cluster that runs, and for each that is stopped when all asks for them.
func clusterEntries(ctx context.Context, r runner, t target, all bool) []statusEntry {
	me := currentOwner()
	var out []statusEntry
	for _, c := range clustersFor(t) {
		state := "running"
		if !r.clusterRunning(ctx, c) {
			if !all || !r.exists(ctx, c.node()) {
				continue
			}
			state = "stopped"
		}
		x := t
		x.Cluster, x.Context, x.Kubeconfig = c.name, "kind-"+c.name, c.kubeconfig()
		out = append(out, statusEntry{
			target: x, State: state,
			Owner: c.state.Owner, OwnerName: c.state.OwnerName,
			Mine: c.state.Owner == me, Started: c.state.Created.Format(time.RFC3339),
		})
	}
	return out
}

// doLogsCluster shows what the node of the cluster of the caller said.
func doLogsCluster(ctx context.Context, r runner, t target, o options) error {
	t = withCluster(t, currentOwner())
	node := nodeOf(t.Cluster)
	if !r.exists(ctx, node) {
		return errors.New("there is no such cluster")
	}
	args := []string{"logs"}
	if o.follow {
		args = append(args, "--follow")
	}
	cmd := exec.CommandContext(ctx, r.name, append(args, node)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
