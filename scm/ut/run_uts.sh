#!/usr/bin/env bash

function run_uts {
  echo "" > coverage.out
  for d in $(go list ./... | grep -v vendor | grep -v crd); do
      go test -covermode=count -v -coverprofile=profile.out -coverpkg=github.com/nokia/danm/pkg/... "${d}"
      if [ -f profile.out ]; then
          cat profile.out >> coverage.out
          rm profile.out
      fi
  done
  awk '! a[$0]++' coverage.out > coverage2.out
  awk 'NF' coverage2.out > coverage3.out
}

function check_codegen_tampering {
  local CRDDIR="${GOPATH}/src/github.com/nokia/danm/crd"
  rm -rf ${CRDDIR}test
  cp -r ${CRDDIR}{,test}
  rm -r ${CRDDIR}/client "${CRDDIR}/apis/danm/v1/zz_generated.deepcopy.go"
  go generate ./...
  set +e
  DID_YOU_TAMPER="$(diff -qr ${CRDDIR}{,test})"
}

set -e
export CGO_ENABLED=0
cd ${GOPATH}/src/github.com/nokia/danm

run_uts
check_codegen_tampering
if [ -n "${DID_YOU_TAMPER}" ]
then
  echo "${DID_YOU_TAMPER}"
  echo "Generated DANM client code was manually modified in the working copy, failing tests!"
  exit 1
fi

cp ${GOPATH}/src/github.com/nokia/danm/coverage*.out /coverage/
