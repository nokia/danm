#!/bin/sh -e

echo 'Updating alpine base image'
docker pull golang:1.13-alpine3.10

echo 'Building DANM UT container'
docker build --no-cache --tag=danm_ut:1.0 scm/ut

echo 'Running DANM UT'
docker run --rm --net=host --name=danm_ut -v ${GOPATH}/bin:/go/bin -v ${GOPATH}/src:/go/src -v ${GOPATH}/pkg:/go/pkg -v /var/log:/var/log danm_ut:1.0

echo 'Cleaning up DANM UT container'
docker rmi -f danm_ut:1.0

echo 'DANM tests were successfully executed!'
