FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG ENVIRONMENT
ARG GIT_COMMIT
ARG TAG_NAME

ENV CGO_ENABLED=0

RUN go build -trimpath -a \
      -tags "netgo,osusergo,static_build" \
      -installsuffix netgo \
      -ldflags "-s -w \
        -X github.com/k0sproject/k0sctl/version.Environment=${ENVIRONMENT} \
        -X github.com/carlmjohnson/versioninfo.Revision=${GIT_COMMIT} \
        -X github.com/carlmjohnson/versioninfo.Version=${TAG_NAME} \
        -extldflags '-static'" \
      -o /out/k0sctl .

FROM docker.io/library/alpine:latest

RUN apk add --no-cache tini openssh ca-certificates

COPY --from=builder /out/k0sctl /k0sctl

ENTRYPOINT ["/sbin/tini", "--"]

USER 65534:65534

CMD ["k0sctl"]
