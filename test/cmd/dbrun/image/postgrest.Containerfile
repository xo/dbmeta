# PostgREST with the PostgreSQL it serves, in one container.
#
# The PostgREST image holds one static program and no shell, so dbrun cannot
# run a check or a setup inside it. A PostgREST in one container and a
# PostgreSQL in another need a start order that dbrun does not have.
# This copies the program onto the PostgreSQL 18 image. The entry in
# container/postgrest.go starts both and sets them up.
ARG RELEASE
FROM docker.io/postgrest/postgrest:v${RELEASE} AS postgrest

FROM docker.io/library/postgres:18
COPY --from=postgrest /bin/postgrest /usr/local/bin/postgrest
EXPOSE 3000
