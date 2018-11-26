#!/bin/sh -ex
export CGO_ENABLED=0
export GOOS=linux
cd $GOPATH/src/github.com/nokia/danm/pkg
glide install
go get -d github.com/vishvananda/netlink
go get github.com/containernetworking/plugins/pkg/ns
go get github.com/golang/groupcache/lru
go get k8s.io/code-generator/cmd/deepcopy-gen
go get k8s.io/code-generator/cmd/client-gen
go get k8s.io/code-generator/cmd/lister-gen
go get k8s.io/code-generator/cmd/informer-gen
deepcopy-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 -O zz_generated.deepcopy --bounding-dirs github.com/nokia/danm/pkg/crd/apis
client-gen -v5 --alsologtostderr --clientset-name versioned --input-base "" --input github.com/nokia/danm/pkg/crd/apis/danm/v1 --clientset-path github.com/nokia/danm/pkg/crd/client/clientset
lister-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --output-package github.com/nokia/danm/pkg/crd/client/listers
informer-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --versioned-clientset-package github.com/nokia/danm/pkg/crd/client/clientset/versioned --listers-package github.com/nokia/danm/pkg/crd/client/listers --output-package github.com/nokia/danm/pkg/crd/client/informers 
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/danm
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/netwatcher
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/fakeipam
go install -a -ldflags '-extldflags "-static"' github.com/nokia/danm/pkg/svcwatcher
