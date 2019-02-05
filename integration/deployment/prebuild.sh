#!/bin/bash
mkdir -p .cache
if [ ! -f .cache/kubectl ];then
	pushd .cache
	curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
	chmod +x kubectl
	popd
fi
COMBINED=danm-install.yaml
cp -r ../../bin .
rm -f $COMBINED
for yaml in ../crds/DanmEp.yaml ../crds/DanmNet.yaml ../manifests/netwatcher/0netwatcher_rbac.yaml ../manifests/netwatcher/netwatcher_ds.yaml ../manifests/svcwatcher/0svcwatcher_rbac.yaml ../manifests/svcwatcher/svcwatcher_ds.yaml deployer.yaml;do 
  echo '---'
  cat $yaml
done |sed 's#\(.*image:[ ]*\)\([^ ]\)#\1'${DOCKERREGISTRY}'\2#' >$COMBINED


