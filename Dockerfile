#############      builder       #############
FROM golang:1.14.2 AS builder

ARG TARGETS=dev
ARG NAME=coredns

WORKDIR /go/src/github.com/mandelsoft/kubedyndns
COPY . .

RUN make $TARGETS

#############      certs     #############
FROM debian:stable-slim AS certs

RUN apt-get update && apt-get -uy upgrade
RUN apt-get -y install ca-certificates && update-ca-certificates

#############      image     #############
FROM scratch AS image

WORKDIR /
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /go/bin/$NAME /$NAME

EXPOSE 53 53/udp
ENTRYPOINT ["/$NAME"]
