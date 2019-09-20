
# DANM
[![Build Status](https://travis-ci.org/nokia/danm.svg?branch=master)](https://travis-ci.org/Nokia/danm)
[![Unit Test Coverage](https://coveralls.io/repos/github/nokia/danm/badge.svg?branch=master)](https://coveralls.io/github/nokia/danm?branch=master)

<img src="https://github.com/nokia/danm/raw/master/logo_w_name.png" width="100">

# Join our community!
Want to hang-out with us? Join our Slack under https://danmws.slack.com/!

Feel yourself officially invited by clicking on [this](https://join.slack.com/t/danmws/shared_invite/enQtNzEzMTQ4NDM2NTMxLTA3MDM4NGM0YTRjYzlhNGRiMDVlZWRlMjdlNTkwNTBjNWUyNjM0ZDQ3Y2E4YjE3NjVhNTE1MmEyYzkyMDRlNWU) link!

## Table of Contents
* [Table of Contents](#table-of-contents)
* [Introduction](#introduction)
* [Our philosophy and motivation behind DANM](#our-philosophy-and-motivation-behind-danm)
* [Note about boundaries](#note-about-boundaries)
* [Getting started](#getting-started)
     * [Install an Akraino REC and get DANM for free](#install-an-akraino-rec-and-get-danm-for-free)
     * [Prerequisites](#prerequisites)
     * [Building the binaries](#building-the-binaries)
     * [Building the containers](#building-the-containers)
* [Deployment](#deployment)
* [User guide](#user-guide)
  * [Usage of DANM's CNI](#usage-of-danms-cni)
    * [Configuring DANM](#configuring-danm)
    * [Network management](#network-management)
      * [Overview](#overview)
      * [Lightweight network management experience](#lightweight-network-management-experience)
      * [Production-grade network management experience](#production-grade-network-management-experience)
      * [Network management in the practical sense](#network-management-in-the-practical-sense)
    * [Generally supported DANM API features](#generally-supported-danm-api-features)
      * [Naming container interfaces](#naming-container-interfaces)
      * [Provisioning static IP routes](#provisioning-static-ip-routes)
      * [Provisioning policy-based IP routes](#provisioning-policy-based-ip-routes)
    * [Delegating to other CNI plugins](#delegating-to-other-cni-plugins)
      * [Creating the configuration for delegated CNI operations](#creating-the-configuration-for-delegated-cni-operations)
      * [Connecting Pods to specific networks](#connecting-pods-to-specific-networks)
      * [Defining default networks](#defining-default-networks)
      * [Internal workings of the metaplugin](#internal-workings-of-the-metaplugin)
    * [DANM IPAM](#danm-ipam)
      * [Using IPAM with static backends](#using-ipam-with-static-backends)
      * [IPv6 and dual-stack support](#ipv6-and-dual-stack-support)
    * [DANM IPVLAN CNI](#danm-ipvlan-cni)
    * [Device Plugin Support](#device-plugin-support)
      * [Using Intel SR-IOV CNI](#using-intel-sr-iov-cni)
  * [Usage of DANM's Webhook component](#usage-of-danms-webhook-component)
     * [Responsibilities](#responsibilities)
     * [Connecting TenantNetworks to TenantConfigs](#connecting-tenantnetworks-to-tenantconfigs)
       * [TenantConfig API](#tenantconfig-api)
       * [Selecting a physical interface profile](#selecting-a-physical-interface-profile)
       * [Overwrite NetworkID for static delegates](#overwrite-networkid-for-static-delegates)
     * [List of validation rules](#list-of-validation-rules)
        * [DanmNet](#danmnet)
        * [TenantNetwork](#tenantnetwork)
        * [ClusterNetwork](#clusternetwork)
        * [TenantConfig](#tenantconfig)
  * [Usage of DANM's Netwatcher component](#usage-of-danms-netwatcher-component)
  * [Usage of DANM's Svcwatcher component](#usage-of-danms-svcwatcher-component)
    * [Feature description](#feature-description)
    * [Svcwatcher compatible Service descriptors](#svcwatcher-compatible-service-descriptors)
    * [Demo: Multi-domain service discovery in Kubernetes](#demo-multi-domain-service-discovery-in-kubernetes)
* [Contributing](#contributing)
* [Authors](#authors)
* [License](#license)


## Introduction
__DANM__ is Nokia's solution to bring TelCo grade network management into a Kubernetes cluster! DANM has more than 4 years of history inside the company, is currently deployed into production, and it is finally available for everyone, here on GitHub.

The name stands for "Damn, Another Network Manager!", because yes, we know: the last thing the K8s world needed is another TelCo company "revolutionizing" networking in Kubernetes.
But still we hope that potential users checking out our project will involuntarily proclaim "DANM, that's some good networking stuff!" :)

Please consider for a moment that there is a whole other world out there, with special requirements, and DANM is the result of those needs!
We are certainly not saying DANM is __THE__ network solution, but we think it is a damn good one!
Want to learn more about this brave new world? Don't hesitate to contact us, we are always quite happy to share the special requirements we need to satisfy each and every day.

**In any case, DANM is more than just a plugin, it is an End-To-End solution to a whole problem domain**.
It is:
* a CNI plugin capable of provisioning IPVLAN interfaces with advanced features
* an in-built IPAM module with the capability of managing multiple, ***cluster-wide***, discontinuous L3 networks and provide dynamic, static, or no IP allocation scheme on-demand
* a CNI metaplugin capable of attaching multiple network interfaces to a container, either through its own CNI, or through delegating the job to any of the popular CNI solution e.g. SR-IOV, Calico, Flannel etc. ***in parallel***
* a Kubernetes controller capable of centrally managing both VxLAN and VLAN interfaces of all Kubernetes hosts
* another Kubernetes controller extending Kubernetes' Service-based service discovery concept to work over all network interfaces of a Pod
* a standard Kubernetes Validating and Mutating Webhook responsible for making you adhere to the schemas, and also automating network resource management for tenant users in a production-grade environment

The above functionalities are implemented by the following components:
- **danm** is the CNI plugin which can be directly integrated with kubelet. Internally it consists of the CNI metaplugin, the CNI plugin responsible for managing IPVLAN interfaces, and the in-built IPAM plugin.
Danm binary is integrated to kubelet as any other [CNI plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/).

- **fakeipam** is a little program used in natively integrating 3rd party CNI plugins into the DANM ecosystem. It is basically used to echo the result of DANM's in-built IPAM to CNIs DANM delegates operations to.
Fakeipam binary should be placed into kubelet's configured CNI plugin directory, next to danm.
Fakeipam is a temporary solution, the long-term aim is to separate DANM's IPAM component into a full-fledged, standalone IPAM solution.

- **netwatcher** is a Kubernetes Controller watching the Kubernetes API for changes in the DANM related CRD network management APIs.
This component is responsible for validating the semantics of network objects, and also for maintaining VxLAN and VLAN host interfaces of all Kubernetes nodes.
Netwatcher binary is deployed in Kubernetes as a DaemonSet, running on all nodes.

- **svcwatcher** is another Kubernetes Controller monitoring Pod, Service, Endpoint, and DanmEp API paths.
This Controller is responsible for extending Kubernetes native Service Discovery to work even for the non-primary networks of the Pod.
Svcwatcher binary is deployed in Kubernetes as a DaemonSet, running only on the Kubernetes master nodes in a clustered setup.

- **webhook** is a standard Kubernetes Validating and Mutating Webhook. It has multiple, crucial responsibilities:

 - it validates all DANM introduced CRD APIs both syntactically, and semantically both during creation, and modification
 - it automatically mutates parameters only relevant to the internal implementation of DANM into the API objects
 - it automatically assigns physical network resources to the logical networks of tenant users in a production-grade infrastructure

## Our philosophy and motivation behind DANM
It is undeniable that TelCo products- even in containerized format- ***must*** own physically separated network interfaces, but we have always felt other projects put too much emphasis on this lone fact, and entirely ignored -or were afraid to tackle- the larger issue with Kubernetes.
That is: capability to **provision** multiple network interfaces to Pods is a very limited enhancement if the cloud native feature of Kubernetes **cannot be used with those extra interfaces**.

This is the very big misconception our solution aims to rectify - we strongly believe that all network interfaces shall be natively supported by K8s, and there are no such things as "primary", or "secondary" network interfaces.
Why couldn't NetworkPolicies, Services, LoadBalancers, all of these existing and proven Kubernetes constructs work with all network interfaces?
Why couldn't network administrators freely decide which physical networks are reachable by a Pod?
In our opinion the answer is quite simple: because networks are not first-class citizens in Kubernetes.

This is the historical reason why DANM's CRD based, abstract network management APIs were born, and why is the whole ecosystem built around the concept of promoting networks to first-class Kubernetes API objects.

This approach opens-up a plethora of possibilities, even with today's Kubernetes core code!

The following chapters will guide you through the description of these features, and will show you how you can leverage them in your Kubernetes cluster.
## Note about boundaries
You will see at the end of this README that we really went above and beyond what "networks" are in vanilla Kubernetes.

But, DANM core project never did, and will break one core concept: DANM is first and foremost a run-time agnostic standard CNI system for Kubernetes, 100% adhering to the Kubernetes life-cycle management principles.

It is important to state this, because the features DANM provides open up a couple of very enticing, but also very dangerous avenues:
 - what if we would monitor the run-time and provide added high-availability feature based on events happening on that level?
 - what if we could change the networks of existing Pods?

 We strongly feel that all such scenarios incompatible with the life-cycle of a standard CNI plugin firmly fall outside the responsibility of the core DANM project.
That being said, tell us about your Kubernetes breaking ideas! We are open to accept such plugins into the wider umbrella of the existing eco-system: outside of the core project, but still loosely linked to suite as optional, external components.
Just because something doesn't fit into DANM, it does not mean it can't fit into your cloud!

## Getting started
### Install an Akraino REC and get DANM for free!
Just kidding as DANM is always free, but if you want to install a production grade, open-source Kubernetes-based bare metal CaaS infrastructure by default equipped with DANM **and** with a single click of a button nonetheless; just head over to Linux Foundation Akraino Radio Edge Cloud (REC) wiki for the [Akraino REC Architecture](https://wiki.akraino.org/display/AK/REC+Architecture+Document) and the [Akraino REC Installation Guide](https://wiki.akraino.org/display/AK/REC+Installation+Guide)
Not just for TelCo!
### Prerequisites
Otherwise, you need to create your own Kubernetes cluster, and install DANM manually. We suggest to use any of the automated Kubernetes installing solutions (kubeadm, minikube etc.) for a painless experience.
We currently test DANM with Kubernetes 1.15.X.
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
Copying the respective binary into the right folder (netwatcher into integration/docker/netwatcher, svcwatcher into integration/docker/svcwatcher, webhook into integration/docker/webhook), then executing:
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
builds the respective containers. Afterwards these containers can be directly integrated into a running Kubernetes cluster!
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
 We use Flannel for the purpose in our environments, and connect Pods to it with such simple network descriptors like what you can find in **integration/bootstrap_networks**.

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
**Disclaimer**: svcwatcher already leverages DANM CNI to create its network interface. Don't forget to change the name of the network referenced in the example manifest file to your bootstrap network!
We use Flannel for this purpose in our infrastructures.
We also assume RBAC is configured in your cluster.
## User guide
This section describes what features the DANM networking suite adds to a vanilla Kubernetes environment, and how can users utilize them.

### Usage of DANM's CNI
#### Configuring DANM
As DANM becomes more and more complex, we offer some level of control over the internal behaviour of how network provisioning is done.
Unless stated otherwise, DANM behaviour can be configured purely through its CNI configuration file.
The following configuration options are currently supported:
 - cniDir: Users can define where should DANM search for the CNI config files for static delegates. Default value is /etc/cni/net.d
 - namingScheme: if it is set to legacy, container network interface names are set exactly to the value of the respective network's Spec.Options.container_prefix parameter. Otherwise refer to [Naming container interfaces](#naming-container-interfaces) for details"
#### Network management
##### Overview
The DANM CNI is a full-fledged CNI metaplugin, capable of provisioning multiple network interfaces to a Pod, on-demand!
DANM can utilize any of the existing and already integrated CNI plugins to do so.
DANM supports two kind of network management experiences as of DANM 4.0 - **lightweight** (the only supported mode before v4.0), and **production-grade**.
Your experience depends on which CRD-based management APIs you chose to add to your cluster during installation.
If you want you can even add all available APIs at the same time to see which method better fits your need!
##### Lightweight network management experience
We advise new users, or users operating a single tenant Kubernetes cluster to start out with a streamlined, lightweight network management experience.
In this "mode" DANM only recognizes one network management API, called **DanmNet**.
Both administrators, and tenant users manage their networks through the same API. Everyone has the same level of access, and can configure all the parameters supported by DANM at their leisure.
At the same time it is impossible to create networks, which can be used across tenants (disclaimer: we use the word "tenant" as a synonym to "Kubernetes namespace" throughout the document).
##### Production-grade network management experience
In a real, production-grade cluster the lightweight management paradigm does not suffice, because usually there are different users, with different roles interacting with each other.
There are possibly multiple users using their own segment of the cloud -or should we say tenant?- at the same time; while there can be administrator(s) overseeing that everything is configured, and works as it should be.
The idea behind production-grade network management is that:
 - tenant users shall be restricted to using only the network resources allocated to them by the administrators, but should be able to freely decide what to do with these resources within the confines of their tenant
 - administrators, and only administrators shall have control over the network resources of the whole cloud

To satisfy the needs of this complex ecosystem, DANM provides different APIs for the different purposes: **TenantNetworks**, and **ClusterNetworks**!

**TenantNetworks** is a namespaced API, and can be freely created by tenant users. It basically is the same API as DanmNet, with one big difference: parameters any way related to host settings cannot be freely configured through this API. These parameters are automatically filled by DANM instead!
Wonder how? Refer to chapter [Connecting TenantNetworks to TenantConfigs](#connecting-tenantnetworks-to-tenantconfigs) for more information.

 **ClusterNetworks** on the other hand is a cluster-wide API, and as such, can be -or should be- only provisioned by administrator level users. Administrators can freely set all available configuration options, even the physical parameters.
 The other nice thing in ClusterNetworks is that all Pods, in any namespace can connect to them - unless the network administrator forbade it via the newly introduced **AllowedTenants** configuration list.

Interested user can find reference manifests showcasing the features of the new APIs under [DANM V4 example manifests](https://github.com/nokia/danm/tree/master/example/4_0_examples).
 ##### Network management in the practical sense
Regardless which paradigm thrives in your cluster, network objects are managed the exact same way - you just might not be allowed to execute a specific provisioning operation in case you are trying to overstep your boundaries! Don't worry, as DANM will always explicitly and instantly tell you if this is the case.
Unless explicitly stated in the description of a specific feature, all API features are generally supported, and supported the same way regardless through which network management API type you use them.

Network management always starts with the creation of Kubernetes API objects, logically representing the characteristics of a network Pods can connect to.
Users first need to create the manifest files of these objects according to the schema described in the [DanmNet schema](https://github.com/nokia/danm/tree/master/schema/DanmNet.yaml) ,  [TenantNetwork schema](https://github.com/nokia/danm/tree/master/schema/TenantNetwork.yaml) , or [ClusterNetwork schema](https://github.com/nokia/danm/tree/master/schema/ClusterNetwork.yaml)  template files.
A network object can be created just like any other Kubernetes object, for example by issuing:
 ```
kubectl create -f test-net1.yaml
```
Users can also interact with the existing network management objects just as they would with other core API objects:
 ```
/ # kubectl describe danmnet test-net1
Name:         test-net1
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  kubernetes.nokia.com/v1
Kind:         DanmNet
Metadata:
  Cluster Name:
  Creation Timestamp:  2018-05-24T16:53:27Z
  Generation:          0
  Resource Version:    3146
  Self Link:           /apis/kubernetes.nokia.com/v1/namespaces/default/danmnets/test-net1
  UID:                 fb1fdfb5-5f72-11e8-a8d0-fa163e98af00
Spec:
  Network ID:    test-net1
  Network Type:  ipvlan
  Options:
    Allocation _ Pool:
      Start: 192.168.1.10
      End: 192.168.1.100
    Container _ Prefix:  eth0
    Host _ Device:       ens4
    Rt _ Tables:         201
  Validation:            True
Events:                  <none>
```

__WARNING: DANM stores pretty important information in these API objects. Under no circumstances shall a network be deleted, if there are any running Pods still referencing it!__
__Such action will undoubtedly lead to ruin and DANMation!__
#### Generally supported DANM API features
##### Naming container interfaces
Generally speaking, you need to care about how the network interfaces of your Pods are named inside their respective network namespaces.
The hard reality to keep in mind is that you shall always have an interface literally called "eth0" created within all your Kubernetes Pods, because Kubelet will always search for the existence of such an interface at the end of Pod instantiation.
If such an interface does not exist after CNI is invoked (also having an IPv4 address), the state of the Pod will be considered "faulty", and it will be re-created in a loop.
To be able to comply with this Kubernetes limitation, DANM always names the first container interface "eth0", regardless of your intention.

Sorry, but they made us do it :)

**Note**: some CNI plugins try to be smart about this limitation on their own, and decided not to adhere to the CNI standard! An example of this behaviour can be found in Flannel.
It is the user's responsibility to put the network connection of such boneheaded backends to the first place in the Pod's annotation!

Besides making sure the first interface is always named correctly, DANM also supports both explicit, and implicit interface naming schemes for all NetworkTypes to help you flexibly name the other -and CNI standard- interfaces!
An interface connected to a network containing the container_prefix attribute is always named accordingly. You can use this API to explicitly set descriptive, unique names to NICs connecting to this network.
In case container_prefix is not set in an interface's network descriptor, DANM automatically uses the "eth" as the prefix when naming the interface.
Regardless which prefix is used, the interface name is also suffixed with an integer number corresponding to the sequence number of the network connection (e.g. the first interface defined in the annotation is called "eth0", second interface "eth1" etc.)
DANM even supports the mixing of the networking schemes within the same Pod, and it supports the whole naming scheme for all network backends.
This enables network administrators to even connect Pods to the same network more than once!

##### Provisioning static IP routes
We recognize that not all networking involves an overlay technology, so provisioning IP routes directly into the Pod's network namespace needs to be generally supported.
Network administrators can define routing rules for both IPv4, and IPv6 destination subnets under the "routes", and "routes6" attributes respectively.
These attributes take a map of string-string key (destination subnet)-value(gateway address) pairs.
The configured routes will be added to the default routing table of all Pods connecting to this network.
##### Provisioning policy-based IP routes
Configuring generic routes on the network level is a nice feature, but in more complex network configurations (e.g. Pod connects to multiple networks) it is desirable to support Pod-level route provisioning.
The routing table to hold the Pods' policy-based IP routes can be configured via the "rt_tables" API attribute.
Whenever a Pod asks for policy-based routes via the "proutes", and/or "proutes6" network connection attributes, the related routes will be added to the configured table.
DANM also provisions the necessary rule pointing to the configured routing table.

#### Delegating to other CNI plugins
Pay special attention to the network attribute called "NetworkType". This parameter controls which CNI plugin is invoked by the DANM metaplugin during the execution of a CNI operation to setup, or delete exactly one network interface of a Pod.

In case this parameter is set to "ipvlan", or is missing; then DANM's in-built IPVLAN CNI plugin creates the network (see next chapter for details).
In case this attribute is provided and set to another value than "ipvlan", then network management is delegated to the CNI plugin with the same name.
The binary will be searched in the configured CNI binary directory.
Example: when a Pod is created and requests a connection to a network with "NetworkType" set to "flannel", then DANM will delegate the creation of this network interface to the <CONFIGURED_CNI_PATH_IN_KUBELET>/flannel binary.
##### Creating the configuration for delegated CNI operations
We strongly believe that network management in general should be driven by generic APIs -almost- completely adhering to the same schema. Therefore, DANM is capable to "translate" the generic options coming from network objects into the specific "language" the delegated CNI plugin understands.
This way users can dynamically configure various networking solutions via the same, abstract interface without caring about how a specific option is called exactly in the terminology of the delegated solution.

A generic framework supporting this method is built into DANM's code, but still this level of integration requires case-by-case implementation.
As a result, DANM currently supports two integration levels:

 - **Dynamic integration level:** CNI-specific network attributes (such as IP ranges, parent host devices etc.) can be controlled on a per network level, exclusively taken directly from the CRD object
 - **Static integration level:** CNI-specific network attributes are by default configured via static CNI configuration files (Note: this is the default CNI configuration method); but certain parameters are influenced by the DANM API configuration values.

Always refer to the schema descriptors for more details on which parameters are universally supported!

Our aim is to integrate all the popular CNIs into the DANM eco-system over time, but currently the following CNI's achieved dynamic integration level:

 - DANM's own, in-built IPVLAN CNI plugin
	 - Set the "NetworkType" parameter to value "ipvlan" to use this backend
- Intel's [SR-IOV CNI plugin](https://github.com/intel/sriov-cni )
	- Set the "NetworkType" parameter to value "sriov" to use this backend
- Generic MACVLAN CNI from the CNI plugins example repository [MACVLAN CNI plugin](https://github.com/containernetworking/plugins/blob/master/plugins/main/macvlan/macvlan.go )
	- Set the "NetworkType" parameter to value "macvlan" to use this backend

No separate configuration needs to be provided to DANM when it connects Pods to networks, if the network is backed by a CNI plugin with dynamic integration level.
Everything happens automatically purely based on the network manifest!

When network management is delegated to CNI plugins with static integration level; DANM first reads their configuration from the configured CNI config directory.
The directory can be configured via setting the "CNI_CONF_DIR" environment variable in DANM CNI's context (be it in the host namespace, or inside a Kubelet container). Default value is "/etc/cni/net.d".
In case there are multiple configuration files present for the same backend, users can control which one is used in a specific network provisioning operation via the NetworkID parameter.

So, all in all: a Pod connecting to a network with "NetworkType" set to "bridge", and "NetworkID" set to "example_network" gets an interface provisioned by the <CONFIGURED_CNI_PATH_IN_KUBELET>/bridge binary based on the <CNI_CONF_DIR>/example_network.conf file!
In addition to simply delegating the interface creation operation, the universally supported features of the DANM management APIs -such as static and dynamic IP route provisioning, flexible interface naming, or centralized IPAM- are also configured either before, or after the delegation took place.
##### Connecting Pods to specific networks
Pods can request network connections to networks by defining one or more network connections in the annotation of their (template) spec field, according to the schema described in the **schema/network_attach.yaml** file.

For each connection defined in such a manner DANM provisions exactly one interface into the Pod's network namespace, according to the way described in previous chapters (configuration taken from the referenced API object).
In case you have added more than one network management APIs to your cluster, it is possible to connect the same Pod to different networks of different APIs. But please note, that physical network interfaces are 1:1 mapped to logical networks.

In addition to simply invoking other CNI libraries to set-up network connections, Pod's can even influence the way their interfaces are created to a certain extent.
For example Pods can ask DANM to provision L3 IP addresses to their network interfaces dynamically, statically, or not at all!
Or, as described earlier; creation of policy-based L3 IP routes into their network namespace is also universally supported by the solution.
##### Defining default networks
If the Pod annotation is empty (no explicit connections are defined), DANM tries to fall back to a configured default network.
In the lightweight network management paradigm default networks can be only configured on a per namespace level, by creating one DanmNet object with ObjectMeta.Name field set to "default" in the Pod's namespace.
In a production grade cluster, default networks can be configured both on the namespace, and on the cluster level. If both are configured for a Pod -both a TenantNetwork named default in the Pod's namespace, and a ClusterNetwork named default exist in the cluster-; the namespace level default takes precedence.

There are no restrictions as to what DANM supported attributes can be configured for a default network. However, in this case users cannot specify any further fine-grained properties for the Pod (i.e. static IP address, policy-based IP routes).
This feature is beneficial for cluster operators who would like to use unmodified upstream manifest files (i.e. community maintained Helm charts or Pods created by K8s operators), or would like to use DANM in the "vanilla K8s" way.

##### Internal workings of the metaplugin
Regardless which CNI plugins are involved in managing the networks of a Pod, and how they are configured; DANM invokes all of them at the same time, in parallel threads.

DANM waits for the CNI result of all executors before converting, and merging them together into one summarized result object. The aggregated result is then sent back to kubelet.

If any executor reported an error, or hasn't finished its job even after 10 seconds; the result of the whole operation will be an error.
DANM reports all errors towards kubelet in case multiple CNI plugins failed to do their job.
#### DANM IPAM
DANM includes a fully generic and very flexible IPAM module in-built into the solution. The usage of this module is seamlessly integrated together with all the natively supported CNI plugins (DANM's IPVLAN, Intel's SR-IOV, and the CNI project's reference MACVLAN plugins); as well as with any other CNI backend fully adhering to the v0.3.1 CNI standard!

The main feature of DANM's IPAM is that it's fully integrated into DANM's network management APIs through the attributes called "cidr", "allocation_pool", and "net6". Therefore users of the module can easily configure all aspects of network management by manipulating solely dynamic Kubernetes API objects!

This native integration also enables a very tempting possibility. **As IP allocations belonging to a network are dynamically tracked *within the same API object***, it becomes possible to define:
* discontinuous subnets 1:1 mapped to a logical network
* **cluster-wide usable subnets** (instead of node restricted sub CIDRs)

Network administrators can simply put the CIDR, and the allocation pool into the network object. Whenever a Pod is instantiated or deleted **on any host within the cluster**, DANM simply updates the allocation record belonging to the network through the Kubernetes API before provisioning the chosen IP to the Pod's interface.

The flexible IPAM module also allows Pods to define the IP allocation scheme best suited for them. Pods can ask dynamically allocated IPs from the defined allocation pool, or can ask for one, specific, static address.
The application can even ask DANM to forego the allocation of any IPs to their interface in case a L2 network interface is required.
##### Using IPAM with static backends
While using the DANM IPAM with dynamic backends is mandatory, netadmins can freely choose if they want their static CNI backends to be also integrated to DANM's IPAM; or they would prefer these interfaces to be statically configured by another IPAM module.
By default the "ipam" section of a static delegate is always configured from the CNI configuration file identified by the network's NetworkID parameter.
However, users can overwrite this inflexible -and most of the time host-local- option by defining "cidr", and/or "net6" in their network manifest just as they would with a dynamic backend.
When a Pod connects to a network with static NetworkType but containing allocation subnets, and explicitly asks for an "ip", and/or "ip6" address from DANM in its annotation; DANM overwrites the "ipam" section coming from the static config with its own, dynamically allocated address.
If a Pod does not ask DANM to allocate an IP, or the network does not define the necessary parameters; the delegation automatically falls back to the "ipam" defined in the static config file.
**Note**: DANM can only integrate static backends to its flexible IPAM if the CNI itself is fully compliant to the standard, i.e. uses the plugin defined in the "ipam" section of its configuration. It is the administrator's responsibility to configure the DANM management APIs according to the capabilities of every CNI!
##### IPv6 and dual-stack support
DANM's IPAM module, and its integration to dynamic backends -IPVLAN, MACVLAN, and SR-IOV CNIs- support both IPv6, and dual-stack (one IPv4, and one IPv6 address provisioned to the same interface) addresses!
To configure an IPv6 CIDR for a network, network administrators shall configure the "net6" attribute. Additionally, IP routes for IPv6 subnets can be configured via "routes6".
If both "cidr", and "net6" are configured for the same network, Pods connecting to that network can ask either one IPv4 or IPv6 address - or even both at the same time!

That being said, network administrators using IPv6, or dual-stack features need to be aware of the current restrictions of the solution:
* dynamic IPs are randomly allocated from the defined IPv6 subnet according to the following algorithm:
  * the IP is prefixed with the net6 parameter of the network
  * MAC address is randomly generated for the EUI64
* the smallest supported IPv6 subnet is /64
* allocation pools cannot be defined for IPv6 subnets

This feature is generally supported the same way even for static CNI backends! However guaranteeing that every specific backend is compabile and comfortable with both IPv6, and dual IPs allocated by an IPAM cannot be guaranteed by DANM.
Therefore, it is the administrator's responsibility to configure the DANM management APIs according to the capabilities of every CNI!
#### DANM IPVLAN CNI
DANM's IPVLAN CNI uses the Linux kernel's IPVLAN module to provision high-speed, low-latency network interfaces for applications which need better performance than a bridge (or any other overlay technology) can provide.

*Keep in mind that the IPVLAN module is a fairly recent addition to the Linux kernel, so the feature cannot be used on systems whose kernel is older than 4.4!
4.14+ would be even better (lotta bug fixes)*

The CNI provisions IPVLAN interfaces in L2 mode, and supports the following extra features:
* attaching IPVLAN sub-interfaces to any host interface
* attaching IPVLAN sub-interfaces to dynamically created VLAN or VxLAN host interfaces
* renaming the created interfaces according to the "container_prefix" attribute defined in the network object
* allocating IP addresses by using DANM's flexible, in-built IPAM module
* provisioning generic IP routes into a configured routing table inside the Pod's network namespace
* Pod-level controlled provisioning of policy-based IP routes into Pod's network namespace
#### Device Plugin support
DANM provides general support to CNIs which interwork with Kubernetes' Device Plugin mechanism such as SR-IOV CNI.
When a properly configured Network Device Plugin runs, the allocatable resource list for the node should be updated with resource discovered by the plugin.
##### Using Intel SR-IOV CNI
SR-IOV Network Device Plugin allows to create a list of *netdevice* type resource definitions with *sriovMode*, where each resource definition can have one or more assigned *rootDevice* (Physical Function). The plugin looks for Virtual Functions (VF) for each configured Physical Function (PF) and adds all discovered VFs to the allocatable resource's list of the given Kubernetes Node. The Device Plugin resource name will be the device pool name on the Node. These device pools can be referred in Pod definition's resource request part on the usual way.

In the following example, the "nokia.k8s.io/sriov_ens1f0" device pool name consists of the "nokia.k8s.io" prefix and "sriov_ens1f0" resourceName.
```
kubectl get nodes 172.30.101.104 -o json | jq '.status.allocatable'
{
  "cpu": "48",
  "ephemeral-storage": "48308001098",
  "hugepages-1Gi": "16Gi",
  "memory": "246963760Ki",
  "nokia.k8s.io/default": "0",
  "nokia.k8s.io/sriov_ens1f0": "8",
  "nokia.k8s.io/sriov_ens1f1": "8",
  "pods": "110"
}
```
All network management APIs contain an optional **device_pool** field where a specific device pool can be assigned to the given network.
**Note: device_pool and host_device parameters are mutually exclusive!**
Before DANM invokes a CNI which expects a given resource to be attached to the Pod, it gathers all the Kubelet assigned device IDs belonging to device pool defined in the Pod's network, and passes one ID from the list to the CNI.
**Note: Pods connecting to networks depending on a device_pool must declare their respective resource requests through their Pod.Spec.Resources API!**

The following example network definition shows how to configure device_pool parameter for sriov network type.
```
apiVersion: danm.k8s.io/v1
kind: DanmNet
metadata:
  name: sriov-a
  namespace: example-sriov
spec:
  NetworkID: sriov-a
  NetworkType: sriov
  Options:
    device_pool: "nokia.k8s.io/sriov_ens1f0"
```
The following Pod definition shows how to combine K8s Device resource requests and multiple network connections using the assigned resources:
```
apiVersion: v1
kind: Pod
metadata:
  name: sriov-pod
  namespace: example-sriov
  labels:
    env: test
  annotations:
    danm.k8s.io/interfaces: |
      [
        {"network":"management", "ip":"dynamic"},
        {"network":"sriov-a", "ip":"none"},
        {"network":"sriov-b", "ip":"none"}
      ]
spec:
  containers:
 - name: sriov-pod
    image: busybox:latest
    args:
      - sleep
      - "1000"
    resources:
      requests:
        nokia.k8s.io/sriov_ens1f0: '1'
        nokia.k8s.io/sriov_ens1f1: '1'
      limits:
        nokia.k8s.io/sriov_ens1f0: '1'
        nokia.k8s.io/sriov_ens1f1: '1'
  nodeSelector:
    sriov: enabled
```
### Usage of DANM's Webhook component
#### Responsibilities
The Webhook component introduced in DANM V4 is responsible for three things:
 - it initializes essential, but not human configurable API attributes (i.e. allocation tracking bitmasks) at the time of object creation
 - it matches, and connects TenantNetworks to administrator configured physical profiles allowed for tenant users
 - it validates the syntactic and semantic integrity of all API objects before any CREATE, or PUT REST operation are allowed to be persisted in the K8s API server's data store
#### Connecting TenantNetworks to TenantConfigs
##### TenantConfig API
TenantNetworks cannot freely define the following attributes:
 - host_devices
 - device_pool
 - vlan
 - vxlan
 - NetworkID

Reason is that all these attributes are related to physical resources, which might not be allowed to be used by the specific tenants: VLANs might not be configured in the switches, specific NICs are reserved for infrastructure use, static CNI configuration files might not exist on the container host's disk etc.
Instead, these parameters are either entirely, or partially managed by DANM in TenantNetwork provisioning time.

DANM does this by introducing a third new API with v4.0 called **TenantConfig**. TenantConfig is a mandatory API when DANM is used in the production grade mode.
TenantConfig is a cluster-wide API, containing two major parameters: physical interface profiles usable by TenantNetworks, and NetworkType:NetworkID mappings.

Refer to [TenantConfig schema](https://github.com/nokia/danm/tree/master/schema/TenantConfig.yaml) for more information on TenantConfigs.
##### Selecting a physical interface profile
There are multiple ways of how DANM can select the appropriate interface profile for a tenant user's network.
Note: physical interface profiles are only relevant for dynamic backends.

For backends dependent on the host_device option (such as IPVLAN, and MACVLAN):

 - if the TenantNetwork contains host_device attribute, DANM selects the entry from the TenantConfig with the matching name
 - if host_device is not provided by user, DANM randomly selects an interface profile from the TenantConfig

For backends dependent on the device_pool option (such as SR-IOV), the user needs to explicitly state which device_pool it wants to use.
The reasoning behind not supporting random profile selection for K8s Devices based backends is that the Pod using such Devices anyway need to explicitly request resources from a specific pool in its own Pod manifest. Randomly matching its network with a possibly different pool could result in run-time failures.

If there are no suitable physical interface profiles configured by the cluster's network administrator, or the TenantNetwork tried to select a physical device which is not allowed; webhook denies the creation of the TenantNetwork.

If a suitable profile could be selected, DANM:

 - mutates the physical interface profile's name into either the TenantNetwork's host_device, or device_pool attribute (DANM automatically figures out which one based on the name of the profile, and the NetworkType parameter)
 - if the interface profile is a virtual profile, DANM automatically reserves the next previously unused VNI from the configured VNI range
 - then mutates the reserved VNI into the TenantNetwork's respective attribute (vlan, or vxlan)

To avoid the leaking of VNIs in the cluster, DANM also takes care of freeing the reserved VNI of a TenantNetwork when it is deleted.
##### Overwrite NetworkID for static delegates
Delegation to backends with static integration level (e.g. Calico, Flannel etc.) is configured via static CNI config files read from the container host's disk.
These files are selected based on the NetworkType parameter of the TenantNetwork.
Network administrators can configure NetworkType: NetworkID mappings into the TenantConfig. When a TenantNetwork is created with a NetworkType having a configured mapping, DANM automatically overwrites it's NetworkID with the provided value.
Thus it becomes guaranteed that the tenant user's network will use the right CNI configuration file during Pod creation!
#### List of validation rules
##### DanmNet
Every CREATE, and ~~PUT~~ (see [https://github.com/nokia/danm/issues/144](https://github.com/nokia/danm/issues/144)) DanmNet operation is subject to the following validation rules:

 1. spec.Options.Cidr must be supplied in a valid IPv4 CIDR notation
 2. all gateway addresses belonging to an entry of spec.Options.Routes  shall be in the defined IPv4 CIDR
 3. spec.Options.Net6 must be supplied in a valid IPv6 CIDR notation
 4. all gateway addresses belonging to an entry of spec.Options.Routes6  shall be in the defined IPv6 CIDR
 5. spec.Options.Alloc shall not be manually defined
 6. spec.Options.Allocation_pool cannot be defined without defining spec.Options.Cidr
 7. spec.Options.Allocation_pool.Start shall be in the provided IPv4 CIDR
 8. spec.Options.Allocation_pool.End shall be in the provided IPv4 CIDR
 9. spec.Options.Allocation_pool.End shall be smaller than spec.Options.Allocation_pool.Start
 10. spec.Options.Vlan and spec.Options.Vxlan cannot be provided together
 11. spec.NetworkID cannot be longer than 11 characters for dynamic backends
 12. spec.AllowedTenants is not a valid parameter for this API type
 13. spec.Options.Device_pool must be, and spec.Options.Host_device mustn't be provided for K8s Devices based networks (such as SR-IOV)
 14. Any of spec.Options.Device, spec.Options.Vlan, or spec.Options.Vxlan attributes cannot be changed if there are any Pods currently connected to the network

 Every DELETE DanmNet operation is subject to the following validation rules:
 15. the network cannot be deleted if there are any Pods currently connected to the network

Not complying with any of these rules results in the denial of the provisioning operation.
##### TenantNetwork
Every CREATE, and ~~PUT~~ (see [https://github.com/nokia/danm/issues/144](https://github.com/nokia/danm/issues/144)) TenantNetwork operation is subject to the DanmNet validation rules no. 1-9, 11, 12.
In addition TenantNetwork provisioning has the following extra rules:

 1. spec.Options.Vlan cannot be provided
 2. spec.Options.Vxlan cannot be provided
 3. spec.Options.Vlan cannot be modified
 4. spec.Options.Vxlan cannot be modified
 5. spec.Options.Host_device cannot be modified
 6. spec.Options.Device_pool cannot be modified

Every DELETE TenantNetwork operation is subject to the DanmNet validation rule no.15.

Not complying with any of these rules results in the denial of the provisioning operation.
##### ClusterNetwork
Every CREATE, and ~~PUT~~ (see [https://github.com/nokia/danm/issues/144](https://github.com/nokia/danm/issues/144)) ClusterNetwork operation is subject to the DanmNet validation rules no. 1-11, 13-14.

Every DELETE ClusterNetwork operation is subject to the DanmNet validation rule no.15.

Not complying with any of these rules results in the denial of the provisioning operation.
##### TenantConfig
Every CREATE, and PUT TenantConfig operation is subject to the following validation rules:

 1. Either HostDevices, or NetworkIDs must not be empty
 2. VniType and VniRange must be defined together for every HostDevices entry
 3. Both key, and value must not be empty in every NetworkType: NetworkID mapping entry
 4. A NetworkID cannot be longer than 11 characters in a NetworkType: NetworkID mapping belonging to a dynamic NetworkType

### Usage of DANM's Netwatcher component
Netwatcher is a mandatory component of the DANM networking suite.
It is implemented using Kubernetes' Informer paradigm, and is deployed as a DaemonSet.
It shall be running on all hosts where DANM CNI is the configured CNI plugin.

The netwatcher component is responsible for dynamically managing (i.e. creation and deletion) VxLAN and VLAN interfaces on all the hosts based on the dynamic network management APIs.

Whenever a network is created, modified, or deleted -any network, belonging to any of the supported API types- within the Kubernetes cluster, netwatcher will be triggered.
If the network in question contained either the "vxlan", or the "vlan" attributes; then netwatcher immediately creates, or deletes the VLAN or VxLAN host interface with the matching VID.
If the Spec.Options.host_device, .vlan, or .vxlan attributes are modified netwatcher first deletes the old, and then creates the new host interface.

This feature is the most beneficial when used together with a dynamic network provisioning backend supporting connecting Pod interfaces to virtual host devices (IPVLAN, MACVLAN, SR-IOV for VLANs). Whenever a Pod is connected to such a network containing a virtual network identifier, the CNI component automatically connects the created interface to the VxLAN or VLAN host interface created by the netwatcher; instead of directly connecting it to the configured host device.
### Usage of DANM's Svcwatcher component
#### Feature description
Svcwatcher component showcases the whole reason why DANM exists, and is designed the way it is. It is the first higher-level feature accomplishing our true goal described in the introduction section, that is, extending basic Kubernetes constructs to seamlessly work with multiple network interfaces.

The first such construct is the Kubernetes Service!
Let's see how it works.

Svcwatcher basically works the same way as the default Service controller inside Kubernetes. It continuously monitors both the Service and the Pod APIs, and provisions Endpoints whenever the cluster administrator creates, updates, or deletes relevant API objects (e.g. creates a new Service, updates a Pod label etc.).

DANM svcwatcher does the same, and more! The default Service controller assumes the Pod has one interface, so whenever a logical Service Endpoint is created it will be always done with the IP of the Pod's first (the infamous "eth0" in Kubernetes), and supposedly only network interface.

DANM svcwatcher on the other hand makes this behaviour configurable! DANM enhances the same Service API so an object will always explicitly select one logical network, rather than implicitly choosing the one with the hard-coded name of "eth0".
Then, svcwatcher provisions a Service Endpoint with the address of the selected Pod's chosen network interface.

This enhancement basically upgrades the in-built Kubernetes Service Discovery concept to work over multiple network interfaces, making Service Discovery only return truly relevant Endpoints in every scenario!

The services of the svcwatcher component work with all supported network management APIs!
#### Svcwatcher compatible Service descriptors
Based on the feature description experienced Kubernetes users are probably already thinking "but wait, there is no "network selector" field in the Kubernetes Service core API".
That is indeed true right now, but consider the core concept behind the creation of DANM: "what use-cases would become possible if Networks would be part of the core Kubernetes API"?
So, we went ahead and simulated exactly this scenario, while making sure our solution also works with a vanilla Kubernetes today; just as we did with all our other API enhancements.

This is possible by leveraging the so-called "headless and selectorless Services" concept in Kubernetes. Headless plus selectorless Services do not contain Pod selector field, which tells the Kubernetes native Service controller that Endpoint administration is handled by a 3rd party service.

DANM svcwatcher is triggered when such a service is created, if it contains the DANM "core API" attributes in their annotation.
These extra attributes are the following:
"danm.k8s.io/selector": this selector serves the exact same purpose as the default Pod selector field (which is missing from a selectorless Service by definition). Endpoints are created for Pods which match all labels provided in this list
"danm.k8s.io/network": this is the "special sauce" of DANM. When svcwatcher creates an Endpoint, it's IP will be taken from the selected Pod's physical interface connected to the DanmNet with the matching name
"danm.k8s.io/tenantNetwork": serves the exact same purpose as the network selector, but it selects interfaces connected to TenantNetworks, rather than DanmNets
"danm.k8s.io/clusterNetwork": serves the exact same purpose as the network selector, but it selects interfaces connected to ClusterNetworks, rather than DanmNets

This means that DANM controlled Services behave exactly as in Kubernetes: a selected Pod's availability is advertised through one of its network interfaces.
The big difference is that operators can now decide through which interface(s) they want the Pod to be discoverable! (Of course nothing forbids the creation of multiple Services selecting different interfaces of the same Pod, in case a Pod should be discoverable by different kind of communication partners).

The schema of the enhanced, DANM-compatible Service object is described in detail in **schema/DanmService**.yaml file.
#### Demo: Multi-domain service discovery in Kubernetes
Why is this feature useful, the reader might ask?
The answer depends on the use-case your application serves. If you share one, cloud-wide network between all application and infrastructure components, and everyone communicates with everyone through this -most probably overlay- network, then you are probably not excited by DANM's svcwatcher.

However, if you believe in physically separated interfaces (or certain government organizations made you believe in it), non-default networks, multi-domain gateway components; then this is the feature you probably already built-in to your application's Helm chart in the form of an extra Consul, or Etcd component.

This duplication of platform responsibility ends today! :)

Allow us to demonstrate the usage of this feature via an every-day common TelCo inspired example located in the project's example/svcwatcher_demo directory.
The example contains three Pods running in the same cluster:
 - A LoadBalancer Pod, whose job is to accept connections over any exotic but widely used non-L7 protocols (e.g. DIAMETER, LDAP, SIP, SIGTRAN etc.), and distribute the workload to backend services
 - An ExternalClient Pod, supplying the LoadBalancer with traffic through an external network
 - An InternalProcessor Pod, receiving requests to be served from the LoadBalancer Pod

Our cluster contains three physical networks: external, internal, management.
LoadBalancer connects to all three, because it needs to be able to establish connections to entities both supplying, and serving traffic. LoadBalancer also wishes to be scaled via Prometheus, hence it connects to the cluster's management network to expose its own "packet_served_per_second" custom metric.

ExternalClient only connects to the LoadBalancer Pod, because it simply wants to send traffic to the application (VNF), and deal with the result of transactions. It doesn't care, or know anything about the internal architecture of the application (VNF).
Because ExternalClient is not part of the same application (namespace) as LoadBalancer and InternalProcessor, it can't have access to their internal network.
It doesn't require scaling, being a lightweight, non-critical component, therefore it also does not connect to the cluster's management network.

InternalProcessor only connects to the LoadBalancer Pod, but being a small, dynamically changing component, we don't want to expose it to external clients.
InternalProcessor wants to have access to the many network-based features of Kubernetes, so it also connects to the management network, similarly to LoadBalancer.

**So, how can ExternalClient(S) discover LoadBalancer(S), how can LoadBalancer(S) discover InternalProcessor(S), and how can we avoid making LoadBalancer(S) and InternalProcessor(S) discoverable through their management interface?**

With DANM, the answer is as simple as instantiating the demonstration Kubernetes manifest files in the following order:
Namespaces -> DanmNets -> Deployments -> Services
"vnf-internal-processor" will make the InternalProcessors discoverable through their application-internal network interface. LoadBalancers can use this Service to discover working backends serving transactions.
"vnf-internal-lb" will make the LoadBalancers discoverable through their application-internal  network interface. InternalProcessors can use this Service to discover application egress points/gateway components.
Lastly, "vnf-external-svc" makes the same LoadBalancer instances discoverable but this time through their external network interfaces. External clients connecting to the same network can use this Service to find the ingress/gateway interfaces of the whole application (VNF)!

As a closing note: remember to delete the now unnecessary Service Discovery tool's Deployment manifest from your Helm chart :)

## Contributing

Please read [CONTRIBUTING.md](https://github.com/nokia/danm/blob/master/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

## Authors

* **Robert Springer** (@rospring) - Initial work (V1 Python), IPAM, Netwatcher, Svcwatcher [Nokia](https://github.com/nokia)
* **Levente Kale** (@Levovar) - Initial work (V2 Golang), Documentation, Integration, SCM, UTs, Metaplugin, V4 work [Nokia](https://github.com/nokia)

Special thanks to the original author who started the whole project in 2015 by putting a proprietary network management plugin between Kubelet and Docker; and also for coining the DANM acronym:
**Peter Braun** (@peter-braun)

## License

This project is licensed under the 3-Clause BSD License - see the [LICENSE](LICENSE)
