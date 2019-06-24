package cnidel

import (
  "errors"
  "log"
  "os"
  "strconv"
  "strings"
  "path/filepath"
  "github.com/containernetworking/cni/pkg/invoke"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/danmep"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/ipam"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
)

const (
  LegacyNamingScheme = "legacy"
  CniAddOp = "ADD"
  CniDelOp = "DEL"
)

var (
  ipamType = "fakeipam"
  defaultDataDir = "/var/lib/cni/networks"
  flannelBridge = GetEnv("FLANNEL_BRIDGE", "cbr0")
)

// IsDelegationRequired decides if the interface creation operations should be delegated to a 3rd party CNI, or can be handled by DANM
// Decision is made based on the NetworkType parameter of the network object
func IsDelegationRequired(netInfo *danmtypes.DanmNet) bool {
  neType := strings.ToLower(netInfo.Spec.NetworkType)
  if neType == "ipvlan" || neType == "" {
    return false
  }
  return true
}

// DelegateInterfaceSetup delegates K8s Pod network interface setup task to the input 3rd party CNI plugin
// Returns the CNI compatible result object, or an error if interface creation was unsuccessful, or if the 3rd party CNI config could not be loaded
func DelegateInterfaceSetup(netConf *datastructs.NetConf, danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) (*current.Result,error) {
  var (
    err error
    ipamOptions datastructs.IpamConfig
  )
  if isIpamNeeded(netInfo.Spec.NetworkType) {
   ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6, _, err = ipam.Reserve(danmClient, *netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
      return nil, errors.New("IP address reservation failed for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
    }
   //TODO: as netInfo is only copied to IPAM above, the IP allocation is not refreshed in the original copy.
   //Therefore, anyone wishing to further update the same DanmNet later on will use an outdated representation as the input.
   //IPAM should be refactored to always pass back the up-to-date DanmNet object.
   //I guess it is okay now because we only want to free IPs, and RV differences are resolved by the generated client code.
    ipamOptions, err = getCniIpamConfig(netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
      return nil, errors.New("IPAM config creation failed for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
    }
  }
  rawConfig, err := getCniPluginConfig(netConf, netInfo, ipamOptions, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return nil, err
  }
  cniType := netInfo.Spec.NetworkType
  cniResult,err := execCniPlugin(cniType, CniAddOp, netInfo, rawConfig, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return nil, errors.New("Error delegating ADD to CNI plugin:" + cniType + " because:" + err.Error())
  }
  if cniResult != nil {
    setEpIfaceAddress(cniResult, &ep.Spec.Iface)
  }
  err = danmep.CreateRoutesInNetNs(*ep, netInfo)
  if err != nil {
    // We don't consider this serious error, so we only log a warning about the issue.
    log.Println("WARNING: Could not create IP routes for CNI:" + cniType + " because:" + err.Error())
  }
  return cniResult, nil
}

func isIpamNeeded(cniType string) bool {
  if cni, ok := SupportedNativeCnis[strings.ToLower(cniType)]; ok {
    return cni.ipamNeeded
  } else {
    return false
  }
}

func IsDeviceNeeded(cniType string) bool {
  if cni, ok := SupportedNativeCnis[strings.ToLower(cniType)]; ok {
    return cni.deviceNeeded
  } else {
    return false
  }
}

func getCniIpamConfig(netinfo *danmtypes.DanmNet, ip4, ip6 string) (datastructs.IpamConfig, error) {
  var ipSlice = []datastructs.IpamIp{}
  if ip4 == "" && ip6 == "" && netinfo.Spec.NetworkType != "sriov" {
    return datastructs.IpamConfig{}, errors.New("unfortunetaly 3rd party CNI plugins usually don't support foregoing putting any IPs on an interface, so with heavy hearts but we need to fail this network delegation operation")
  }
  if ip4 != "" {
    ipSlice = append(ipSlice, datastructs.IpamIp{
                                IpCidr: ip4,
                                Version: 4,
                              })
  }
  if ip6 != "" {
    ipSlice = append(ipSlice, datastructs.IpamIp{
                                IpCidr: ip6,
                                Version: 6,
                              })
  }
  return  datastructs.IpamConfig{
            Type: ipamType,
            Ips: ipSlice,
          }, nil
}

func getCniPluginConfig(netConf *datastructs.NetConf, netInfo *danmtypes.DanmNet, ipamOptions datastructs.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  if cni, ok := SupportedNativeCnis[strings.ToLower(netInfo.Spec.NetworkType)]; ok {
    return cni.readConfig(netInfo, ipamOptions, ep, cni.CNIVersion)
  } else {
    return readCniConfigFile(netConf.CniConfigDir, netInfo)
  }
}

func execCniPlugin(cniType, cniOpType string, netInfo *danmtypes.DanmNet, rawConfig []byte, ep *danmtypes.DanmEp) (*current.Result,error) {
  cniPath, cniArgs, err := getExecCniParams(cniType, cniOpType, netInfo, ep)
  if err != nil {
    return nil, errors.New("exec CNI params couldn't be gathered:" + err.Error())
  }
  exec := invoke.RawExec{Stderr: os.Stderr}
  rawResult, err := exec.ExecPlugin(cniPath, rawConfig, cniArgs)
  if err != nil {
    return nil, errors.New("OS exec call failed:" + err.Error())
  }
  versionDecoder := &version.ConfigDecoder{}
  confVersion, err := versionDecoder.Decode(rawResult)
  if err != nil || rawResult == nil {
    return &current.Result{}, nil
  }
  convertedResult, err := version.NewResult(confVersion, rawResult)
  if err != nil || convertedResult == nil {
    return &current.Result{}, nil
  }
  finalResult := convertCniResult(convertedResult)
  return finalResult, nil
}

func getExecCniParams(cniType, cniOpType string, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) (string,[]string,error) {
  cniPaths := filepath.SplitList(os.Getenv("CNI_PATH"))
  cniPath, err := invoke.FindInPath(cniType, cniPaths)
  if err != nil {
    return "", nil, err
  }
  cniArgs := []string {
    "CNI_COMMAND="     + cniOpType,
    "CNI_CONTAINERID=" + os.Getenv("CNI_CONTAINERID"),
    "CNI_NETNS="       + os.Getenv("CNI_NETNS"),
    "CNI_IFNAME="      + ep.Spec.Iface.Name,
    "CNI_ARGS="        + os.Getenv("CNI_ARGS"),
    "CNI_PATH="        + os.Getenv("CNI_PATH"),
    "PATH="            + os.Getenv("PATH"),
  }
  return cniPath, cniArgs, nil
}

func setEpIfaceAddress(cniResult *current.Result, epIface *danmtypes.DanmEpIface) error {
  for _, ip := range cniResult.IPs {
    if ip.Version == "4" {
      epIface.Address = ip.Address.String()
    } else {
      epIface.AddressIPv6 = ip.Address.String()
    }
  }
  return nil
}

// DelegateInterfaceDelete delegates Ks8 Pod network interface delete task to the input 3rd party CNI plugin
// Returns an error if interface creation was unsuccessful, or if the 3rd party CNI config could not be loaded
func DelegateInterfaceDelete(netConf *datastructs.NetConf, danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) error {
  rawConfig, err := getCniPluginConfig(netConf, netInfo, datastructs.IpamConfig{Type: ipamType}, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return err
  }
  cniType := netInfo.Spec.NetworkType
  _, err = execCniPlugin(cniType, CniDelOp, netInfo, rawConfig, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return errors.New("Error delegating DEL to CNI plugin:" + cniType + " because:" + err.Error())
  }
  return freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
}

func freeDelegatedIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip4, ip6 string) error {
  err4 := freeDelegatedIp(danmClient, netInfo, ip4)
  err6 := freeDelegatedIp(danmClient, netInfo, ip6)
  if err4 != nil {
    return err4
  }
  return err6
}

func freeDelegatedIp(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip string) error {
  if netInfo.Spec.NetworkType == "flannel" && ip != ""{
    flannelIpExhaustionWorkaround(ip)
  }
  if isIpamNeeded(netInfo.Spec.NetworkType) && ip != "" {
    err := ipam.Free(danmClient, *netInfo, ip)
    if err != nil {
      return errors.New("cannot give back ip address for DanmNet:" + netInfo.ObjectMeta.Name + " addr:" + ip)
    }
  }
  return nil
}

// Host-local IPAM management plugin uses disk-local Store by default.
// Right now it is buggy in a sense that it does not try to free IPs if the container being deleted does not exist already.
// But it should!
// Exception handling 101 dear readers: ALWAYS try and reset your environment to the best of your ability during an exception
// TODO: remove this once the problem is solved upstream
func flannelIpExhaustionWorkaround(ip string) {
  var dataDir = filepath.Join(defaultDataDir, flannelBridge)
  os.Remove(filepath.Join(dataDir, ip))
}

// ConvertCniResult converts a CNI result from an older API version to the latest format
// Returns nil if conversion is unsuccessful
func convertCniResult(rawCniResult types.Result) *current.Result {
  convertedResult, err := current.NewResultFromResult(rawCniResult)
  if err != nil {
    log.Println("Delegated CNI result could not be converted:" + err.Error())
    return nil
  }
  return convertedResult
}

func GetEnv(key, fallback string) string {
  if value, doesExist := os.LookupEnv(key); doesExist {
    return value
  }
  return fallback
}

// CalculateIfaceName decides what should be the name of a container's interface.
// If a name is explicitly set in the related DanmNet API object, the NIC will be named accordingly.
// If a name is not explicitly set, then DANM names the interface ethX where X=sequence number of the interface
// When legacy naming scheme is configured container_prefix behaves as the exact name of an interface, rather than its name suggest
func CalculateIfaceName(namingScheme, chosenName, defaultName string, sequenceId int) string {
  //Kubelet expects the first interface to be literally named "eth0", so...
  if sequenceId == 0 {
    return "eth0"
  }
  if chosenName != "" {
    if namingScheme != LegacyNamingScheme {
      chosenName += strconv.Itoa(sequenceId)
    }
    return chosenName
  }
  return defaultName + strconv.Itoa(sequenceId)
}