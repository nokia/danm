#!/bin/sh -ex
export GOOS=linux
# Force turn on go modules feature becase the project is inside GOPATH
export GO111MODULE=on
# Force turn off CGO enables building pure static binaries, otherwise
# built binary still depends on and dinamically linked against the build
# environments standard library implementation (e.g. glibc/musl/...)
export CGO_ENABLED=0
cd "${GOPATH}/src/github.com/nokia/danm"
go mod vendor
LATEST_TAG=$(git describe --tags)
COMMIT_HASH=$(git rev-parse HEAD)
go install -mod=vendor -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/...