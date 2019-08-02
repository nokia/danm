FROM alpine:latest
MAINTAINER Levente Kale <levente.kale@nokia.com>

COPY webhook /usr/local/bin/webhook

RUN apk add --no-cache --virtual .tools curl libcap iputils  \
&&  adduser -u 147 -D -H -s /sbin/nologin danm \
&&  chown root:danm /usr/local/bin/webhook \
&&  chmod 750 /usr/local/bin/webhook \
&&  apk del .tools

USER danm

WORKDIR /

ENTRYPOINT ["/usr/local/bin/webhook"]
