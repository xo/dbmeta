package container

import "strings"

// The Kubernetes releases that dbrun starts a cluster for.
//
// A cluster is not a database. It is a bare Kubernetes cluster that kind runs
// as one node container, so that dbfwd can test its port forward against the
// operators that run databases inside Kubernetes. Nothing here installs
// anything into the cluster, and dbmeta learns no operator. See D269.
//
// This holds data and nothing else, the same way the rest of this package
// does. It starts nothing and imports no kind client. A cluster is not in
// [All] and not in any tier, so a selector such as all or tested never
// reaches one. A caller reads [Clusters] to find the releases.
//
// # The range
//
// kind publishes a node image for each of the newest four Kubernetes lines
// with every release of kind, and says that only the digest guarantees the
// image built for that release. So each entry holds the tag and the digest,
// and [Cluster.Ref] returns both. The four below are from the notes of kind
// v0.33.0, which were read on 2026-11-07. The list changes when a new kind
// release names new images.

// Cluster is one Kubernetes release that kind runs.
type Cluster struct {
	// Release is the Kubernetes line, such as "1.37". It names the cluster.
	Release string
	// Version is the Kubernetes release the node image holds, such as
	// "1.37.0".
	Version string
	// Digest is the digest of the node image, which pins it.
	Digest string
}

// Name returns the name of the cluster as a selector, such as "k8s-1.37". The
// cluster that a caller owns adds the owner after it.
func (c Cluster) Name() string {
	return "k8s-" + c.Release
}

// Image returns the tag of the node image, without its digest.
func (c Cluster) Image() string {
	return "kindest/node:v" + c.Version
}

// Ref returns the node image as kind takes it, with the tag and the digest.
func (c Cluster) Ref() string {
	return c.Image() + "@" + c.Digest
}

// Line returns the Kubernetes line of a version, such as "1.37" for
// "1.37.0".
func Line(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

// Clusters returns every Kubernetes release dbrun starts a cluster for, the
// oldest first.
func Clusters() []Cluster {
	return []Cluster{
		{Release: "1.34", Version: "1.34.11", Digest: "sha256:44e222ee2132dab25ff87301682f89eb82c7880ea3a1bf543bfe9708fd08d67d"},
		{Release: "1.35", Version: "1.35.8", Digest: "sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0"},
		{Release: "1.36", Version: "1.36.4", Digest: "sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed"},
		{Release: "1.37", Version: "1.37.0", Digest: "sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5"},
	}
}
