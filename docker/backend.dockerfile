FROM golang:1.14.5-alpine3.12 AS builder
RUN apk add --update --no-cache ca-certificates tzdata && update-ca-certificates

ADD . /opt
WORKDIR /opt

RUN git update-index --refresh; make backend

FROM scratch as runner

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /opt/backend /bin/backend

LABEL vendor="kakkoyun" \
    name="observable-remote-write-backend" \
    description="An application to demonstrate observability and instrumentation tools which conforms Prometheus Remote Write protocol." \
    maintainer="Kemal Akkoyun <kakkoyun@gmail.com>" \
    org.label-schema.description="An application to demonstrate observability and instrumentation tools which confroms Prometheus Remote Write protocol." \
    org.label-schema.docker.cmd="docker run --rm -v '$(pwd)':/app -e kakkoyun/observable-remote-write-backend" \
    org.label-schema.vcs-url="https://github.com/kakkoyun/observable-remote-write" \
    org.label-schema.vendor="kakkoyun" \
    org.label-schema.usage="https://kakkoyun.github.io/observable-remote-write" \
    org.opencontainers.image.authors="Kemal Akkoyun <kakkoyun@gmail.com>" \
    org.opencontainers.image.url="https://github.com/kakkoyun/observable-remote-write" \
    org.opencontainers.image.documentation="https://kakkoyun.github.io/observable-remote-write" \
    org.opencontainers.image.source="https://github.com/kakkoyun/observable-remote-write/blob/master/docker/backend.dockerfile" \
    org.opencontainers.image.vendor="kakkoyun" \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.title="observable-remote-write-backend" \
    org.opencontainers.image.description="An application to demonstrate observability and instrumentation tools which confroms Prometheus Remote Write protocol." \

ENTRYPOINT ["/bin/backend"]
