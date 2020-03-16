FROM golang:1.13-alpine3.10
MAINTAINER Levente Kale <levente.kale@nokia.com>

RUN apk add --no-cache ca-certificates make git bash

COPY build.sh /build.sh
ENTRYPOINT /build.sh
