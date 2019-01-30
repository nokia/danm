FROM alpine:latest
MAINTAINER Levente Kale <levente.kale@nokia.com>

COPY netwatcher /usr/local/bin/netwatcher

RUN apk add --no-cache --virtual .tools curl libcap iputils  \
&&  adduser -u 147 -D -H -s /sbin/nologin danm \
&&  chown root:danm /usr/local/bin/netwatcher \
&&  chmod 750 /usr/local/bin/netwatcher \
&&  setcap cap_sys_ptrace,cap_sys_admin,cap_net_admin=eip /usr/local/bin/netwatcher \
&&  setcap cap_net_raw=eip /usr/sbin/arping \
&&  apk del .tools

USER danm

WORKDIR /
ENTRYPOINT ["/usr/local/bin/netwatcher"]
