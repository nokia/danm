package cnidel

import (  
  "errors"
  "log"
  "io/ioutil"
  "encoding/json"
  "github.com/nokia/danm/pkg/danmep"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
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
    },
    &cniBackendConfig {
      danmtypes.CniBackend {
        BackendName: "macvlan",
        CniVersion: "0.3.1",
      },
      cniConfigReader(getMacvlanCniConfig),
      true,
    },
  }
)

//This function creates CNI configuration for all static-level backends
func readCniConfigFile(netInfo *danmtypes.DanmNet) ([]byte, error) {
  cniType := netInfo.Spec.NetworkType
  //TODO: the path from where the config is read should not be hard-coded
  rawConfig, err := ioutil.ReadFile("/etc/cni/net.d/" + cniType + ".conf")
  if err != nil {
    return nil, errors.New("Could not load CNI config file for plugin:" + cniType)
  }
  return rawConfig, nil
}

//This function creates CNI configuration for the dynamic-level SR-IOV backend
func getSriovCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  vlanid := netInfo.Spec.Options.Vlan
  sriovConfig := sriovNet {
    Name:   netInfo.Spec.NetworkID,
    Type:   "sriov",
    PfName: netInfo.Spec.Options.Device,
    IfName: ep.Spec.Iface.Name,
    L2Mode: true,
    Vlan:   vlanid,
    Dpdk:   DpdkOption{},
    Ipam:   ipamOptions,
  }
  if ipamOptions.Ip != "" {
    sriovConfig.L2Mode = false
  }
  if netInfo.Spec.Options.Dpdk {
    sriovConfig.Dpdk = DpdkOption {
      NicDriver: dpdkNicDriver,
      DpdkDriver: dpdkDriver,
      DpdkTool: dpdkTool,
    }
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
  log.Printf("LOFASZ MACVLAN CONFIG %v/n",macvlanConfig)
  rawConfig, err := json.Marshal(macvlanConfig)
  if err != nil {
    return nil, errors.New("Error putting together CNI config for MACVLAN plugin: " + err.Error())
  }
  return rawConfig, nil
}