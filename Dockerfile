FROM docker.io/library/alpine:latest
ARG TARGETARCH

RUN apk add --no-cache tini

ADD ./bin/k0sctl-linux-$TARGETARCH /usr/local/bin/k0sctl

ENTRYPOINT ["/sbin/tini", "--"]

CMD ["k0sctl"]
