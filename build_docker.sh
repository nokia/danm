#!/bin/bash -xe

echo 'Updating alpine base image'
docker pull alpine:latest

version=3.1.0
echo 'Building DANM builder container'
docker build --build-arg http_proxy=$http_proxy --build-arg https_proxy=$https_proxy --tag=danm_builder:1.0 scm/build

echo 'Running DANM build'
docker run --net=host -e http_proxy=$http_proxy -e https_proxy=$https_proxy --name=danm_build danm_builder:1.0

echo 'Get Binaries'
rm -rf bin
mkdir bin
docker cp danm_build:/go/bin/danm bin/
docker cp danm_build:/go/bin/fakeipam bin/
docker cp danm_build:/go/bin/netwatcher bin/
docker cp danm_build:/go/bin/svcwatcher bin/

echo 'Cleaning up DANM builder container'
docker rm danm_build
docker rmi -f danm_builder:1.0

echo 'Building containers'
pushd integration/deployment/
./prebuild.sh
docker build -t danm-deployer:$version .
popd


cp -f bin/netwatcher integration/docker/netwatcher/
pushd integration/docker/netwatcher
docker build --build-arg http_proxy=$http_proxy --build-arg https_proxy=$https_proxy --tag=netwatcher:$version .
popd

cp -f bin/svcwatcher integration/docker/svcwatcher/
pushd integration/docker/svcwatcher
docker build --build-arg http_proxy=$http_proxy --build-arg https_proxy=$https_proxy --tag=svcwatcher:$version .
popd

if [ -n "$DOCKERREGISTRY" ];then
  for image in netwatcher:$version svcwatcher:$version danm-deployer:$version;do
    docker tag $image $DOCKERREGISTRY/$image
    docker push $DOCKERREGISTRY/$image
  done
fi

echo 'DANM libraries successfully built!'
