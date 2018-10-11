#!/bin/sh -ex
export CGO_ENABLED=0
export GOOS=linux
cd $GOPATH/src/nokia.net
glide install
go get -d github.com/vishvananda/netlink
go get github.com/golang/groupcache/lru
go get k8s.io/code-generator/cmd/deepcopy-gen
go get k8s.io/code-generator/cmd/client-gen
go get k8s.io/code-generator/cmd/lister-gen
go get k8s.io/code-generator/cmd/informer-gen
deepcopy-gen -v5 --alsologtostderr --input-dirs nokia.net/crd/apis/danm/v1 -O zz_generated.deepcopy --bounding-dirs nokia.net/crd/apis
client-gen -v5 --alsologtostderr --clientset-name versioned --input-base "" --input nokia.net/crd/apis/danm/v1 --clientset-path nokia.net/crd/client/clientset
lister-gen -v5 --alsologtostderr --input-dirs nokia.net/crd/apis/danm/v1 --output-package nokia.net/crd/client/listers
informer-gen -v5 --alsologtostderr --input-dirs nokia.net/crd/apis/danm/v1 --versioned-clientset-package nokia.net/crd/client/clientset/versioned --listers-package nokia.net/crd/client/listers --output-package nokia.net/crd/client/informers 
go install -a -ldflags '-extldflags "-static"' nokia.net/danm
go install -a -ldflags '-extldflags "-static"' nokia.net/watcher
go install -a -ldflags '-extldflags "-static"' nokia.net/fakeipam
go install -a -ldflags '-extldflags "-static"' nokia.net/svcwatcher
