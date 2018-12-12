package cnidel

import (  
  "errors"
  "log"
  "net"
  "os"
  "path/filepath"
  "strings"
  "encoding/json"
  "io/ioutil"
  "github.com/containernetworking/cni/pkg/invoke"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/ipam"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/pkg/crd/client/clientset/versioned"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
  ipamType = "fakeipam"
  defaultDataDir = "/var/lib/cni/networks"
  flannelBridge = getEnv("FLANNEL_BRIDGE", "cbr0")
)

type cniBackendConfig struct {
  danmtypes.CniBackend
  readConfig cniConfigReader
  ipamNeeded bool
}

type cniConfigReader func(netInfo *danmtypes.DanmNet, ipam danmtypes.IpamConfig, nicParams danmtypes.Interface) ([]byte, error)

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
  // IPAM configuration to be used for this network.
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

var (
  dpdkNicDriver = os.Getenv("DPDK_NIC_DRIVER")
  dpdkDriver = os.Getenv("DPDK_DRIVER")
  dpdkTool = os.Getenv("DPDK_TOOL")
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
  }
)

// IsDelegationRequired decides if the interface creation operations should be delegated to a 3rd party CNI, or can be handled by DANM
// Decision is made based on the NetworkType parameter of the DanmNet object
func IsDelegationRequired(danmClient danmclientset.Interface, nid, namespace string) (bool,*danmtypes.DanmNet,error) {
  netInfo, err := danmClient.DanmV1().DanmNets(namespace).Get(nid, meta_v1.GetOptions{})
  if err != nil {
    return false, nil, err
  }
  neType := netInfo.Spec.NetworkType
  if neType == "ipvlan" || neType == "" {
    return false, netInfo, nil
  }
  return true, netInfo, nil
}

// DelegateInterfaceSetup delegates Ks8 Pod network interface setup task to the input 3rd party CNI plugin
// Returns the CNI compatible result object, or an error if interface creation was unsuccessful, or if the 3rd party CNI config could not be loaded
func DelegateInterfaceSetup(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, iface danmtypes.Interface) (types.Result,error) {
  var (
    ip4 string
    ip6 string
    err error
    ipamOptions danmtypes.IpamConfig
  )
  if isIpamNeeded(netInfo.Spec.NetworkType) {
    ip4, ip6, _, err = ipam.Reserve(danmClient, *netInfo, iface.Ip, iface.Ip6)
    if err != nil {
      return nil, errors.New("IP address reservation failed for network:" + netInfo.Spec.NetworkID + " with error:" + err.Error())
    }
    ipamOptions = getCniIpamConfig(netInfo.Spec.Options, ip4, ip6)
  }
  rawConfig, err := getCniPluginConfig(netInfo, ipamOptions, iface)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ip4, ip6)
    return nil, err
  }
  cniType := netInfo.Spec.NetworkType
  cniResult,err := execCniPlugin(cniType, netInfo, iface, rawConfig)
  if err != nil {
    freeDelegatedIps(danmClient, netInfo, ip4, ip6)
    return nil, errors.New("Error delegating ADD to CNI plugin:" + cniType + " because:" + err.Error())
  }
  return cniResult, nil
}

func isIpamNeeded(cniType string) bool {
  for _, cni := range supportedNativeCnis {
    if cni.BackendName == cniType {
      return cni.ipamNeeded
    }
  }
  return false
}

func getCniIpamConfig(options danmtypes.DanmNetOption, ip4 string, ip6 string) danmtypes.IpamConfig {
  var (
    subnet string
    routes []danmtypes.IpamRoute
    defaultGw string
    ip string
  )
  if options.Cidr != "" {
    ip = ip4
    subnet = options.Cidr
    routes, defaultGw = parseRoutes(options.Routes, subnet)
  } else {
    ip = ip6
    subnet = options.Net6
    routes, defaultGw = parseRoutes(options.Routes6, subnet)
  }
  return danmtypes.IpamConfig {
    Type: ipamType,
    Subnet: subnet,
    Routes: routes,
    DefaultGw: defaultGw,
    Ip: strings.Split(ip, "/")[0],
  }
}

func getCniPluginConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, nicParams danmtypes.Interface) ([]byte, error) {
  cniType := netInfo.Spec.NetworkType
  for _, cni := range supportedNativeCnis {
    if cni.BackendName == cniType {
      return cni.readConfig(netInfo, ipamOptions, nicParams)
    }
  }
  return readCniConfigFile(netInfo)
}

func execCniPlugin(cniType string, netInfo *danmtypes.DanmNet, nicParams danmtypes.Interface, rawConfig []byte) (types.Result,error) {
  cniPath, cniArgs, err := getExecCniParams(cniType, netInfo, nicParams)
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

func getExecCniParams(cniType string, netInfo *danmtypes.DanmNet, nicParams danmtypes.Interface) (string,[]string,error) {
  cniPaths := filepath.SplitList(os.Getenv("CNI_PATH"))
  cniPath, err := invoke.FindInPath(cniType, cniPaths)
  if err != nil {
    return "", nil, err
  }
  cniArgs := []string{
    "CNI_COMMAND="     + os.Getenv("CNI_COMMAND"),
    "CNI_CONTAINERID=" + os.Getenv("CNI_CONTAINERID"),
    "CNI_NETNS="       + os.Getenv("CNI_NETNS"),
    "CNI_IFNAME="      + CalculateIfaceName(netInfo.Spec.Options.Prefix, nicParams.DefaultIfaceName),
    "CNI_PATH="        + os.Getenv("CNI_PATH"),
  }
  return cniPath, cniArgs, nil
}

func getSriovCniConfig(netInfo *danmtypes.DanmNet, ipamOptions danmtypes.IpamConfig, nicParams danmtypes.Interface) ([]byte, error) {
  vlanid := netInfo.Spec.Options.Vlan
  sriovConfig := sriovNet {
    Name:   netInfo.Spec.NetworkID,
    Type:   "sriov",
    PfName: netInfo.Spec.Options.Device,
    IfName: netInfo.Spec.Options.Prefix,
    L2Mode: true,
    Vlan:   vlanid,
    Dpdk:   DpdkOption{},
    Ipam:   ipamOptions,
  }
  sriovConfig.IfName = CalculateIfaceName(netInfo.Spec.Options.Prefix, nicParams.DefaultIfaceName)
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
    return nil, errors.New("Error getting sriov plugin config: " + err.Error())
  }
  return rawConfig, nil
}

func readCniConfigFile(netInfo *danmtypes.DanmNet) ([]byte, error) {
  cniType := netInfo.Spec.NetworkType
  //TODO: the path from where the config is read should not be hard-coded
  rawConfig, err := ioutil.ReadFile("/etc/cni/net.d/" + cniType + ".conf")
  if err != nil {
    return nil, errors.New("Could not load CNI config file for plugin:" + cniType)
  }
  return rawConfig, nil
}

func parseRoutes(rawRoutes map[string]string, netCidr string) ([]danmtypes.IpamRoute, string) {
  defaultGw := ""
  routes := []danmtypes.IpamRoute{}
  for dst, gw := range rawRoutes {
    routes = append(routes, danmtypes.IpamRoute{
      Dst: dst,
      Gw: gw,
    })
    if _, sn, _ := net.ParseCIDR(dst); sn.String() == netCidr {
      defaultGw = gw
    }
  }
  return routes, defaultGw
}

// DelegateInterfaceDelete delegates Ks8 Pod network interface delete task to the input 3rd party CNI plugin
// Returns an error if interface creation was unsuccessful, or if the 3rd party CNI config could not be loaded
func DelegateInterfaceDelete(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip string) error {
  rawConfig, err := getCniPluginConfig(netInfo, danmtypes.IpamConfig{}, danmtypes.Interface{})
  if err != nil {
    return err
  }
  cniType := netInfo.Spec.NetworkType
  err = invoke.DelegateDel(cniType, rawConfig)
  if err != nil {
    //Best-effort clean-up because we know how to handle exceptions
    freeDelegatedIp(danmClient, netInfo, ip)
    return errors.New("Error delegating DEL to CNI plugin:" + cniType + " because:" + err.Error())
  }
  return freeDelegatedIp(danmClient, netInfo, ip)
}

func freeDelegatedIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip4, ip6 string) error {
  err := freeDelegatedIp(danmClient, netInfo, ip4)
  err = freeDelegatedIp(danmClient, netInfo, ip6)
  return err
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
    return chosenName
  }
  return defaultName 
}