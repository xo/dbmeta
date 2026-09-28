# ksqlDB with the Kafka broker it needs, in one container.
#
# ksqlDB keeps its state in Kafka, and dbrun has no start order between two
# containers. The ksqlDB image carries some Kafka jars and not the broker, so
# this copies the broker from Confluent's Kafka image of the same release.
# The two images share a base and a Java. The entry in container/ksqldb.go
# starts both and sets them up.
ARG RELEASE
FROM docker.io/confluentinc/cp-kafka:${RELEASE} AS kafka

FROM docker.io/confluentinc/cp-ksqldb-server:${RELEASE}
COPY --from=kafka /usr/share/java/kafka /usr/share/java/kafka
COPY --from=kafka /usr/bin/kafka-run-class /usr/bin/kafka-server-start /usr/bin/kafka-storage /usr/bin/
COPY --from=kafka /etc/kafka /etc/kafka
EXPOSE 8088
