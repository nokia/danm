FROM alpine:3.9
MAINTAINER Levente Kale <levente.kale@nokia.com>

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
ENV GOOS=linux

WORKDIR /

RUN mkdir -p $GOPATH/bin \
&&  mkdir -p $GOPATH/src

RUN apk add --no-cache libcap iputils

RUN apk add --no-cache --virtual .tools ca-certificates gcc musl-dev go glide git tar curl \
&& mkdir -p $GOPATH/src/github.com/nokia/danm \
&& git clone -b 'webhook' --depth 1 https://github.com/nokia/danm.git $GOPATH/src/github.com/nokia/danm \
&& cd $GOPATH/src/github.com/nokia/danm \
&& glide install --strip-vendor \
&& go get -d github.com/vishvananda/netlink \
&& go get github.com/containernetworking/plugins/pkg/ns \
&& go get github.com/golang/groupcache/lru \
&& rm -rf $GOPATH/src/k8s.io/code-generator \
&& git clone -b 'kubernetes-1.13.4' --depth 1 https://github.com/kubernetes/code-generator.git $GOPATH/src/k8s.io/code-generator \
&& go install k8s.io/code-generator/cmd/deepcopy-gen \
&& go install k8s.io/code-generator/cmd/client-gen \
&& go install k8s.io/code-generator/cmd/lister-gen \
&& go install k8s.io/code-generator/cmd/informer-gen \
&& deepcopy-gen --alsologtostderr --input-dirs github.com/nokia/danm/crd/apis/danm/v1 -O zz_generated.deepcopy --bounding-dirs github.com/nokia/danm/crd/apis \
&& client-gen --alsologtostderr --clientset-name versioned --input-base "" --input github.com/nokia/danm/crd/apis/danm/v1 --clientset-path github.com/nokia/danm/crd/client/clientset \
&& lister-gen --alsologtostderr --input-dirs github.com/nokia/danm/crd/apis/danm/v1 --output-package github.com/nokia/danm/crd/client/listers \
&& informer-gen --alsologtostderr --input-dirs github.com/nokia/danm/crd/apis/danm/v1 --versioned-clientset-package github.com/nokia/danm/crd/client/clientset/versioned --listers-package github.com/nokia/danm/crd/client/listers --output-package github.com/nokia/danm/crd/client/informers \
&& go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/cmd/webhook \
&& cp $GOPATH/bin/webhook /usr/local/bin/webhook \
&& rm -rf $GOPATH/src \
&& rm -rf $GOPATH/bin \
&& apk del .tools \
&& rm -rf /var/cache/apk/* \
&& rm -rf /var/lib/apt/lists/* \
&& rm -rf /tmp/* \
&& rm -rf ~/.glide

ENTRYPOINT ["/usr/local/bin/webhook"]
