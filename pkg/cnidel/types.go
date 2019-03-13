package cnidel

import (
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
)

type cniConfigReader func(netInfo *danmtypes.DanmNet, ipam danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error)

type cniBackendConfig struct {
  danmtypes.CniBackend
  readConfig cniConfigReader
  ipamNeeded bool
  deviceNeeded bool
}

// sriovNet represent the configuration of sriov plugin v1.0.0
type sriovNet struct {
  // the name of the network
  Name   string     `json:"name"`
  // currently constant "sriov"
  Type   string     `json:"type"`
  // name of the PF since sriov cni v1.0.0
  PfName string     `json:"master"`
  // interface name in the Container
  IfName string     `json:"if0name,omitEmpty"`
  // if true then add VF as L2 mode only, IPAM will not be executed
  L2Mode bool       `json:"l2enable,omitEmpty"`
  // VLAN ID to assign for the VF
  Vlan   int        `json:"vlan,omitEmpty"`
  // IPAM configuration to be used for this network
  Ipam   danmtypes.IpamConfig `json:"ipam,omitEmpty"`
  // CNI binary location
  CNIDir string `json:"cniDir"`
  // Device PCI ID
  DeviceID string `json:"deviceID"`
  // Device Info
  DeviceInfo *VfInformation `json:"deviceinfo,omitempty"`
}

// VfInformation is a DeviceInfo desctiprtor expected by sriov plugin v1.0.0
type VfInformation struct {
  PCIaddr string `json:"pci_addr"`
  Pfname  string `json:"pfname"`
  Vfid    int    `json:"vfid"`
}

type macvlanNet struct {
  //Name of the master NIC the MACVLAN slave needs to be connected to
  Master string `json:"master"`
  //The mode in which the MACVLAN slave is configured (default bridge)
  Mode   string `json:"mode"`
  //MTU to be set to the MACVLAN slave interface (default 1500)
  MTU    int    `json:"mtu"`
  //IPAM configuration to be used for this network
  Ipam   danmtypes.IpamConfig `json:"ipam,omitEmpty"`
}