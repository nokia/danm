# DANM
[![Build Status](https://travis-ci.org/nokia/danm.svg?branch=master)](https://travis-ci.org/Nokia/danm)
<img src="https://github.com/nokia/danm/raw/master/logo.png" width="100">

## Don't like reading? Then start by watching our demo video!
[Demo](example/danm_demo.mp4)

## Table of Contents
* [Table of Contents](#table-of-contents)
* [Introduction](#introduction)
* [Our philosophy and motivation behind DANM](#our-philosophy-and-motivation-behind-danm)
* [How is DANM different than Multus?](#how-is-danm-different-than-multus)
* [Getting started](#getting-started)
     * [Prerequisites](#prerequisites)
     * [Building the binaries](#building-the-binaries)
     * [Building the containers](#building-the-containers)
* [Deployment](#deployment)
* [User guide](#user-guide)
  * [Usage of DANM's CNI](#usage-of-danms-cni)
    * [Network management](#network-management)
    * [Generally supported DANM API features](#generally-supported-danm-api-features)
      * [Naming container interfaces](#naming-container-interfaces)
      * [Provisioning static IP routes](#provisioning-static-ip-routes)
      * [Provisioning policy-based IP routes](#provisioning-policy-based-ip-routes)
    * [Delegating to other CNI plugins](#delegating-to-other-cni-plugins)
      * [Creating the configuration for delegated CNI operations](#creating-the-configuration-for-delegated-cni-operations)
      * [Connecting Pods to DanmNets](#connecting-pods-to-danmnets)
      * [Internal workings of the metaplugin](#internal-workings-of-the-metaplugin)
    * [DANM IPAM](#danm-ipam)
    * [DANM IPVLAN CNI](#danm-ipvlan-cni)
  * [Usage of DANM's Netwatcher component](#usage-of-danms-netwatcher-component)
  * [Usage of DANM's Svcwatcher component](#usage-of-danms-svcwatcher-component)
    * [Feature description](#feature-description)
    * [Svcwatcher compatible Service descriptors](#svcwatcher-compatible-service-descriptors)
    * [Demo: Multi-domain service discovery in Kubernetes](#demo-multi-domain-service-discovery-in-kubernetes)
* [Contributing](#contributing)
* [Authors](#authors)
* [License](#license)


## Introduction
__DANM__ is Nokia's solution to bring TelCo grade network management into a Kubernetes cluster! DANM has more than 3 years of history inside the company, is currently deployed into production, and it is finally available for everyone, here on GitHub.

The name stands for "Damn, Another Network Manager!", because yes, we know: the last thing the K8s world needed is another TelCo company "revolutionizing" networking in Kubernetes.
But still we hope that potential users checking out our project will involuntarily proclaim "DANM, that's some good networking stuff!" :)

Please consider for a moment that there is a whole other world out there, with special requirements, and DANM is the result of those needs!
We are certainly not saying DANM is __THE__ network solution, but we think it is a damn good one!
Want to learn more about this brave new world? Don't hesitate to contact us, we are always quite happy to share the special requirements we need to satisfy each and every day.

**In any case, DANM is more than just a plugin, it is an End-To-End solution to a whole problem domain**.
It is:
* a CNI plugin capable of provisioning IPVLAN interfaces with advanced features
* an in-built IPAM module with the capability of managing multiple, ***cluster-wide***, discontinuous L3 networks and provide dynamic, static, or no IP allocation scheme on-demand
* a CNI metaplugin capable of attaching multiple network interfaces to a container, either through its own CNI, or through delegating the job to any of the popular CNI solution like SR-IOV, or Flannel ***in parallel***
* a Kubernetes controller capable of centrally managing both VxLAN and VLAN interfaces of all Kubernetes hosts
* another Kubernetes controller extending Kubernetes' Service-based service discovery concept to work over all network interfaces of a Pod

at the same time!
## Our philosophy and motivation behind DANM
It is undeniable that TelCo products- even in containerized format- ***must*** own physically separated network interfaces, but we have always felt other projects put too much emphasis on this lone fact, and entirely ignored -or were afraid to tackle- the larger issue with Kubernetes.
That is: capability to **provision** multiple network interfaces to Pods is a very limited enhancement if the cloud native feature of Kubernetes **cannot be used with those extra interfaces**.

This is the very big misconception our solution aims to rectify - we strongly believe that all network interfaces shall be natively supported by K8s, and there are no such things as "primary", or "secondary" network interfaces.
Why couldn't NetworkPolicies, Services, LoadBalancers, all of these existing and proven Kubernetes constructs work with all network interfaces?
Why couldn't network administrators freely decide which physical networks are reachable by a Pod?
In our opinion the answer is quite simple: because networks are not first-class citizens in Kubernetes.

This is the historical reason why DanmNets were born, and why was the whole ecosystem built around the concept of promoting networks to first-class Kubernetes API objects.

This approach opens-up a plethora of possibilities, even with today's Kubernetes core code!

The following chapters will guide you through the description of these features, and will show you how you can leverage them in your Kubernetes cluster.
## How is DANM different than Multus?
Let's get this question immediately out-of-the way though, shall we? Considering that one of DANM’s aspects is to be a CNI metaplugin, we are sure this question has crossed most people’s mind.

Let it be first known, that the creators of this project have a a great deal of respect towards the contributors of Multus, and actually we are working together on trying to enhance the upstream Kubernetes with many generic features (both within networking SIG, and without).
We have always viewed DANM as a temporary solution, and were waiting for the upstream community, or another project to provide the complete feature set TelCo applications require (while we were sitting on our own for more than 4 years now, way before even the concept of CNI specification was conceived. Shame on us!)
As even until today this has not happened, we thought it is time to showcase our approach instead!

**So, which CNI metaplugin is actually better?**
*The answer is very simple: **neither***. They are just a different solution to the same problem, neither of them being the ultimate one!

**Multus** is a very generic CNI metaplugin 100% employing the capabilities of other CNI plugins to manage multiple networks transparently.
Multus leaves everything to delegates, while its role remains neutral.
Multus is mainly configuration driven, and very loosely coupled.
CNI configurations are always transparent for Multus.

Whereas **DANM** is an E2E suite implementing one, big solution to a whole array of specific networking related features TelCo applications usually demand.
While Multus always delegates, DANM implements a lot of these functionalities on its own (but still being capable of delegating sub-tasks whenever it is configured to).
All components of DANM are mainly driven by and tightly coupled with a CRD-based proprietary Kubernetes API schema called DanmNet.
DANM actively interprets and mutates CNI configurations, even when it delegates network provisioning operations to other CNIs.

So, basically it is up to you to decide which solution is better suited to your needs. If you would like to remain close to the original networking ideals of Kubernetes, and only want to provision multiple network interfaces to your Pod, you might go with Multus.

If you are not afraid of going against the grain, and looking for a coupled E2E solution revolving around the idea of making networks first-class citizens in Kubernetes, you might be tempted to try DANM instead.
## Getting started
### Prerequisites

As all the features of DANM are based on Kubernetes, you will need a Kubernetes cluster up-and running before you can use any components of the DANM suite. We suggest to use any of the automated Kubernetes installing solutions (kubeadm, minikube etc.) for a painless experience.
We currently test DANM with Kubernetes 1.11.X.
Compatibility with earlier than 1.9.X versions of Kubernetes is not officially supported.
Compatibility with newer versions of Kubernetes is not tested (theoretically it should work though, considering our project uses the official REST client generator created by the K8s community).

This project currently does not have a Docker container release, so we will walk you through the entire process of building all artifacts from scratch.
To be able to do that, your development environment shall already have Docker daemon installed and ready to build containers.

Note, that the project itself depends on Golang 1.10+ and glide being available, but we packaged these dependencies into an automatically created builder container, so you don't have to worry about them!
### Building the binaries

It is actually as easy as go get-ting the repository from GitHub, and executing the build_danm.sh script from the root of the project!
```
go get https://github.com/nokia/danm
cd $GOPATH/src/github.com/nokia/danm
./build_danm.sh
```
This will first build the Alpine 3.8 based builder container, mount the $GOPATH/src and the $GOPATH/bin directory into it, and invoke the necessary script to build all binaries inside the container.
The builder container destroys itself once its purpose has been fulfilled.

The result will be 4, statically linked binaries put into your $GOPATH/bin directory.

**"danm"** is the CNI plugin which can be directly integrated with kubelet. Internally it consists of the CNI metaplugin, the CNI plugin responsible for managing IPVLAN interfaces, and the in-built IPAM plugin.
Danm binary is integrated to kubelet as any other [CNI plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/).

**"fakeipam"** is a little program used in natively integrating 3rd party CNI plugins into the DANM ecosystem. It is basically used to echo the result of DANM's in-built IPAM to CNIs DANM delegates operations to.
Fakeipam binary should be placed into kubelet's configured CNI plugin directory, next to danm.
Fakeipam is a temporary solution, the long-term aim is to separate DANM's IPAM component into a full-fledged, standalone IPAM solution.

**"netwatcher"** is a Kubernetes Controller watching the Kubernetes API for changes in the DANM related CRD path (called DanmNet).
This component is responsible for validating the semantics of network objects, and also for maintaining VxLAN and VLAN host interfaces of all Kubernetes nodes.
Netwatcher binary is deployed in Kubernetes as a DaemonSet, running on all nodes.

**"svcwatcher"** is another Kubernetes Controller monitoring Pod, Service, Endpoint, and DanmEp API paths.
This Controller is responsible for extending Kubernetes native Service Discovery to work even for the non-primary networks of the Pod.
Svcwatcher binary is deployed in Kubernetes as a DaemonSet, running only on the Kubernetes master nodes in a clustered setup.
### Building the containers
Netwatcher and svcwatcher binaries are built into their own containers.
The project contains example Dockerfiles for both components under the integration/docker directory.
Copying the respective binary into the right folder (netwatcher into integration/docker/netwatcher, svcwatcher into integration/docker/svcwatcher), then executing:
```
docker build -t netwatcher:latest integration/docker/netwatcher
docker build -t svcwatcher:latest integration/docker/svcwatcher
```
or
```
buildah bud -t netwatcher:latest integration/docker/netwatcher
buildah bud -t svcwatcher:latest integration/docker/svcwatcher
```
builds the respective containers which can be directly integrated into a running Kubernetes cluster!
## Deployment
The method of deploying the whole DANM suite into a Kubernetes cluster is the following.
**1. Extend the Kubernetes API with DANM's CRD objects (DanmNet and DanmEp) by executing the following command from the project's root directory:**
```
kubectl create -f integration/crds/
```
**2. Put the following CNI config file into the CNI configuration directory of all your kubelet nodes' (by default it is /etc/cni/net.d/):**
```
/ # cat /etc/cni/net.d/00-danm.conf
{
  "name": "meta_cni",
  "type": "danm",
  "kubeconfig": "<PATH_TO_VALID_KUBECONFIG_FILE>"
}
```
The parameter "kubeconfig" is mandatory, and shall point to a valid kubeconfig file.
As kubelet considers the first .conf file in the configured directory as the valid CNI config of the cluster, it is generally a good idea to prefix the .conf file of any CNI metaplugin with "00".

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

**6. Onboard the netwatcher and svcwatcher containers into the image registry of your cluster**

 **7. Create the netwatcher DaemonSet by executing the following command from the project's root directory:**
 ```
kubectl create -f integration/manifests/netwatcher/
```
Note: we assume RBAC is configured for the Kubernetes API, so the manifests include the required Role and ServiceAccount for this case.

You are now ready to use the services of DANM, and can start bringing-up Pods within your cluster!

 **+1. OPTIONAL: Create the servicewatcher DaemonSet by executing the following command from the project's root directory:**
 ```
kubectl create -f integration/manifests/svcwatcher/
```
This component is an optional part of the suite. You only need to install it if you would like to use Kubernetes Services for all the network interfaces of your Pod.
Note: svcwatcher already leverages DANM CNI to create its network interface. Don't forget to change the name of the network referenced in the example manifest file to one which:
 - exists in your cluster
 - and through which svcwatcher can reach the Kubernetes API server
We use Flannel for this purpose in our product. We also assume here the RBAC as the API access control method.
## User guide
This section describes what features the DANM networking suite adds to a vanilla Kubernetes environment, and how can users utilize them.

### Usage of DANM's CNI
#### Network management
The DANM CNI is a full-fledged CNI metaplugin, capable of provisioning multiple network interfaces to a Pod, on-demand!
DANM can utilize any of the existing and already integrated CNI plugins to do so.

Network management starts with the creation of Kubernetes network objects, called DanmNets in DANM terminology. A DanmNet logically represents the characteristics of a network Pods can connect to.
Users first need to create a DanmNet according to the schema described in the **schema/DanmNet.yaml** template file.
A DanmNet object can be created just like any other Kubernetes object, for example by issuing:
 ```
kubectl create -f test-net1.yaml
```
Users can also interact with existing DanmNet objects just as they would with other core API objects:
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
      End: 192.168.1.10
      Start: 192.168.1.100
    Container _ Prefix:  eth0
    Host _ Device:       ens4
    Rt _ Tables:         201
  Validation:            True
Events:                  <none>
```
#### Generally supported DANM API features
##### Naming container interfaces
Generally speaking, you need to care about how the network interfaces of your Pods are named inside their respective network namespaces.
The hard reality to keep in mind is that you shall always have an interface literally called "eth0" created within all your Kubernetes Pods, because Kubelet will always search for the existence of such an interface at the end of Pod instantiation.
If such an interface does not exist after CNI is invoked, the state of the Pod will be considered "faulty", and it will be re-created in a loop.
To be able to comply with this Kubernetes limitation, DANM supports both explicit, and implicit interface naming schemes for all NetworkTypes!

An interface connected to a DanmNet containing the container_prefix attribute will be always named accordingly. You can use this API to explicitly set descriptive, unique names to NICs connecting to this network.
In case container_prefix is not set in an interface's network descriptor, DANM will automatically name the interface "ethX", where X is a unique integer number corresponding to the sequence number of the network connection (e.g. the first interface defined in the annotation is called "eth0", second interface "eth1" etc.)
DANM even supports the mixing of the networking schemes within the same Pod, and it supports the whole naming scheme for all network backends.
While the feature provides complete control over the name of interfaces, ultimately it is the network administrators' responsibility to:
 - make sure exactly one interface is named eth0 in every Pod
 - don't configure multiple NICs into the same Pod with clashing names (e.g. provisioning two implicitly named interfaces, and then a third one explicitly named "eth0", or "eth1" etc.)
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
Pay special attention to the DanmNet attribute called "NetworkType". This parameter controls which CNI plugin is invoked by the DANM metaplugin during the execution of a CNI operation to setup, or delete exactly one network interface of a Pod.

In case this parameter is set to "ipvlan", or is missing; then DANM's in-built IPVLAN CNI plugin creates the network (see next chapter for details).
In case this attribute is provided and set to another value than "ipvlan", then network management is delegated to the CNI plugin with the same name.
The binary will be searched in the configured CNI binary directory.
Example: when a Pod is created and requests a network connection to a DanmNet with "NetworkType" set to "flannel", then DANM will delegate the creation of this network interface to the /opt/cni/bin/flannel binary.
##### Creating the configuration for delegated CNI operations
We strongly believe that network management in general should be driven by one, generic API. Therefore, DANM is capable to "translate" the generic options coming from a DanmNet object into the specific "language" the delegated CNI plugin understands.
This way users can dynamically configure various networking solutions via the same, abstract interface without caring about how a specific option is called exactly in the terminology of the delegated solution.

A generic framework supporting this method is built into DANM's code, but still this level of integration requires case-by-case implementation.
As a result, DANM currently supports two integration levels:

 - **Dynamic integration level:** CNI-specific network attributes (such as IP ranges, parent host devices etc.) can be controlled on a per network level, taken directly from a DanmNet object
 - **Static integration level:** CNI-specific network attributes (such as IP ranges, parent host devices etc.) can be only configured on a per node level, via a static CNI configuration files (Note: this is the default CNI configuration method)

Our aim is to integrate all the popular CNIs into the DANM eco-system over time, but currently the following CNI's achieved dynamic integration level:

 - DANM's own, in-built IPVLAN CNI plugin
	 - Set the "NetworkType" parameter to value "ipvlan" to use this backend
- Intel's DPDK-capable [SR-IOV CNI plugin](https://github.com/intel/sriov-cni )
	- Set the "NetworkType" parameter to value "sriov" to use this backend
- Generic MACVLAN CNI from the CNI plugins example repository [MACVLAN CNI plugin](https://github.com/containernetworking/plugins/blob/master/plugins/main/macvlan/macvlan.go )
	- Set the "NetworkType" parameter to value "macvlan" to use this backend

No separate configuration needs to be provided to DANM when it connects Pods to DanmNets, if the network is backed by a CNI plugin with dynamic integration level.
Everything happens automatically based on the DanmNet API itself!

When network management is delegated to CNI plugins with static integration level; DANM will read their configuration from the configured CNI config directory.
For example, when a Pod is connected to a DanmNet with "NetworkType" set to "flannel", DANM will pass the content of /etc/cni/net.d/flannel.conf file to the /opt/cni/bin/flannel binary by invoking a standard CNI operation.
Generally supported DANM API-based features are configured even in this case.
##### Connecting Pods to DanmNets
Pods can request network connections to DanmNets by defining one or more network connections in the annotation of their (template) spec field, according to the schema described in the **schema/network_attach.yaml** file.

For each connections defined in such a manner DANM will provision exactly one interface into the Pod's network namespace, according to the way described in previous chapters (configuration taken from the referenced DanmNet API object).
In addition to simply invoking other CNI libraries to set-up network connections, Pod's can even influence the way their interfaces are created to a certain extent.
For example Pods can ask DANM to provision L3 IP addresses to their IPVLAN, MACVLAN or SR-IOV interfaces dynamically, statically, or not at all!
Or, as described earlier; creation of policy-based L3 IP routes into their network namespace is also a supported by the solution.

If the Pod annotation is empty (no DanmNet connections are defined), DANM will try and fall back to a configured default network definition.
A default network can be configured for a namespace by creating one DanmNet object with ObjectMeta.Name field set to "default" in the Pod's namespace. There are no restrictions as to what DANM supported attributes can be configured for a default network. However, in this case users cannot specify any further fine-grained properties for the Pod (i.e. static IP address, policy-based IP routes).
This feature is beneficial for cluster operators who would like to use unmodified manifest files (i.e. community maintained Helm charts or Pods created by K8S operators), or would like to use DANM in the "K8s intended" way.

##### Internal workings of the metaplugin
Regardless which CNI plugins are involved in managing the networks of a Pod, and how they are configured; DANM will invoke all of them at the same time, in parallel threads.

DANM will wait for the CNI result of all executors before converting, and merging them together into one summarized object. The aggregated result is then sent back to kubelet.

If any executor reported an error, or hasn't finished its job even after 10 seconds; the result of the whole operation will be an error.
DANM will report all errors towards kubelet if multiple CNI plugins failed to do their job.
#### DANM IPAM
DANM includes a fully generic and very flexible IPAM module in-built into the solution. The usage of this module is seamlessly integrated together with the natively supported CNI plugins, that is, DANM's IPVLAN, Intel's SR-IOV, and the CNI project's reference MACVLAN plugin.

That is because just like the above CNIs, configuration of DANM's IPAM is also integrated into the DANM's Kubernetes API extension called DanmNet through the attributes called "cidr" and "allocation_pool". Therefore users of the module can easily configure all aspects of network management by manipulating solely dynamic Kubernetes API objects!

This native integration also enables a very tempting possibility. **As IP allocations belonging to a network are dynamically tracked *within the same API object***, it becomes possible to define:
* discontinuous subnets 1:1 mapped to a logical network
* **cluster-wide usable subnets** (instead of node restricted sub CIDRs)

Network administrators can simply put the CIDR, and the allocation pool into the DanmNet object. Whenever a Pod is instantiated or deleted **on any host within the cluster**, DANM simply updates the allocation record belonging to the network through the Kubernetes API before provisioning the chosen IP to the Pod's interface.

The flexible IPAM module also allows Pods to define the IP allocation scheme best suited for them. Pods can ask dynamically allocated IPs from the defined allocation pool, or can ask for one, specific, static address.
The application can even ask DANM to forego the allocation of any IPs to their interface in case a L2 network interface is required.
#### DANM IPVLAN CNI
DANM's IPVLAN CNI uses the Linux kernel's IPVLAN module to provision high-speed, low-latency network interfaces for applications which need better performance than a bridge (or any other overlay technology) can provide.

*Keep in mind that the IPVLAN module is a fairly recent addition to the Linux kernel, so the feature cannot be used on systems whose kernel is older than 4.4!
4.9, 4.11, or 4.14 would be even better (lotta bug fixes)*

The CNI provisions IPVLAN interfaces in L2 mode, and supports the following extra features:
* attaching IPVLAN sub-interfaces to any host interface
* attaching IPVLAN sub-interfaces to dynamically created VLAN or VxLAN host interfaces
* renaming the created interfaces according to the "container_prefix" attribute defined in the DanmNet object
* allocating IP addresses by using DANM's flexible, in-built IPAM module
* provisioning generic IP routes into a configured routing table inside the Pod's network namespace
* Pod-level controlled provisioning of policy-based IP routes into Pod's network namespace

### Usage of DANM's Netwatcher component
Netwatcher is a mandatory component of the DANM networking suite.
It is implemented using Kubernetes' Informer paradigm, and is deployed as a DaemonSet.
It shall be running on all hosts where DANM CNI is the configured CNI plugin.

The netwatcher component has two responsibilities:
* it semantically validates all the newly created DanmNet objects (as Kubernetes' OpenAPI based CRD validation feature cannot handle all the required rules)
* it dynamically manages (i.e. creates and deletes) VxLAN and VLAN interfaces on all the hosts based on the dynamic DanmNet API

Whenever a DanmNet is created or deleted within the Kubernetes cluster, netwatcher will be triggered. If the DanmNet in question contained either the "vxlan", or the "vlan" attributes; then netwatcher immediately creates, or deletes the VLAN or VxLAN host interface with the matching VID.

This feature is the most beneficial when used together with a fully dynamic network provisioning backend. Whenever a Pod is connected to a DanmNet defining such attribute, the CNI will automatically connect the created interface to the VxLAN or VLAN host interface created by the netwatcher; instead of directly connecting it to the configured host interface.
### Usage of DANM's Svcwatcher component
#### Feature description
Svcwatcher component showcases the whole reason why DANM exists, and is architected the way it is. It is the first higher-level feature which accomplishes our true intention (described earlier), that is, extending basic Kubernetes constructs to seamlessly work with multiple network interfaces.

The first such construct is the Kubernetes Service!
Let's see how it works.

Svcwatcher basically works the same way as the default Service controller inside Kubernetes. It continuously monitors both the Service and the Pod APIs, and provisions Endpoints whenever the cluster administrator creates, updates, or deletes relevant API objects (e.g. creates a new Service, updates a Pod label etc.).

DANM svcwatcher does the same, and more! The default Service controller assumes the Pod has one interface, so whenever a logical Service Endpoint is created it will be always done with the IP of the Pod's first (the infamous "eth0" in Kubernetes), and supposedly only network interface.

DANM svcwatcher on the other hand makes this behaviour configurable! DANM enhances the same Service API so an object will always explicitly select one logical network (a DanmNet in DANM terminology), rather than implicitly choose the one with the hard-coded name of "eth0".
Then, svcwatcher will provision a Service Endpoint with the address of the selected Pod's chosen network interface.

This enhancement basically upgrades the in-built Kubernetes Service Discovery concept to work over multiple network interfaces, making Service Discovery only return truly relevant Endpoints in every scenario!
#### Svcwatcher compatible Service descriptors
Based on the feature description experienced Kubernetes users are probably already thinking "but wait, there is no "network selector" field in the Kubernetes Service core API".
That is indeed true right now, but consider the core concept behind the creation of DANM: "what use-cases would become possible if Networks would be part of the core Kubernetes API"?
So, we went ahead and simulated exactly this scenario, while making sure our solution also works with a vanilla Kubernetes today; just as we did with all our other API enhancements.

This is possible by leveraging the so-called "headless and selectorless Services" concept in Kubernetes. Headless plus selectorless Services do not contain Pod selector field, which tells the Kubernetes native Service controller that Endpoint administration is handled by a 3rd party service.

DANM svcwatcher is triggered when such a service is created, if it contains the DANM "core API" attributes in their annotation.
These extra attributes are the following:
"danm.k8s.io/selector": this selector serves the exact same purpose as the default Pod selector field (which is missing from a selectorless Service by definition). Endpoints will be created for Pods which match all labels provided in this list
"danm.k8s.io/network": this is the "special sauce" of DANM. When svcwatcher creates an Endpoint, it's IP will be taken from the selected Pod's physical interface connected to the DanmNet with the matching name

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

* **Robert Springer** (@rospring) - Initial work (V1 Python), IPAM, Netwatcher, Servicewatcher [Nokia](https://github.com/nokia)
* **Levente Kale** (@Levovar) - Initial work (V2 Golang), Documentation, Integration, SCM, Metaplugin [Nokia](https://github.com/nokia)

Special thanks to the original author who started the whole project in 2015 by putting a proprietary network management plugin between Kubelet and Docker; and also for coining the DANM acronym:
**Peter Braun** (@peter-braun)

## License

This project is licensed under the 3-Clause BSD License - see the [LICENSE](LICENSE)
