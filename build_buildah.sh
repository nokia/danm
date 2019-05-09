#!/bin/sh

echo 'Building DANM builder container'
buildah bud --no-cache -t danm_builder:1.0 scm/build

echo 'Running DANM build'
build_container=$(buildah from danm_builder:1.0)
buildah run --net=host -v $GOPATH/bin:/go/bin -v $GOPATH/src:/go/src $build_container /bin/sh -c /build.sh
buildah rm $build_container

echo 'Cleaning up DANM builder container'
buildah  rmi -f danm_builder:1.0

echo 'DANM libraries successfully built!'
