package cnidel

import (
  "github.com/containernetworking/cni/pkg/types"
  sriov_types "github.com/intel/sriov-cni/pkg/types"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/datastructs"
)

type cniConfigReader func(netInfo *danmtypes.DanmNet, ipam datastructs.IpamConfig, ep *danmtypes.DanmEp, cniVersion string) ([]byte, error)

type cniBackendConfig struct {
  datastructs.CniBackend
  readConfig cniConfigReader
  ipamNeeded bool
  deviceNeeded bool
}

// sriovNet represent the configuration of sriov cni v1.0.0
type SriovNet struct {
  sriov_types.NetConf
  // IPAM configuration to be used for this network
  Ipam   datastructs.IpamConfig `json:"ipam,omitEmpty"`
}

type MacvlanNet struct {
  types.NetConf
  //Name of the master NIC the MACVLAN slave needs to be connected to
  Master string `json:"master"`
  //The mode in which the MACVLAN slave is configured (default bridge)
  Mode   string `json:"mode"`
  //MTU to be set to the MACVLAN slave interface (default 1500)
  MTU    int    `json:"mtu"`
  //IPAM configuration to be used for this network
  Ipam   datastructs.IpamConfig `json:"ipam,omitEmpty"`
}
