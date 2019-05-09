FROM alpine:latest
MAINTAINER Levente Kale <levente.kale@nokia.com>

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

COPY run_uts.sh /run_uts.sh

RUN apk add --no-cache ca-certificates \
 && apk update --no-cache \
 && apk upgrade --no-cache \
 && apk add --no-cache make gcc musl-dev go glide git bash \
 && mkdir -p $GOPATH/bin \
 && mkdir -p $GOPATH/src \
 && rm -rf /var/cache/apk/* \
 && rm -rf /var/lib/apt/lists/* \
 && rm -rf /tmp/*

ENTRYPOINT /run_uts.sh
