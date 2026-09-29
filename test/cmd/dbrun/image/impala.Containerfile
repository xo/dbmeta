# Apache Impala in one container: the Hive metastore, the statestore, the
# catalog and one daemon that coordinates and executes.
#
# Apache publishes each of those as an image of its own, and its quickstart
# runs them as four containers. dbrun starts one container for a server, so
# this image builds on the daemon image and takes the metastore from the
# quickstart's metastore image. The daemon image already holds statestored
# and catalogd, because they are the impalad binary under other names, and
# Java 8, which the metastore needs too. Ken asked for Impala on 2026-09-30,
# which amends D118. See D145.
#
# The metastore keeps its catalog in Derby under /var/lib/hive, and the
# tables are files under /user/hive/warehouse, on the container's own
# filesystem, as the quickstart keeps them on a volume. Nothing is kept
# across a new container.
#
# Impala refuses the local filesystem as the default filesystem, and aborts
# on that as a configuration error, measured on 4.5.2. The tables here are
# files on the container's own filesystem, so the catalog and the daemon are
# told not to abort on it.
#
# The memory is set for dbmeta's limit of 4 GB. Each JVM is held to 512 MB,
# and the daemon's query memory to 1 GB.
ARG RELEASE=4.5.2
FROM docker.io/apache/impala:${RELEASE}-impala_quickstart_hms AS hms

ARG RELEASE=4.5.2
FROM docker.io/apache/impala:${RELEASE}-impalad_coord_exec

USER root
COPY --from=hms --chown=impala /opt/hive /opt/hive
COPY --from=hms --chown=impala /opt/hadoop /opt/hadoop
RUN mkdir -p /var/lib/hive/logs /user/hive/warehouse/managed /user/hive/warehouse/external /opt/dbmeta && \
    chown -R impala /var/lib/hive /user/hive /opt/dbmeta

# The quickstart's hive-site.xml, with the metastore on this host. Impala
# reads it from its conf directory, and the metastore from its own.
RUN printf '%s\n' \
    '<?xml version="1.0"?>' \
    '<configuration>' \
    '  <property><name>hive.metastore.uris</name><value>thrift://127.0.0.1:9083</value></property>' \
    '  <property><name>hive.metastore.warehouse.dir</name><value>/user/hive/warehouse/managed</value></property>' \
    '  <property><name>hive.metastore.warehouse.external.dir</name><value>/user/hive/warehouse/external</value></property>' \
    '  <property><name>hive.metastore.event.db.notification.api.auth</name><value>false</value></property>' \
    '  <property><name>hive.metastore.dml.events</name><value>true</value></property>' \
    '  <property><name>hive.support.concurrency</name><value>true</value></property>' \
    '  <property><name>hive.txn.manager</name><value>org.apache.hadoop.hive.ql.lockmgr.DbTxnManager</value></property>' \
    '  <property><name>javax.jdo.option.ConnectionDriverName</name><value>org.apache.derby.jdbc.EmbeddedDriver</value></property>' \
    '  <property><name>javax.jdo.option.ConnectionURL</name><value>jdbc:derby:;databaseName=/var/lib/hive/metastore/metastore_db;create=true</value></property>' \
    '  <property><name>hive.stats.autogather</name><value>false</value></property>' \
    '</configuration>' > /opt/dbmeta/hive-site.xml && \
    mkdir -p /opt/hive/conf && \
    cp /opt/dbmeta/hive-site.xml /opt/hive/conf/hive-site.xml && \
    cp /opt/dbmeta/hive-site.xml /opt/impala/conf/hive-site.xml && \
    chown -R impala /opt/hive/conf /opt/impala/conf

# Start the metastore, wait for its port, then the statestore and the
# catalog in the background, and the daemon in the foreground. printf
# writes each argument as a line of its own, so each command is one
# argument.
RUN printf '%s\n' \
    '#!/bin/bash' \
    'set -e' \
    'export JAVA_HOME=$(dirname $(dirname $(readlink -f $(command -v java))))' \
    'export HADOOP_HOME=/opt/hadoop' \
    'export JAVA_TOOL_OPTIONS="-Xmx512m"' \
    'if [ ! -d /var/lib/hive/metastore/metastore_db ]; then' \
    '  /opt/hive/bin/schematool -dbType derby -initSchema > /var/lib/hive/logs/schematool.log 2>&1' \
    'fi' \
    '/opt/hive/bin/hive --service metastore > /var/lib/hive/logs/metastore.log 2>&1 &' \
    'until (echo > /dev/tcp/127.0.0.1/9083) 2> /dev/null; do sleep 1; done' \
    'D=/opt/impala/bin/daemon_entrypoint.sh' \
    '$D /opt/impala/bin/statestored -redirect_stdout_stderr=false -logtostderr > /opt/impala/logs/statestored.out 2>&1 &' \
    '$D /opt/impala/bin/catalogd -redirect_stdout_stderr=false -logtostderr -abort_on_config_error=false -hms_event_polling_interval_s=1 -invalidate_tables_timeout_s=999999 > /opt/impala/logs/catalogd.out 2>&1 &' \
    'exec $D /opt/impala/bin/impalad -redirect_stdout_stderr=false -logtostderr -abort_on_config_error=false -mem_limit=1gb -mt_dop_auto_fallback=true -default_query_options=default_file_format=parquet,default_transactional_type=none' \
    > /opt/dbmeta/start.sh && \
    chmod +x /opt/dbmeta/start.sh

USER impala
ENTRYPOINT ["/opt/dbmeta/start.sh"]
