#!/bin/sh -e

echo 'Updating alpine base image'
docker pull alpine:latest

echo 'Building DANM builder container'
docker build --no-cache --tag=danm_builder:1.0 scm/build

echo 'Running DANM build'
docker run --rm --net=host --name=danm_build -v $GOPATH/bin:/usr/local/go/bin -v $GOPATH/src:/usr/local/go/src danm_builder:1.0

echo 'Cleaning up DANM builder container'
docker rmi -f danm_builder:1.0

echo 'DANM libraries successfully built!'
