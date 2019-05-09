FROM alpine:latest
MAINTAINER Levente Kale <levente.kale@nokia.com>

COPY svcwatcher /usr/local/bin/svcwatcher

RUN apk add --no-cache --virtual .tools curl libcap iputils  \
&&  adduser -u 147 -D -H -s /sbin/nologin danm \
&&  chown root:danm /usr/local/bin/svcwatcher \
&&  chmod 750 /usr/local/bin/svcwatcher \
&&  apk del .tools

USER danm

WORKDIR /
ENTRYPOINT ["/usr/local/bin/svcwatcher"]
