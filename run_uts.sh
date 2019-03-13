#!/usr/bin/env bash
set -e
echo "" > coverage.out
for d in $(go list ./... | grep -v vendor | grep -v crd); do
    go test -covermode=count -v -coverprofile=profile.out -coverpkg=github.com/nokia/danm/pkg/cnidel,github.com/nokia/danm/pkg/bitarray,github.com/nokia/danm/pkg/ipam,github.com/nokia/danm/pkg/danmep,github.com/nokia/danm/pkg/netcontrol,github.com/nokia/danm/pkg/syncher,github.com/nokia/danm/pkg/metacni,github.com/nokia/danm/pkg/svccontrol $d
    if [ -f profile.out ]; then
        cat profile.out >> coverage.out
        rm profile.out
    fi
done
awk '! a[$0]++' coverage.out > coverage2.out
awk 'NF' coverage2.out > coverage3.out
