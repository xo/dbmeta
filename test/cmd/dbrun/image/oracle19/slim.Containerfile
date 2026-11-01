# The 19c image that containers run on once the kept database exists (D248).
#
# Oracle's own build makes a 7 GB image. It holds everything DBCA needs to
# build a database, and a container that starts on a kept database needs none
# of it. This file removes what the server does not read at run time, and
# --squash-all makes the removals shrink the image. The result is about 2.3 GB.
#
# The first build of the database still runs on the full image, because DBCA is
# a Java program that needs the JDK and the assistants.
#
# What stays: the oracle binary, the shared libraries it loads, the message and
# timezone files, SQL*Plus, the network files and the Java virtual machine,
# because the database holds Java classes that the server loads. What goes:
# the other products in the home, the static libraries, the math and backup
# libraries, and the spatial, text and OLAP components, which this database
# holds as dictionary objects that nothing here reads.
#
# libedtn19.a stays, because the entrypoint reads it to learn the edition.
FROM localhost/oracle/database:19.3.0-ee
USER root
RUN cd /opt/oracle/product/19c/dbhome_1 \
 && rm -rf OPatch QOpatch assistants jdk jdbc sdk demo dbjava deinstall oui cv R perl diagnostics \
           md ctx ord olap owm dv mgw sqlj hs drdaas odbc opmn racg slax srvm usm utl wwg xdk \
           install/*.zip relnotes instantclient inventory/backup crs css has schagent.conf clone \
 && find lib -name "*.a" ! -name libedtn19.a -delete \
 && rm -f lib/libmkl* lib/libcrs*.so lib/libosbws.so lib/libopc.so lib/libra.so lib/libolapapi*.so \
          lib/libsvml* lib/libimf* lib/libirc* bin/afdboot bin/proc bin/procob bin/wrap bin/orion bin/rman \
 && rm -rf /var/cache /var/lib/rpm-state /tmp/* /opt/oracle/cfgtoollogs /opt/oracle/oraInventory/logs
USER oracle
