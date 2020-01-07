#!/bin/sh -ex
export GOOS=linux
cd $GOPATH/src/github.com/nokia/danm
glide install --strip-vendor
go get -d github.com/vishvananda/netlink
go get github.com/containernetworking/plugins/pkg/ns
go get github.com/golang/groupcache/lru
LATEST_TAG=$(git describe --tags)
COMMIT_HASH=$(git rev-parse HEAD)
go install -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/danm
go install -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/netwatcher
go install -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/svcwatcher
go install -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/webhook
go install -a -ldflags "-extldflags '-static'" github.com/nokia/danm/cmd/fakeipam
go install -a -ldflags "-extldflags '-static'" github.com/nokia/danm/cmd/cnitest