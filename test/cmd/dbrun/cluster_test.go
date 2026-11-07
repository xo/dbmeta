package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOwnerID(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ owner, want string }{
		{"claude-code:05124815-a7b3-4c2d", "05124815"},
		{"user:ken", "ken"},
		{"user:Ken.Shaw-Jones", "kenshawj"},
		{"ci", "ci"},
		{"user:...", "x"},
		{"", "x"},
	} {
		if got := ownerID(tt.owner); got != tt.want {
			t.Errorf("ownerID(%q) = %q, want %q", tt.owner, got, tt.want)
		}
	}
}

func TestAClusterNameHoldsItsOwner(t *testing.T) {
	t.Setenv("DBMETA_CLUSTER_STATE", t.TempDir())
	got := withCluster(target{Name: "k8s-1.37"}, "claude-code:05124815-a7b3")
	if want := "k8s-1.37-05124815"; got.Cluster != want {
		t.Errorf("cluster = %q, want %q", got.Cluster, want)
	}
	if want := "kind-k8s-1.37-05124815"; got.Context != want {
		t.Errorf("context = %q, want %q", got.Context, want)
	}
	if !strings.HasSuffix(got.Kubeconfig, "/k8s-1.37-05124815/kubeconfig") {
		t.Errorf("kubeconfig = %q", got.Kubeconfig)
	}
	if other := withCluster(target{Name: "k8s-1.37"}, "claude-code:99999999"); other.Cluster == got.Cluster {
		t.Errorf("two owners share the cluster %q", got.Cluster)
	}
	if nodeOf(got.Cluster) != "k8s-1.37-05124815-control-plane" {
		t.Errorf("node = %q", nodeOf(got.Cluster))
	}
}

func TestClustersAreReadFromTheirState(t *testing.T) {
	t.Setenv("DBMETA_CLUSTER_STATE", t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	for i, name := range []string{"k8s-1.37-bbbbbbbb", "k8s-1.36-aaaaaaaa", "k8s-1.37-aaaaaaaa"} {
		c := cluster{name: name, dir: clusterRoot() + "/" + name, state: clusterState{
			Owner: "user:" + name[len(name)-8:], Release: name[:8], Created: now.Add(time.Duration(i) * time.Minute),
		}}
		if err := writeCluster(c); err != nil {
			t.Fatal(err)
		}
	}
	var names []string
	for _, c := range clustersOf("k8s-1.37") {
		names = append(names, c.name)
	}
	// The oldest is first, and k8s-1.36 is not a cluster of 1.37.
	if want := []string{"k8s-1.37-bbbbbbbb", "k8s-1.37-aaaaaaaa"}; !slices.Equal(names, want) {
		t.Errorf("clustersOf(k8s-1.37) = %v, want %v", names, want)
	}
	if got := len(clustersOf("")); got != 3 {
		t.Errorf("clustersOf(\"\") has %d clusters, want 3", got)
	}
	c, ok := readCluster("k8s-1.37-aaaaaaaa")
	if !ok || c.state.Owner != "user:aaaaaaaa" || !c.state.Created.Equal(now.Add(2*time.Minute)) {
		t.Errorf("readCluster = %+v, %v", c, ok)
	}
	if _, ok := readCluster("k8s-1.34-cccccccc"); ok {
		t.Error("readCluster found a cluster that was never written")
	}
}

func TestNodesReady(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		out  string
		want bool
	}{
		{"True\n", true},
		{strings.Repeat("True\n", 2), true},
		{"", false},
		{"False\n", false},
		{"True\nUnknown\n", false},
	} {
		if got := nodesReady(tt.out); got != tt.want {
			t.Errorf("nodesReady(%q) = %v, want %v", tt.out, got, tt.want)
		}
	}
}

func TestTheHintNamesWhatIsMissing(t *testing.T) {
	t.Parallel()
	none := func(string) bool { return false }
	all := func(string) bool { return true }
	if got := clusterHint("", all); got != "" {
		t.Errorf("a host with every module and no log fault got the hint %q", got)
	}
	got := clusterHint("ERROR: could not find a log line that matches", none)
	for _, want := range []string{"ip6_tables", "iptable_nat", "modules-load.d", "k8s-file"} {
		if !strings.Contains(got, want) {
			t.Errorf("the hint %q does not say %q", got, want)
		}
	}
	only := func(m string) bool { return m != "ip_tables" }
	if got := clusterHint("", only); !strings.Contains(got, "ip_tables,") && !strings.Contains(got, "ip_tables.") {
		t.Errorf("the hint %q does not name ip_tables as the missing module", got)
	}
}

func TestNoSelectorReachesAClusterButItsOwn(t *testing.T) {
	t.Setenv("DBMETA_CLUSTER_STATE", t.TempDir())
	all := targets()
	for _, sel := range []string{"all", "tested", "nightly", "verified", "staged"} {
		picked, _, err := resolve(all, []string{sel}, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range picked {
			if p.Kind == kindCluster {
				t.Errorf("%s reached the cluster %s", sel, p.Name)
			}
		}
	}
	picked, _, err := resolve(all, []string{"clusters"}, false)
	if err != nil || len(picked) < 4 {
		t.Fatalf("clusters = %d targets, %v", len(picked), err)
	}
	newest, notes, err := resolve(all, []string{"k8s"}, false)
	if err != nil || len(newest) != 1 || len(notes) != 1 {
		t.Fatalf("k8s = %v %v %v", newest, notes, err)
	}
	if want := picked[len(picked)-1].Name; newest[0].Name != want {
		t.Errorf("k8s is %s, and the newest cluster is %s", newest[0].Name, want)
	}
	if newest[0].Image == "" || newest[0].Kubeconfig == "" {
		t.Errorf("the cluster target has no image or kubeconfig: %+v", newest[0])
	}
}

func TestACommandThatConnectsIsRefusedForACluster(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"usql", "cli", "test", "fixture"} {
		err := oneCluster(t.Context(), runner{}, command, target{Kind: kindCluster}, options{})
		if err == nil || !strings.Contains(err.Error(), "holds no database") {
			t.Errorf("%s: err = %v, want the refusal", command, err)
		}
	}
}

// TestPodmanRaisesThePidLimitOfTheNode checks the command that raisePids runs,
// with a runner that records its arguments, and that docker is left alone.
func TestPodmanRaisesThePidLimitOfTheNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	script := filepath.Join(dir, "podman")
	body := "#!/bin/sh\necho \"$@\" >> " + log + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	c := cluster{name: "k8s-1.37-abcd1234"}
	if err := raisePids(t.Context(), runner{name: script}, c); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "update --pids-limit " + strconv.Itoa(clusterPids) + " " + c.node() + "\n"
	if string(got) != want {
		t.Errorf("podman was run as %q, and the expected command is %q", got, want)
	}
	if err := raisePids(t.Context(), runner{name: "docker-not-installed"}, c); err != nil {
		t.Errorf("raising the limit for docker returned %v, and it must do nothing", err)
	}
}
