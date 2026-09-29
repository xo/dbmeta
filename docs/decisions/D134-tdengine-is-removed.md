# D134. TDengine is removed

Status: Amends D112.

Ken decided on 2026-09-30 that dbrun's TDengine entry is removed.

D112 added TDengine to dbrun for a TDengine driver that dbimp planned. dbimp's
D127 decided that no such driver is coming, and dburl's D41 removed the
tdengine scheme in v0.38.0, so tdengine:// no longer parses. dbmeta has no
TDengine model. Nothing reads the entry, and a product nothing reads is
removed (D118).

Removing it moves the host port of every server after it in container.All,
which D68 allows: a container whose port no longer matches is created again
when it next starts.
