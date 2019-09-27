package v1

import (
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmNet struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmNetSpec `json:"spec"`
}

type DanmNetSpec struct {
  NetworkID   string         `json:"NetworkID"`
  NetworkType string         `json:"NetworkType,omitempty"`
  AllowedTenants []string    `json:"AllowedTenants,omitempty"`
  Options     DanmNetOption  `json:"Options,omitempty"`
}

type DanmNetOption struct {
  // The device to where the network is attached
  Device string  `json:"host_device,omitempty"`
  // The resource_pool contains allocated device IDs
  DevicePool string  `json:"device_pool,omitempty"`
  // the vxlan id on the host device (creation of vxlan interface)
  Vxlan  int  `json:"vxlan,omitempty"`
  // The name of the interface in the container
  Prefix string  `json:"container_prefix,omitempty"`
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
  RTables int `json:"rt_tables,omitempty"`
  // the VLAN id of the VLAN interface created on top of the host device
  Vlan  int  `json:"vlan,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmNetList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []DanmNet `json:"items"`
}

type IP4Pool struct {
  Start string `json:"start"`
  End   string `json:"end"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmEp struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmEpSpec `json:"spec"`
}

type DanmEpSpec struct {
  NetworkName string      `json:"NetworkName"`
  NetworkType string      `json:"NetworkType"`
  EndpointID  string      `json:"EndpointID"`
  Iface       DanmEpIface `json:"Interface"`
  Host        string      `json:"Host,omitempty"`
  Pod         string      `json:"Pod"`
  CID         string      `json:"CID,omitempty"`
  Netns       string      `json:"netns,omitempty"`
  ApiType     string      `json:"apiType"`
}

type DanmEpIface struct {
  Name        string            `json:"Name"`
  Address     string            `json:"Address"`
  AddressIPv6 string            `json:"AddressIPv6"`
  MacAddress  string            `json:"MacAddress"`
  Proutes     map[string]string `json:"proutes"`
  Proutes6    map[string]string `json:"proutes6"`
  DeviceID    string            `json:"DeviceID,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DanmEpList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []DanmEp `json:"items"`
}

// VERY IMPORTANT NOT TO CHANGE THIS, INCLUDING THE EMPTY LINE BETWEEN THE ANNOTATIONS!!!
// https://github.com/kubernetes/code-generator/issues/59
// +genclient:nonNamespaced

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
type TenantConfig struct {
  meta_v1.TypeMeta              `json:",inline"`
  meta_v1.ObjectMeta            `json:"metadata"`
  HostDevices []IfaceProfile    `json:"hostDevices,omitempty"`
  NetworkIds  map[string]string `json:"networkIds,omitempty"`
}

type IfaceProfile struct {
  Name      string `json:"name"`
  VniType   string `json:"vniType,omitempty"`
  VniRange  string `json:"vniRange,omitempty"`
  Alloc     string  `json:"alloc,omitempty"`
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TenantConfigList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []TenantConfig `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TenantNetwork struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmNetSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TenantNetworkList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []TenantNetwork `json:"items"`
}

// VERY IMPORTANT NOT TO CHANGE THIS, INCLUDING THE EMPTY LINE BETWEEN THE ANNOTATIONS!!!
// https://github.com/kubernetes/code-generator/issues/59
// +genclient:nonNamespaced

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
type ClusterNetwork struct {
  meta_v1.TypeMeta   `json:",inline"`
  meta_v1.ObjectMeta `json:"metadata"`
  Spec               DanmNetSpec `json:"spec"`
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterNetworkList struct {
  meta_v1.TypeMeta `json:",inline"`
  meta_v1.ListMeta `json:"metadata"`
  Items            []ClusterNetwork `json:"items"`
}