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
  supportedNativeCnis = map[string]*cniBackendConfig {
    "sriov": &cniBackendConfig {
      CniBackend: danmtypes.CniBackend {
        CNIVersion: "0.3.1",
      },
      readConfig: cniConfigReader(getSriovCniConfig),
      ipamNeeded: true,
      deviceNeeded: true,
    },
    "macvlan": &cniBackendConfig {
      CniBackend: danmtypes.CniBackend {
        CNIVersion: "0.3.1",
      },
      readConfig: cniConfigReader(getMacvlanCniConfig),
      ipamNeeded: true,
      deviceNeeded: false,
    },
  }
)

//This function creates CNI configuration for all static-level backends
//The CNI binary matching with NetowrkType is invoked with the CNI config file matching with NetworkID parameter
func readCniConfigFile(cniconfDir string, netInfo *danmtypes.DanmNet) ([]byte, error) {
  cniConfig := netInfo.Spec.NetworkID
  rawConfig, err := ioutil.ReadFile(cniconfDir + "/" + cniConfig + ".conf")
  if err != nil {
    return nil, errors.New("Could not load CNI config file: " + cniConfig +".conf for plugin:" + netInfo.Spec.NetworkType + " from directory:" + cniconfDir)
  }
  return rawConfig, nil
}

//This function creates CNI configuration for the dynamic-level SR-IOV backend
func getSriovCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp, cniVersion string) ([]byte, error) {
  var sriovConfig SriovNet
  // initialize common fields of "github.com/containernetworking/cni/pkg/types".NetConf
  sriovConfig.CNIVersion = cniVersion
  sriovConfig.Name       = netInfo.Spec.NetworkID
  sriovConfig.Type       = "sriov"
  // initialize SriovNet specific fields:
  pfname, err := sriov_utils.GetPfName(ep.Spec.Iface.DeviceID)
  if err != nil {
    return nil, errors.New("failed to get the name of the sriov PF for device "+ ep.Spec.Iface.DeviceID +" due to:" + err.Error())
  }
  sriovConfig.PfName   = pfname
  sriovConfig.L2Mode   = true
  sriovConfig.Vlan     = netInfo.Spec.Options.Vlan
  sriovConfig.Ipam     = ipamOptions
  sriovConfig.DeviceID = ep.Spec.Iface.DeviceID
  if len(ipamOptions.Ips) > 0 {
    sriovConfig.L2Mode = false
  }
  rawConfig, err := json.Marshal(sriovConfig)
  if err != nil {
    return nil, errors.New("Error putting together CNI config for SR-IOV plugin: " + err.Error())
  }
  return rawConfig, nil
}

//This function creates CNI configuration for the dynamic-level MACVLAN backend
func getMacvlanCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp, cniVersion string) ([]byte, error) {
  var macvlanConfig MacvlanNet
  // initialize common fields of "github.com/containernetworking/cni/pkg/types".NetConf
  macvlanConfig.CNIVersion = cniVersion
  // initialize MacvlanNet specific fields:
  macvlanConfig.Master = danmep.DetermineHostDeviceName(netInfo)
  macvlanConfig.Mode   = "bridge" //TODO: make these params configurable if required
  macvlanConfig.MTU    = 1500
  macvlanConfig.Ipam   = ipamOptions
  rawConfig, err := json.Marshal(macvlanConfig)
  if err != nil {
    return nil, errors.New("Error putting together CNI config for MACVLAN plugin: " + err.Error())
  }
  return rawConfig, nil
}
