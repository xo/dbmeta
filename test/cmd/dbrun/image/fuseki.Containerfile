# Apache Jena Fuseki.
#
# Apache publishes no image. It publishes the Fuseki server as a jar on Maven
# Central for each release, and this puts that jar on the Java 21 runtime,
# which Jena 6 needs. The entry in container/fuseki.go writes the
# configuration and starts the server.
ARG RELEASE
FROM docker.io/library/eclipse-temurin:21-jre
ARG RELEASE
ADD https://repo1.maven.org/maven2/org/apache/jena/jena-fuseki-server/${RELEASE}/jena-fuseki-server-${RELEASE}.jar /opt/fuseki/fuseki-server.jar
RUN chmod 644 /opt/fuseki/fuseki-server.jar
EXPOSE 3030
