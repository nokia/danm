#!/bin/sh -ex
export GOOS=linux
cd $GOPATH/src/github.com/nokia/danm/pkg
glide install --strip-vendor
go get -d github.com/vishvananda/netlink
go get github.com/containernetworking/plugins/pkg/ns
go get github.com/golang/groupcache/lru
rm -rf $GOPATH/src/k8s.io/code-generator
git clone -b 'kubernetes-1.13.4' --depth 1 https://github.com/kubernetes/code-generator.git $GOPATH/src/k8s.io/code-generator
go install k8s.io/code-generator/cmd/deepcopy-gen
go install k8s.io/code-generator/cmd/client-gen
go install k8s.io/code-generator/cmd/lister-gen
go install k8s.io/code-generator/cmd/informer-gen
deepcopy-gen --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 -O zz_generated.deepcopy --bounding-dirs github.com/nokia/danm/pkg/crd/apis
client-gen --alsologtostderr --clientset-name versioned --input-base "" --input github.com/nokia/danm/pkg/crd/apis/danm/v1 --clientset-path github.com/nokia/danm/pkg/crd/client/clientset
lister-gen --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --output-package github.com/nokia/danm/pkg/crd/client/listers
informer-gen --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --versioned-clientset-package github.com/nokia/danm/pkg/crd/client/clientset/versioned --listers-package github.com/nokia/danm/pkg/crd/client/listers --output-package github.com/nokia/danm/pkg/crd/client/informers 
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/danm
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/netwatcher
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/fakeipam
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/svcwatcher
