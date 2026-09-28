# The H2 database server.
#
# H2 publishes no image. It publishes a jar on Maven Central for each
# release, and this puts that jar on the Java 21 runtime. The entry in
# container/h2.go starts the server and sets it up.
ARG RELEASE
FROM docker.io/library/eclipse-temurin:21-jre
ARG RELEASE
ADD https://repo1.maven.org/maven2/com/h2database/h2/${RELEASE}/h2-${RELEASE}.jar /opt/h2.jar
RUN chmod 644 /opt/h2.jar
EXPOSE 9092
