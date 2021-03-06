module github.com/nokia/danm

go 1.13

require (
	github.com/apparentlymart/go-cidr v1.0.1
	github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins v0.8.5
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/intel/sriov-cni v2.1.0+incompatible
	github.com/j-keck/arping v1.0.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.1-0.20201119153432-9d213757d22d
	github.com/satori/go.uuid v1.2.1-0.20181028125025-b2ce2384e17b
	github.com/vishvananda/netlink v1.1.1-0.20200221165523-c79a4b7b4066
	gopkg.in/intel/multus-cni.v3 v3.7.1
	k8s.io/api v0.19.10
	k8s.io/apimachinery v0.19.10
	k8s.io/client-go v0.19.10
	k8s.io/code-generator v0.19.10
	k8s.io/kubernetes v1.14.10
)

replace k8s.io/client-go => k8s.io/client-go v0.19.10
