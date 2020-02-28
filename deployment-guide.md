# DANM Deployment Guide
## Table of Contents
* [Getting started](#getting-started)
     * [Prerequisites](#prerequisites)
     * [Building the binaries](#building-the-binaries)
     * [Building the containers](#building-the-containers)
* [Deployment](#deployment)

## Getting started
### Prerequisites
Otherwise, you need to create your own Kubernetes cluster, and install DANM manually. We suggest to use any of the automated Kubernetes installing solutions (kubeadm, minikube etc.) for a painless experience.
We currently test DANM with Kubernetes 1.17.X.
Compatibility with earlier than 1.9.X versions of Kubernetes is not officially supported.
**Running with pre-1.15.X versions have known issues when used together with the production-grade network management APIs. These originate from Kubernetes core code limitations.**
Best bet is to always stay up-to-date!

The project does not currently have a Docker container release, so we will walk you through the entire process of building all artifacts from scratch.
To be able to do that, your development environment shall already have Docker daemon installed and ready to build containers.

Note, that the project itself depends on Golang 1.10+ and glide being available, but we packaged these dependencies into an automatically created builder container, so you don't have to worry about them!
### Building the binaries

It is actually as easy as go get-ting the repository from GitHub, and executing the build_danm.sh script from the root of the project!
```
go get -d github.com/nokia/danm
cd $GOPATH/src/github.com/nokia/danm
./build_danm.sh
```
This will first build the Alpine based builder container, mount the $GOPATH/src and the $GOPATH/bin directory into it, and invoke the necessary script to build all binaries inside the container.
The builder container destroys itself once its purpose has been fulfilled.

The result will be 6, statically linked binaries put into your $GOPATH/bin directory.

- danm

- fakeipam

- netwatcher

- svcwatcher

- webhook

### Building the containers
Netwatcher, svcwatcher, and webhook binaries are built into their own containers.
The project contains example Dockerfiles for all of these components under the integration/docker directory.

**Copy the respective binary into the right folder (netwatcher into integration/docker/netwatcher, svcwatcher into integration/docker/svcwatcher, webhook into integration/docker/webhook), then execute:**
```
docker build -t netwatcher:latest integration/docker/netwatcher
docker build -t svcwatcher:latest integration/docker/svcwatcher
docker build -t webhook:latest integration/docker/webhook
```
or
```
buildah bud -t netwatcher:latest integration/docker/netwatcher
buildah bud -t svcwatcher:latest integration/docker/svcwatcher
buildah bud -t webhook:latest integration/docker/webhook
```
This builds the respective containers. Afterwards, these containers can be directly integrated into a running Kubernetes cluster!
## Deployment
The method of deploying the whole DANM suite into a Kubernetes cluster is the following.

**1A. Extend the Kubernetes API with the DanmNet and DanmEp CRD objects for a simplified network management experience by executing the following command from the project's root directory:**
```
kubectl create -f integration/crds/lightweight
```
**1B. Extend the Kubernetes API with the TenantNetwork, ClusterNetwork, TenantConfig, and DanmEp CRD objects for a multi-tenant capable, production-grade network management experience by executing the following command from the project's root directory:**
```
kubectl create -f integration/crds/production
```
**2. Put a valid CNI config file into the CNI configuration directory of all your kubelet nodes' (by default it is /etc/cni/net.d/) based on:**

[Example CNI config file](https://github.com/nokia/danm/tree/master/integration/cni_config/00-danm.conf)

As kubelet considers the first .conf file in the configured directory as the valid CNI config of the cluster, it is generally a good idea to prefix the .conf file of any CNI metaplugin with "00".
Make sure to configure the optional DANM configuration parameters to match your environment!
The parameter "kubeconfig" is mandatory, and shall point to a valid kubeconfig file.
You can find an example file here:

[Example kubeconf file](https://github.com/nokia/danm/tree/master/integration/cni_config/example_kubeconfig.yaml)

Don't forget to also provision the necessary RBAC rules so DANM can do its job:

[RBAC rules](https://github.com/nokia/danm/tree/master/integration/cni_config/danm_rbac.yaml)

**3. Copy the "danm" binary into the configured CNI plugin directory of all your kubelet nodes' (by default it is /opt/cni/bin/):**
```
/ # ls /opt/cni/bin
bridge       dhcp         flannel      host-local   loopback     portmap      sample       tuning
**danm**     host-device  ipvlan       macvlan      ptp          sriov        vlan
```
**4. Copy the "fakeipam" binary into the configured CNI plugin directory of all your kubelet nodes' (by default it is /opt/cni/bin/):**
```
/ # ls /opt/cni/bin
bridge       dhcp         flannel      host-local   loopback     portmap      sample       tuning
danm        **fakeipam**      host-device  ipvlan       macvlan      ptp          sriov        vlan
```
**5. OPTIONAL: Copy any CNI binaries (flannel, sriov, macvlan etc.) you would like to use in your cluster into the configured CNI plugin directory of all your kubelet nodes' (by default it is /opt/cni/bin/)**

**6. Onboard the netwatcher, svcwatcher, and webhook containers into the image registry of your cluster**

 **7. Create the netwatcher DaemonSet by executing the following command from the project's root directory:**
 ```
kubectl create -f integration/manifests/netwatcher/
```
Note1: you should take a look at the example manifest, and possibly tailor it to your own environment first
Note2: we assume RBAC is configured for the Kubernetes API, so the manifests include the required Role and ServiceAccount for this case.

 **8. Create at least one DANM network to bootstrap your infrastructure Pods!**
 Otherwise you can easily fall into a catch 22 situation - you won't be able to bring-up Pods because you don't have network, but you cannot create networks because you cannot bring-up a Pod to validate them.
 Your bootstrap networking solution can be really anything you fancy!
 We use Flannel or Calico for the purpose in our environments, and connect Pods to it with such simple network descriptors like what you can find in **integration/bootstrap_networks**.

 **9. Create the webhook Deployment and provide it with certificates by executing the following commands from the project's root directory:**
 ```
./integration/manifests/webhook/webhook-create-signed-cert.sh
cat ./integration/manifests/webhook/webhook.yaml | ./integration/manifests/webhook/webhook-patch-ca-bundle.sh > ./integration/manifests/webhook/webhook-ca-bundle.yaml
kubectl create -f integration/manifests/webhook/webhook-ca-bundle.yaml
```
**Disclaimer**: webhook already leverages DANM CNI to create its network interface. Don't forget to change the name of the network referenced in the example manifest file to your bootstrap network!
We also assume RBAC is configured in your cluster.

***You are now ready to use the services of DANM, and can start bringing-up Pods within your cluster!***

 **+1. OPTIONAL: Create the svcwatcher Deployment by executing the following command from the project's root directory:**
 ```
kubectl create -f integration/manifests/svcwatcher/
```
This component is an optional part of the suite. You only need to install it if you would like to use Kubernetes Services for all the network interfaces of your Pod - but who wouldn't want that??
**Disclaimer**: svcwatcher, and webhook already leverage DANM CNI to create their network interface. Don't forget to configure an appropriate default network in your cluster before you instantiate them!
We use Flannel, or Calico for this purpose in our infrastructures.
We also assume RBAC is configured in your cluster.
