#target: danm-deployer
FROM alpine:3.8
MAINTAINER Nandor Galambosi <nandor.galambosi@nokia-bell-labs.com>
COPY .cache/kubectl /opt/bin/
COPY bin/* /opt/bin/
COPY deploy.sh /
ENTRYPOINT ["/deploy.sh"]
