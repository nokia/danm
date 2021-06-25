#!/bin/sh -ex
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

export GOOS=linux
# Force turn off CGO enables building pure static binaries, otherwise
# built binary still depends on and dinamically linked against the build
# environments standard library implementation (e.g. glibc/musl/...)
export CGO_ENABLED=0
cd "${GOPATH}/src/github.com/nokia/danm"
go mod vendor

#
# If we're being invoked inside Docker by the /build_danm script, use the
# COMMIT_HASH and LATEST_TAG values that were already set by the invoking script.
# Otherwise (eg if build.sh invoked directly during development cycle), set them
# here.
#
if [ -z "${COMMIT_HASH}" ]
then
  COMMIT_HASH=$(git rev-parse HEAD)
fi
if [ -z "${LATEST_TAG}" ]
then
  LATEST_TAG=$(git describe --tags)
fi

go install -mod=vendor -a -ldflags "-extldflags '-static' -X main.version=${LATEST_TAG} -X main.commitHash=${COMMIT_HASH}" github.com/nokia/danm/cmd/...
