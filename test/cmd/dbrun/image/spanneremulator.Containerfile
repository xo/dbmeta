# The Cloud Spanner emulator with a shell and wget beside it.
#
# Google's image is distroless. It holds the two emulator programs and no
# shell, so dbrun cannot run a check or a setup inside it. This adds a static
# busybox, which brings sh and wget and needs no library. The programs are
# not moved onto another base, because the emulator needs the C++ library
# that its own image ships, which is newer than Debian 12's.
ARG RELEASE
FROM docker.io/library/busybox:stable-musl AS busybox

FROM gcr.io/cloud-spanner-emulator/emulator:${RELEASE}
COPY --from=busybox /bin/busybox /bin/busybox
RUN ["/bin/busybox", "--install", "-s", "/bin"]
