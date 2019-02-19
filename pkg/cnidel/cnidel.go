package cnidel

import (  
  "errors"
  "log"
  "os"
  "path/filepath"
  "strings"
  "github.com/containernetworking/cni/pkg/invoke"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/danmep"
  "github.com/nokia/danm/pkg/ipam"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/pkg/crd/client/clientset/versioned"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
  ipamType = "fakeipam"
  defaultDataDir = "/var/lib/cni/networks"
  flannelBridge = getEnv("FLANNEL_BRIDGE", "cbr0")
  dpdkNicDriver = os.Getenv("DPDK_NIC_DRIVER")
  dpdkDriver = os.Getenv("DPDK_DRIVER")
  dpdkTool = os.Getenv("DPDK_TOOL")
)

// IsDelegationRequired decides if the interface creation operations should be delegated to a 3rd party CNI, or can be handled by DANM
// Decision is made based on the NetworkType parameter of the DanmNet object
func IsDelegationRequired(danmClient danmclientset.Interface, nid, namespace string) (bool,*danmtypes.DanmNet,error) {
  netInfo, err := danmClient.DanmV1().DanmNets(namespace).Get(nid, meta_v1.GetOptions{})
  if err != nil || netInfo.ObjectMeta.Name == ""{
    return false, nil, errors.New("NID:" + nid + " in namespace:" + namespace + " cannot be GET from K8s API server!")
  }
  neType := netInfo.Spec.NetworkType
  if neType == "ipvlan" || neType == "" {
    return false, netInfo, nil
  }
  return true, netInfo, nil
}

// DelegateInterfaceSetup delegates K8s Pod network interface setup task to the input 3rd party CNI plugin
// Returns the CNI compatible result object, or an error if interface creation was unsuccessful, or if the 3rd party CNI config could not be loaded
func DelegateInterfaceSetup(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) (*current.Result,error) {
  var (
    err error
    ipamOptions danmtypes.IpamConfig
  )
  if isIpamNeeded(netInfo.Spec.NetworkType) {
   ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6, _, err = ipam.Reserve(danmClient, *netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
      return nil, errors.New("IP address reservation failed for network:" + netInfo.Spec.NetworkID + " with error:" + err.Error())
    }
    ipamOptions = getCniIpamConfig(netInfo.Spec.Options, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
  }
  rawConfig, err := getCniPluginConfig(netInfo, ipamOptions, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return nil, err
  }
  cniType := netInfo.Spec.NetworkType
  cniResult,err := execCniPlugin(cniType, netInfo, rawConfig, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return nil, errors.New("Error delegating ADD to CNI plugin:" + cniType + " because:" + err.Error())
  }
  delegatedResult := ConvertCniResult(cniResult)
  if delegatedResult != nil {
    setEpIfaceAddress(delegatedResult, &ep.Spec.Iface)
  }
  err = danmep.CreateRoutesInNetNs(*ep, netInfo)
  if err != nil {
    // We don't consider this serious error, so we only log a warning about the issue.
    log.Println("WARNING: Could not create IP routes for CNI:" + cniType + " because:" + err.Error())
  }
  return delegatedResult, nil
}

func isIpamNeeded(cniType string) bool {
  for _, cni := range supportedNativeCnis {
    if cni.BackendName == cniType {
      return cni.ipamNeeded
    }
  }
  return false
}

func getCniIpamConfig(options danmtypes.DanmNetOption, ip4, ip6 string) danmtypes.IpamConfig {
  var (
    subnet string
    ip string
  )
  if options.Cidr != "" {
    ip = ip4
    subnet = options.Cidr
  } else {
    ip = ip6
    subnet = options.Net6
  }
  return danmtypes.IpamConfig {
    Type: ipamType,
    Subnet: subnet,
    Ip: strings.Split(ip, "/")[0],
  }
}

func getCniPluginConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  cniType := netInfo.Spec.NetworkType
  for _, cni := range supportedNativeCnis {
    if cni.BackendName == cniType {
      return cni.readConfig(netInfo, ipamOptions, ep)
    }
  }
  return readCniConfigFile(netInfo)
}

func execCniPlugin(cniType string, netInfo *danmtypes.DanmNet, rawConfig []byte, ep *danmtypes.DanmEp) (types.Result,error) {
  cniPath, cniArgs, err := getExecCniParams(cniType, netInfo, ep)
  if err != nil {
    return nil, err
  }
  exec := invoke.RawExec{Stderr: os.Stderr}
  rawResult, err := exec.ExecPlugin(cniPath, rawConfig, cniArgs)
  if err != nil {
    return nil, err
  }
  versionDecoder := &version.ConfigDecoder{}
  confVersion, err := versionDecoder.Decode(rawConfig)
  if err != nil {
    return nil, err
  }
  return version.NewResult(confVersion, rawResult)
}

func getExecCniParams(cniType string, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) (string,[]string,error) {
  cniPaths := filepath.SplitList(os.Getenv("CNI_PATH"))
  cniPath, err := invoke.FindInPath(cniType, cniPaths)
  if err != nil {
    return "", nil, err
  }
  cniArgs := []string{
    "CNI_COMMAND="     + os.Getenv("CNI_COMMAND"),
    "CNI_CONTAINERID=" + os.Getenv("CNI_CONTAINERID"),
    "CNI_NETNS="       + os.Getenv("CNI_NETNS"),
    "CNI_IFNAME="      + ep.Spec.Iface.Name,
    "CNI_ARGS="        + os.Getenv("CNI_ARGS"),
    "CNI_PATH="        + os.Getenv("CNI_PATH"),
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
func DelegateInterfaceDelete(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) error {
  rawConfig, err := getCniPluginConfig(netInfo, danmtypes.IpamConfig{Type: ipamType}, ep)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return err
  }
  cniType := netInfo.Spec.NetworkType
  err = invoke.DelegateDel(cniType, rawConfig)
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
      return errors.New("cannot give back ip address for NID:" + netInfo.Spec.NetworkID + " addr:" + ip)
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
func ConvertCniResult(rawCniResult types.Result) *current.Result {
  convertedResult, err := current.NewResultFromResult(rawCniResult)
  if err != nil {
    log.Println("Delegated CNI result could not be converted:" + err.Error())
    return nil
  }
  return convertedResult
}

func getEnv(key, fallback string) string {
  if value, doesExist := os.LookupEnv(key); doesExist {
    return value
  }
  return fallback
}

// CalculateIfaceName decides what should be the name of a container's interface.
// If a name is explicitly set in the related DanmNet API object, the NIC will be named accordingly.
// If a name is not explicitly set, then DANM will name the interface ethX where X=sequence number of the interface
func CalculateIfaceName(chosenName, defaultName string) string {
  if chosenName != "" {
    // TODO: Interface name is not unique when POD requests multiple interfaces from the same DanmNet (eg. SR IOV) // petszila
    return chosenName
  }
  return defaultName 
}