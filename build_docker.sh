#!/bin/sh -e

echo 'Updating alpine base image'
docker pull golang:1.13-alpine3.10

echo 'Building DANM builder container'
docker build --no-cache --tag=danm_builder:1.0 scm/build

echo 'Running DANM build'
docker run --rm --net=host --name=danm_build -v ${GOPATH}/bin:/go/bin -v ${GOPATH}/src:/go/src -v ${GOPATH}/pkg:/go/pkg danm_builder:1.0

echo 'Cleaning up DANM builder container'
docker rmi -f danm_builder:1.0

echo 'DANM libraries successfully built!'
