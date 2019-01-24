package cnidel

import (
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
)

type cniConfigReader func(netInfo *danmtypes.DanmNet, ipam danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error)

type cniBackendConfig struct {
  danmtypes.CniBackend
  readConfig cniConfigReader
  ipamNeeded bool
}

// sriovNet represent the configuration of sriov plugin
type sriovNet struct {
  // the name of the network
  Name   string     `json:"name"`
  // currently constant "sriov"
  Type   string     `json:"type"`
  // name of the PF
  PfName string     `json:"if0"`
  // interface name in the Container
  IfName string     `json:"if0name,omitEmpty"`
  // if true then add VF as L2 mode only, IPAM will not be executed
  L2Mode bool       `json:"l2enable,omitEmpty"`
  // VLAN ID to assign for the VF
  Vlan   int        `json:"vlan,omitEmpty"`
  // IPAM configuration to be used for this network
  Ipam   danmtypes.IpamConfig `json:"ipam,omitEmpty"`
  // DPDK configuration
  Dpdk   DpdkOption `json:"dpdk,omitEmpty"`
}

// DpdkOption represents the DPDK options for the sriov plugin
type DpdkOption struct {
  // The name of kernel NIC driver
  NicDriver  string `json:"kernel_driver"`
  // The name of DPDK capable driver
  DpdkDriver string `json:"dpdk_driver"`
  // Path to the dpdk-devbind.py script
  DpdkTool   string `json:"dpdk_tool"`
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