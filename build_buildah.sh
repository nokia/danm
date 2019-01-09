#!/bin/sh

echo 'Building DANM builder container'
buildah bud --no-cache -t danm_builder:1.0 build/

echo 'Running DANM build'
podman run --rm=true --net=host --name="danm_build" -v $GOPATH/bin:/go/bin -v $GOPATH/src:/go/src danm_builder:1.0

echo 'Cleaning up DANM builder container'
buildah  rmi -f danm_builder:1.0

echo 'DANM libraries successfully built!'
