#!/usr/bin/env bash

function run_uts {
  cd $GOPATH/src/github.com/nokia/danm
  echo "" > coverage.out
  for d in $(go list ./... | grep -v vendor | grep -v crd); do
      go test -covermode=count -v -coverprofile=profile.out -coverpkg=github.com/nokia/danm/pkg/cnidel,github.com/nokia/danm/pkg/bitarray,github.com/nokia/danm/pkg/ipam,github.com/nokia/danm/pkg/danmep,github.com/nokia/danm/pkg/netcontrol,github.com/nokia/danm/pkg/syncher,github.com/nokia/danm/pkg/metacni,github.com/nokia/danm/pkg/svccontrol,github.com/nokia/danm/pkg/admit,github.com/nokia/danm/pkg/confman $d
      if [ -f profile.out ]; then
          cat profile.out >> coverage.out
          rm profile.out
      fi
  done
  awk '! a[$0]++' coverage.out > coverage2.out
  awk 'NF' coverage2.out > coverage3.out
}

function check_codegen_tampering {
  local TESTDIR=github.com/nokia/danm/crd/testclient
  local CLIENTDIR=github.com/nokia/danm/crd/client
  rm -rf $GOPATH/src/k8s.io/code-generator
  rm -rf $GOPATH/src/$TESTDIR
  git clone -b 'kubernetes-1.13.4' --depth 1 https://github.com/kubernetes/code-generator.git $GOPATH/src/k8s.io/code-generator
  go install k8s.io/code-generator/cmd/client-gen
  go install k8s.io/code-generator/cmd/lister-gen
  go install k8s.io/code-generator/cmd/informer-gen
  mkdir -p $GOPATH/src/$TESTDIR
  mv $GOPATH/src/$CLIENTDIR/* $GOPATH/src/$TESTDIR/
  $GOPATH/bin/client-gen --alsologtostderr --clientset-name versioned --input-base "" --input github.com/nokia/danm/crd/apis/danm/v1 --clientset-path $CLIENTDIR/clientset
  $GOPATH/bin/lister-gen --alsologtostderr --input-dirs github.com/nokia/danm/crd/apis/danm/v1 --output-package $CLIENTDIR/listers
  $GOPATH/bin/informer-gen --alsologtostderr --input-dirs github.com/nokia/danm/crd/apis/danm/v1 --versioned-clientset-package $CLIENTDIR/clientset/versioned --listers-package $CLIENTDIR/listers --output-package $CLIENTDIR/informers
  set +e
  DID_YOU_TAMPER="$(diff -qr $GOPATH/src/$CLIENTDIR $GOPATH/src/$TESTDIR)"
}

set -e
run_uts
check_codegen_tampering
if [ -n "$DID_YOU_TAMPER" ]
then
  echo $DID_YOU_TAMPER
  echo "Generated DANM client code was manually modified in the working copy, failing tests!"
  exit 1
fi