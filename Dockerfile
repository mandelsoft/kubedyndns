ARG TARGETS=dev
ARG NAME=coredns

#############      builder       #############
FROM golang:1.24.2 AS builder
ARG TARGETS

WORKDIR /go/src/github.com/mandelsoft/kubedyndns
COPY . .

RUN make $TARGETS

#############      certs     #############
FROM debian:stable-slim AS certs

ENV DEBIAN_FRONTEND=noninteractive

RUN echo "deb http://deb.debian.org/debian stable main" > /etc/apt/sources.list
RUN apt-get update && apt-get -uy upgrade
RUN apt-get -y --no-install-recommends install ca-certificates && update-ca-certificates
#RUN apt-get -y install ca-certificates && update-ca-certificates

#############      image     #############
FROM scratch AS image
ARG NAME
WORKDIR /
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /go/bin/$NAME /main

EXPOSE 53 53/udp
ENTRYPOINT [ "/main" ]
