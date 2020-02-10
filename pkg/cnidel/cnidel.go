package cnidel

import (
  "context"
  "errors"
  "log"
  "net"
  "os"
  "strconv"
  "strings"
  "path/filepath"
  "github.com/containernetworking/cni/pkg/invoke"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/netcontrol"
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
  if isIpamNeeded(netInfo, ep) {
    ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6, err = ipam.Reserve(danmClient, *netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
      return nil, errors.New("IP address reservation failed for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
    }
    //As netInfo is only copied to IPAM above, the IP allocation is not refreshed in the original copy.
    //Without re-reading the network body we risk leaking IPs if error happens later on within the same thread!
    netInfo,err = netcontrol.GetNetworkFromEp(danmClient, *ep)
    if err != nil {
      return nil, err
    }
    ipamOptions = getCniIpamConfig(netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
  }
  rawConfig, err := getCniPluginConfig(netConf, netInfo, ipamOptions, ep)
  if err != nil {
    return nil, err
  }
  cniType := netInfo.Spec.NetworkType
  cniResult,err := execCniPlugin(cniType, CniAddOp, netInfo, rawConfig, ep)
  if err != nil {
    return nil, errors.New("Error delegating ADD to CNI plugin:" + cniType + " because:" + err.Error())
  }
  if cniResult != nil {
    setEpIfaceAddress(cniResult, &ep.Spec.Iface)
  }
  return cniResult, nil
}

func isIpamNeeded(netInfo *danmtypes.DanmNet, ep *danmtypes.DanmEp) bool {
  if cni, ok := SupportedNativeCnis[strings.ToLower(netInfo.Spec.NetworkType)]; ok {
    return cni.ipamNeeded
  }
  //For static delegates we should only overwrite the original IPAM if an IP was explicitly "requested" from the Pod, and the request "makes sense"
  //Requested includes "none" allocation scheme as well, which can happen for L2 networks too
  //When a real IP is asked from DANM it only makes sense to overwrite if there is really a CIDR to allocate it from
  //BEWARE, because once DANM takes over IP allocation, it takes over for both IPv4, and IPv6!
  //TODO: Can we have partial IPAM allocations? Is chaining IPAM CNIs allowed, e.g. IPv6 from DANM, IPv4 from host-ipam etc.? 
  if ep.Spec.Iface.Address     == ipam.NoneAllocType ||
     ep.Spec.Iface.AddressIPv6 == ipam.NoneAllocType ||
     (ep.Spec.Iface.Address     != "" && ep.Spec.Iface.Address     != ipam.NoneAllocType && netInfo.Spec.Options.Cidr != "") ||
     (ep.Spec.Iface.AddressIPv6 != "" && ep.Spec.Iface.AddressIPv6 != ipam.NoneAllocType && netInfo.Spec.Options.Net6 != "") {
    return true
  }
  return false
}

func IsDeviceNeeded(cniType string) bool {
  if cni, ok := SupportedNativeCnis[strings.ToLower(cniType)]; ok {
    return cni.deviceNeeded
  } else {
    return false
  }
}

func getCniIpamConfig(netinfo *danmtypes.DanmNet, ip4, ip6 string) datastructs.IpamConfig {
  var ipSlice = []datastructs.IpamIp{}
  if ip4 != "" && ip4 != ipam.NoneAllocType {
    ipSlice = append(ipSlice, datastructs.IpamIp{
                                IpCidr: ip4,
                                Version: 4,
                              })
  }
  if ip6 != "" && ip6 != ipam.NoneAllocType {
    ipSlice = append(ipSlice, datastructs.IpamIp{
                                IpCidr: ip6,
                                Version: 6,
                              })
  }
  return  datastructs.IpamConfig {
            Type: ipamType,
            Ips: ipSlice,
          }
}

func getCniPluginConfig(netConf *datastructs.NetConf, netInfo *danmtypes.DanmNet, ipamOptions datastructs.IpamConfig, ep *danmtypes.DanmEp) ([]byte, error) {
  if cni, ok := SupportedNativeCnis[strings.ToLower(netInfo.Spec.NetworkType)]; ok {
    return cni.readConfig(netInfo, ipamOptions, ep, cni.CNIVersion)
  } else {
    return readCniConfigFile(netConf.CniConfigDir, netInfo, ipamOptions)
  }
}

func execCniPlugin(cniType, cniOpType string, netInfo *danmtypes.DanmNet, rawConfig []byte, ep *danmtypes.DanmEp) (*current.Result,error) {
  cniPath, cniArgs, err := getExecCniParams(cniType, cniOpType, netInfo, ep)
  if err != nil {
    return nil, errors.New("exec CNI params couldn't be gathered:" + err.Error())
  }
  exec := invoke.RawExec{Stderr: os.Stderr}
  rawResult, err := exec.ExecPlugin(context.Background(), cniPath, rawConfig, cniArgs)
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
  var ip4, ip6 string
  if wasIpAllocatedByDanm(ep.Spec.Iface.Address, netInfo.Spec.Options.Cidr) {
    ip4 = ep.Spec.Iface.Address
  }
  if wasIpAllocatedByDanm(ep.Spec.Iface.AddressIPv6, netInfo.Spec.Options.Net6) {
    ip6 = ep.Spec.Iface.AddressIPv6
  }
  ipamForDelete := getCniIpamConfig(netInfo, ip4, ip6)
  rawConfig, err := getCniPluginConfig(netConf, netInfo, ipamForDelete, ep)
  if err != nil {
    FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return err
  }
  cniType := netInfo.Spec.NetworkType
  _, err = execCniPlugin(cniType, CniDelOp, netInfo, rawConfig, ep)
  if err != nil {
    FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    return errors.New("Error delegating DEL to CNI plugin:" + cniType + " because:" + err.Error())
  }
  return FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
}

func FreeDelegatedIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip4, ip6 string) error {
  err4 := freeDelegatedIp(danmClient, netInfo, ip4)
  err6 := freeDelegatedIp(danmClient, netInfo, ip6)
  if err4 != nil {
    return err4
  }
  return err6
}

func freeDelegatedIp(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip string) error {
  if netInfo.Spec.NetworkType == "flannel" && ip != "" && ip != ipam.NoneAllocType {
    flannelIpExhaustionWorkaround(ip)
  }
  //We only need to Free an IP if it was allocated by DANM IPAM, and it was allocated by DANM only if it falls into any of the defined subnets
  if wasIpAllocatedByDanm(ip, netInfo.Spec.Options.Cidr) || wasIpAllocatedByDanm(ip, netInfo.Spec.Options.Net6) {
    err := ipam.Free(danmClient, *netInfo, ip)
    if err != nil {
      return errors.New("cannot give back ip address: " + ip + " for network:" + netInfo.ObjectMeta.Name +
                        " of type: " + netInfo.TypeMeta.Kind + " because:" + err.Error())
    }
  }
  return nil
}

func wasIpAllocatedByDanm(ip, cidr string) bool {
  _, subnet, _ := net.ParseCIDR(cidr)
  parsedIp := net.ParseIP(ip)
  if parsedIp == nil {
    parsedIp,_,_ = net.ParseCIDR(ip)
  }
  if parsedIp != nil && (subnet != nil && subnet.Contains(parsedIp)) {
    return true
  }
  return false
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
// If a name is explicitly set in the related network API object, the NIC will be named accordingly.
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