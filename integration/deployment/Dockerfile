#target: danm-deployer
FROM alpine:3.8
MAINTAINER Nandor Galambosi <nandor.galambosi@nokia-bell-labs.com>
COPY bin/danm /opt/bin/
COPY bin/fakeipam /opt/bin/
COPY deploy.sh /
ENTRYPOINT ["/deploy.sh"]
