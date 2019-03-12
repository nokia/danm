package v1

import (
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
  OptimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"
)

type CniBackend struct {
  BackendName string
  CniVersion string
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmNet struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmNetSpec `json:"spec"`
}

type DanmNetSpec struct {
  NetworkID   string        `json:"NetworkID"`
  NetworkType string        `json:"NetworkType,omitempty"`
  Options     DanmNetOption `json:"Options"`
  Validation  bool          `json:"Validation,omitempty"`
}

type DanmNetOption struct {
  // The device to where the network is attached
  Device string  `json:"host_device"`
  // The resource_pool contains allocated device IDs
  DevicePool string  `json:"device_pool,omitempty"`
  // the vxlan id on the host device (creation of vxlan interface)
  Vxlan  int  `json:"vxlan,omitempty"`
  // The name of the interface in the container
  Prefix string  `json:"container_prefix"`
  // IPv4 specific parameters
  // IPv4 network address
  Cidr   string  `json:"cidr,omitempty"`
  // IPv4 routes for this network
  Routes map[string]string  `json:"routes,omitempty"`
  // bit array of tracking address allocation
  Alloc  string  `json:"alloc,omitempty"`
  // subset of the Cidr from where dynamic IP address allocation happens
  Pool   IP4Pool `json:"allocation_pool,omitEmpty"`
  // IPv6 specific parameters
  // IPv6 unique global address prefix
  Net6    string  `json:"net6,omitempty"`
  // IPv6 routes for this network
  Routes6 map[string]string  `json:"routes6,omitempty"`
  // Routing table number for policy routing
  RTables int `json:"rt_tables"`
  // the VLAN id of the VLAN interface created on top of the host device
  Vlan  int  `json:"vlan,omitempty"`
}

type IP4Pool struct {
  Start string `json:"start"`
  End   string `json:"end"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmNetList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []DanmNet `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmEp struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmEpSpec `json:"spec"`
}

type DanmEpSpec struct {
  NetworkID   string      `json:"NetworkID"`
  NetworkType string      `json:"NetworkType"`
  EndpointID  string      `json:"EndpointID"`
  Iface       DanmEpIface `json:"Interface"`
  Host        string      `json:"Host,omitempty"`
  Pod         string      `json:"Pod"`
  CID         string      `json:"CID,omitempty"`
  Netns       string      `json:"netns,omitempty"`
  Creator     string      `json:"Creator,omitempty"`
  Expires     string      `json:"Expires,omitempty"`
}

type DanmEpIface struct {
  Name        string            `json:"Name"`
  Address     string            `json:"Address"`
  AddressIPv6 string            `json:"AddressIPv6"`
  MacAddress  string            `json:"MacAddress"`
  Proutes     map[string]string `json:"proutes"`
  Proutes6    map[string]string `json:"proutes6"`
  DeviceID  string            `json:"DeviceID,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmEpList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []DanmEp `json:"items"`
}

// Interface represents a request coming from the Pod to connect it to one DanmNet during CNI_ADD operation
// It contains the name of the DanmNet the Pod should be connected to, and other optional requests
// Pods can influence the scheme of IP allocation (dynamic, static, none),
// and can ask for the provisioning of policy-based IP routes
type Interface struct {
  Network string `json:"network"`
  Ip string `json:"ip"`
  Ip6 string `json:"ip6"`
  Proutes map[string]string `json:"proutes"`
  Proutes6 map[string]string `json:"proutes6"`
  DefaultIfaceName string
  Device string `json:"Device,omitempty"`
}

type IpamConfig struct {
  Type      string      `json:"type"`
  Subnet    string      `json:"subnet"`
  Routes    []IpamRoute `json:"routes,omitEmpty"`
  DefaultGw string      `json:"gateway,omitEmpty"`
  Ip        string      `json:"ip"`
}

type IpamRoute struct {
  Dst string `json:"dst"`
  Gw  string `json:"gw,omitEmpty"`
}
