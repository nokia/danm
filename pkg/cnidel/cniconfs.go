package cnidel

import (  
  "errors"
  "encoding/json"
  "io/ioutil"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/danmep"
  sriov_utils "github.com/intel/sriov-cni/pkg/utils"
)

var (
  supportedNativeCnis = []*cniBackendConfig {
    &cniBackendConfig {
      danmtypes.CniBackend {
        BackendName: "sriov",
        CniVersion: "0.3.1",
      },
      cniConfigReader(getSriovCniConfig),
      true,
      true,
    },
    &cniBackendConfig {
      danmtypes.CniBackend {
        BackendName: "macvlan",
        CniVersion: "0.3.1",
      },
      cniConfigReader(getMacvlanCniConfig),
      true,
      false,
    },
  }
)

//This function creates CNI configuration for all static-level backends
//The CNI binary matching with NetowrkType is invoked with the CNI config file matching with NetworkID parameter
func readCniConfigFile(netInfo *danmtypes.DanmNet) ([]byte, error) {
  cniConfig := netInfo.Spec.NetworkID
  rawConfig, err := ioutil.ReadFile(cniConfigDir + "/" + cniConfig + ".conf")
  if err != nil {
    return nil, errors.New("Could not load CNI config file: " + cniConfig +" for plugin:" + netInfo.Spec.NetworkType)
  }
  return rawConfig, nil
}

//This function creates CNI configuration for the dynamic-level SR-IOV backend
func getSriovCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  pfname, err := sriov_utils.GetPfName(ep.Spec.Iface.DeviceID)
  if err != nil {
    return nil, errors.New("failed to get the name of the sriov PF for device "+ ep.Spec.Iface.DeviceID +" due to:" + err.Error())
  }
  vlanid := netInfo.Spec.Options.Vlan
  sriovConfig := sriovNet {
    Name:      netInfo.Spec.NetworkID,
    Type:      "sriov",
    PfName:    pfname,
    L2Mode:    true,
    Vlan:      vlanid,
    Ipam:      ipamOptions,
    DeviceID:  ep.Spec.Iface.DeviceID,
  }
  if ipamOptions.Ip != "" {
    sriovConfig.L2Mode = false
  }
  rawConfig, err := json.Marshal(sriovConfig)
  if err != nil {
    return nil, errors.New("Error putting together CNI config for SR-IOV plugin: " + err.Error())
  }
  return rawConfig, nil
}

//This function creates CNI configuration for the dynamic-level MACVLAN backend
func getMacvlanCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  hDev := danmep.DetermineHostDeviceName(netInfo)
  macvlanConfig := macvlanNet {
    Master: hDev,
   //TODO: make these params configurable if required
    Mode:   "bridge",
    MTU:    1500,
    Ipam:   ipamOptions,
  }
  rawConfig, err := json.Marshal(macvlanConfig)
  if err != nil {
    return nil, errors.New("Error putting together CNI config for MACVLAN plugin: " + err.Error())
  }
  return rawConfig, nil
}