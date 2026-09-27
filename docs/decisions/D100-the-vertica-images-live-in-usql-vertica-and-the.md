# D100. The Vertica images live in usql/vertica, and the older ones wait for admintools

Status: Amends D88.

The nightly run on 2026-09-27 failed on vertica-7.2 and vertica-10.1, for two
different reasons, and both were only ever measured on this machine with
podman.

## 7.2 never ran in CI

`docker.io/colemantw/vertica`, the only 7.2 image, was pushed in 2016 with a
Docker image manifest of schema 1. The Docker on GitHub's runners refuses to
pull schema 1, and podman still pulls it, so `docker run` failed in the first
second and vertica-7.2 had never run in CI. The other three images are schema
2.

Ken chose to copy all four into one repository that the usql organization
owns, `docker.io/usql/vertica`, with proper tags, rather than to move 7.2 to
Verified or to special case podman in `dbrun`. Ken logged in and podman
pushed each image with `--format v2s2`. Every release has two tags, the
release as `dbrun` names it and the full release, such as `7.2` and `7.2.1`.
The registry reports every manifest as schema 2.

The layers are the originals, so each copy is the same image with a new
manifest, and D88's checks on whose build each is still hold. Each entry pins
the digest of the copy. `container/vertica.go` records each source image and
its digest. A tag that the usql organization owns can still be pushed again,
so the pin stays for D88's reason.

The rule for next time is in `CONTAINERS.md`: check the manifest format of an
image that somebody other than the vendor pushed.

## 10.1 answered before its functions existed

The entrypoint of the three older images runs `admintools -t create_db`,
which installs Vertica's function packages after the database already
answers. LISTAGG is in one of them. CI ran the tests the moment the server
answered, and 10.1 refused LISTAGG as a function that does not exist.
Measured here: LISTAGG is absent when `start` returns and present twenty
seconds later.

The `Init` of the three older images now waits until no `admintools` process
is left, and then creates the user. The pattern is written `[a]dmintools`,
because a plain `pgrep -f admintools` matched the shell that ran it, whose own
command line holds the word. LISTAGG answers the moment `start` returns now.

All four releases pass the whole test module through `dbrun test` on the
`usql/vertica` images, and 7.2 was pulled from the registry by digest.
