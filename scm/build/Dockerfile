FROM alpine:latest
MAINTAINER Levente Kale <levente.kale@nokia.com>

ENV GOPATH=/usr/local/go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
ENV GO_VERSION 1.12.9

COPY build.sh /build.sh

RUN apk add --no-cache ca-certificates \
 && apk update --no-cache \
 && apk upgrade --no-cache \
 && apk add --no-cache make gcc musl-dev glide git bash curl tar \
 && mkdir -p ${GOPATH} \
 && mkdir -p ${GOPATH}/bin \
 && mkdir -p ${GOPATH}/src \
 && curl -fsSL -k https://dl.google.com/go/go${GO_VERSION}.src.tar.gz | tar zx --strip-components=1 -C ${GOPATH} \
 && cd ${GOPATH}/src/ \
 && ./make.bash \
 && rm -rf /var/cache/apk/* \
 && rm -rf /var/lib/apt/lists/* \
 && rm -rf /tmp/*

ENTRYPOINT /build.sh
