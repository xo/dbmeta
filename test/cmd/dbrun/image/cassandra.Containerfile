# Cassandra with the settings the tests need turned on.
#
# The Apache image ships a cassandra.yaml that refuses three things dbmeta has
# queries for, and its entrypoint maps only eight yaml keys to environment
# variables, none of them these. So the settings are baked in.
#
# What is turned on and why:
#
#   user defined functions   Functions and Aggregates read
#                            system_schema.functions and
#                            system_schema.aggregates. Without this, CREATE
#                            FUNCTION is refused and the fixture cannot make
#                            one, which hard rule 9 does not allow.
#   materialized views       Views reads system_schema.views. 5.0 ships with
#                            them off, and 3.11 ships with them on, so a
#                            fixture that works on one silently builds nothing
#                            on the other.
#   PasswordAuthenticator    Roles, RoleGrants and Privileges read system_auth.
#                            With AllowAllAuthenticator there is one role and
#                            no grant, and D61 has no second principal to
#                            compare against.
#   CassandraAuthorizer      Without it a GRANT is refused, so
#                            system_auth.role_permissions stays empty.
#
# The key names changed. 3.11 and 4.0 write enable_user_defined_functions and
# enable_materialized_views. 4.1 and later write user_defined_functions_enabled
# and materialized_views_enabled. Both spellings are edited, and the build
# then checks the result rather than trusting sed, because a sed that matches
# nothing changes nothing and says so to nobody. That is how the Oracle 19c
# build once produced an image that looked finished and could not open a
# database.

ARG RELEASE=5.0
FROM docker.io/library/cassandra:${RELEASE}

RUN set -eu; \
    conf=/etc/cassandra/cassandra.yaml; \
    sed -i -E 's/^authenticator:.*/authenticator: PasswordAuthenticator/' "$conf"; \
    sed -i -E 's/^authorizer:.*/authorizer: CassandraAuthorizer/' "$conf"; \
    sed -i -E 's/^#? ?enable_user_defined_functions:.*/enable_user_defined_functions: true/' "$conf"; \
    sed -i -E 's/^#? ?user_defined_functions_enabled:.*/user_defined_functions_enabled: true/' "$conf"; \
    sed -i -E 's/^#? ?enable_scripted_user_defined_functions:.*/enable_scripted_user_defined_functions: true/' "$conf"; \
    sed -i -E 's/^#? ?enable_materialized_views:.*/enable_materialized_views: true/' "$conf"; \
    sed -i -E 's/^#? ?materialized_views_enabled:.*/materialized_views_enabled: true/' "$conf"; \
    grep -q '^authenticator: PasswordAuthenticator$' "$conf" || { echo "FATAL: authenticator not set"; exit 1; }; \
    grep -q '^authorizer: CassandraAuthorizer$' "$conf" || { echo "FATAL: authorizer not set"; exit 1; }; \
    grep -qE '^(enable_user_defined_functions|user_defined_functions_enabled): true$' "$conf" \
      || { echo "FATAL: user defined functions not enabled"; exit 1; }; \
    grep -qE '^(enable_materialized_views|materialized_views_enabled): true$' "$conf" \
      || { echo "FATAL: materialized views not enabled"; exit 1; }; \
    echo "settings applied:"; \
    grep -nE '^(authenticator|authorizer|.*user_defined_functions.*|.*materialized_views.*):' "$conf"
