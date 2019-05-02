package metacni

import (
  "errors"
  "fmt"
  "log"
  "net"
  "os"
  "runtime"
  "strings"
  "encoding/json"
  "github.com/satori/go.uuid"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/plugins/pkg/ns"
  "github.com/containernetworking/plugins/pkg/utils/sysctl"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  k8s "k8s.io/apimachinery/pkg/types"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/clientcmd"
  "k8s.io/client-go/kubernetes"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/cnidel"
  "github.com/nokia/danm/pkg/danmep"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/syncher"
  checkpoint_utils "github.com/intel/multus-cni/checkpoint"
)

const (
  danmApiPath = "danm.k8s.io"
  danmIfDefinitionSyntax = danmApiPath + "/interfaces"
  v1Endpoint = "/api/v1/"
  cniVersion = "0.3.1"
  defaultNetworkName = "default"
  defaultIfName = "eth"
  DefaultCniDir = "/etc/cni/net.d"
)

var (
  apiHost = os.Getenv("API_SERVERS")
  DanmConfig *datastructs.NetConf
)

// K8sArgs is the valid CNI_ARGS type used to parse K8s CNI event calls (thanks Multus)
type K8sArgs struct {
  types.CommonArgs
  IP                         net.IP
  K8S_POD_NAME               types.UnmarshallableString
  K8S_POD_NAMESPACE          types.UnmarshallableString
  K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
}

type cniArgs struct {
  nameSpace string
  netns string
  podId string
  containerId string
  annotation map[string]string
  labels map[string]string
  stdIn []byte
  interfaces []danmtypes.Interface
  podUid k8s.UID
}

func CreateInterfaces(args *skel.CmdArgs) error {
  cniArgs,err := extractCniArgs(args)
  if err != nil {
    log.Println("ERROR: ADD: CNI args cannot be loaded with error:" + err.Error())
    return fmt.Errorf("CNI args cannot be loaded with error: %v", err)
  }
  log.Println("CNI ADD invoked with: ns:" + cniArgs.nameSpace + " PID:" + cniArgs.podId + " CID: " + cniArgs.containerId)
  err = loadNetConf(cniArgs.stdIn)
  if err != nil {
    return errors.New("ERROR: ADD: cannot load DANM CNI config due to error:" + err.Error())
  }
  err = getPodAttributes(cniArgs)
  if err != nil {
    log.Println("ERROR: ADD: Pod manifest could not be parsed with error:" + err.Error())
    return fmt.Errorf("Pod manifest could not be parsed with error: %v", err)
  }
  extractConnections(cniArgs)
  if len(cniArgs.interfaces) == 1 && cniArgs.interfaces[0].Network == defaultNetworkName {
    log.Println("WARN: ADD: no network connections for Pod: " + cniArgs.podId + " are defined in spec.metadata.annotation. Falling back to use: " + defaultNetworkName)
  }
  cniResult, err := setupNetworking(cniArgs)
  if err != nil {
    //Best effort cleanup - not interested in possible errors, anyway could not do anything with them
    os.Setenv("CNI_COMMAND","DEL")
    DeleteInterfaces(args)
    log.Println("ERROR: ADD: CNI network could not be set up with error:" + err.Error())
    return fmt.Errorf("CNI network could not be set up: %v", err)
  }
  return types.PrintResult(cniResult, cniVersion)
}

func createDanmClient() (danmclientset.Interface,error) {
  config, err := getClientConfig()
  if err != nil {
    return nil, errors.New("Parsing kubeconfig failed with error:" + err.Error())
  }
  client, err := danmclientset.NewForConfig(config)
  if err != nil {
    return nil, errors.New("Creation of K8s Danm REST client failed with error:" + err.Error())
  }
  return client, nil
}

func getClientConfig() (*rest.Config, error){
  config, err := clientcmd.BuildConfigFromFlags("", DanmConfig.Kubeconfig)
  if err != nil {
    return nil, err
  }
  return config, nil
}

func loadNetConf(bytes []byte) error {
  netconf := &datastructs.NetConf{}
  err := json.Unmarshal(bytes, netconf)
  if err != nil {
    return errors.New("Failed to parse DANM's CNI config file:" + err.Error())
  }
  DanmConfig = netconf
  if DanmConfig.CniConfigDir == "" {
    DanmConfig.CniConfigDir = DefaultCniDir
  }
  return nil
}

func extractCniArgs(args *skel.CmdArgs) (*cniArgs,error) {
  kubeArgs := K8sArgs{}
  err := types.LoadArgs(args.Args, &kubeArgs)
  if err != nil {
    return nil,err
  }
  cmdArgs := cniArgs{string(kubeArgs.K8S_POD_NAMESPACE),
                     args.Netns,
                     string(kubeArgs.K8S_POD_NAME),
                     string(kubeArgs.K8S_POD_INFRA_CONTAINER_ID),
                     nil,
                     nil,
                     args.StdinData,
                     nil,
                     "",
                    }
  return &cmdArgs, nil
}

func getPodAttributes(args *cniArgs) error {
  k8sClient, err := createK8sClient(DanmConfig.Kubeconfig)
  if err != nil {
    return errors.New("cannot create K8s REST client due to error:" + err.Error())
  }
  pod, err := k8sClient.CoreV1().Pods(string(args.nameSpace)).Get(string(args.podId), meta_v1.GetOptions{})
  if err != nil {
    return errors.New("failed to get Pod info from K8s API server due to:" + err.Error())
  }
  args.annotation = pod.Annotations
  args.labels = pod.Labels
  args.podUid = pod.UID
  return nil
}

func createK8sClient(kubeconfig string) (kubernetes.Interface, error) {
  config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
  if err != nil {
    return nil, err
 }
 return kubernetes.NewForConfig(config)
}

func extractConnections(args *cniArgs) error {
  var ifaces []danmtypes.Interface
  for key, val := range args.annotation {
    if strings.Contains(key, danmIfDefinitionSyntax) {
      err := json.Unmarshal([]byte(val), &ifaces)
      if err != nil {
        return errors.New("Can't create network interfaces for Pod: " + args.podId + " due to badly formatted " + danmIfDefinitionSyntax + " definition in Pod annotation")
      }
      break
    }
  }
  if len(ifaces) == 0 {
    ifaces = []danmtypes.Interface{{Network: defaultNetworkName}}
  }
  args.interfaces = ifaces
  return nil
}

func getAllocatedDevices(args *cniArgs, checkpoint checkpoint_utils.Checkpoint, devicePool string)(*[]string, error){
  resourceMap, err := checkpoint.GetComputeDeviceMap(string(args.podUid))
  if err != nil || len(resourceMap) == 0 {
    return nil, errors.New("failed to retrieve Pod info from checkpoint object due to:" + err.Error())
  }
  if _, ok := resourceMap[devicePool]; !ok {
    return nil, errors.New("failed to retrieve resources of DevicePool")
  }
  return &resourceMap[devicePool].DeviceIDs, nil
}

func popDevice(devicePool string, allocatedDevices map[string]*[]string)(string, error) {
  if len(allocatedDevices) == 0 { return "", errors.New("allocatedDevices is empty") }
  devices := (*allocatedDevices[devicePool])
  if len(devices) == 0 { return "", errors.New("devicePool is empty") }
  device, devices := devices[len(devices)-1], devices[:len(devices)-1]
  allocatedDevices[devicePool] = &devices
  return device, nil
}

func preparePodForIpv6(args *cniArgs) error {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  // save the current namespace
  origNs, err := ns.GetCurrentNS()
  if err != nil {
    return errors.New("failed to get current network namespace due to:" + err.Error())
  }
  // enter to the Pod's network namespace
  podNs, err := ns.GetNS(args.netns)
  if err != nil {
    return errors.New("failed to get Pod's network namespace due to:" + err.Error())
  }
  err = podNs.Set()
  if err != nil {
    return errors.New("failed to switch to Pod's network namespace due to:" + err.Error())
  }
  defer func() {
    podNs.Close()
    origNs.Set()
  }()
  // set sysctl
  _, err = sysctl.Sysctl("net.ipv6.conf.all.disable_ipv6", "0")
  if err != nil {
    return errors.New("failed to set sysctl due to:" + err.Error())
  }
  return nil
}

func setupNetworking(args *cniArgs) (*current.Result, error) {
  danmClient, err := createDanmClient()
  if err != nil {
    return nil, errors.New("failed to create DanmClient due to:" + err.Error())
  }
  err = preparePodForIpv6(args)
  if err != nil {
    return nil, errors.New("failed to prepare Pod for IPv6 due to:" + err.Error())
  }
  syncher := syncher.NewSyncher(len(args.interfaces))
  allocatedDevices := make(map[string]*[]string)
  checkpoint, err := checkpoint_utils.GetCheckpoint()
  if err != nil {
    return nil, errors.New("failed to instantiate checkpoint object due to:" + err.Error())
  }
  var cniRes *current.Result
  for nicID, nicParams := range args.interfaces {
    isDelegationRequired, netInfo, err := cnidel.IsDelegationRequired(danmClient, nicParams.Network, args.nameSpace)
    if err != nil {
      return cniRes, errors.New("failed to get DanmNet due to:" + err.Error())
    }
    nicParams.SequenceId = nicID
    nicParams.DefaultIfaceName = defaultIfName
    if isDelegationRequired {
      if cnidel.IsDeviceNeeded(netInfo.Spec.NetworkType) {
        if _, ok := allocatedDevices[netInfo.Spec.Options.DevicePool]; !ok {
          allocatedDevices[netInfo.Spec.Options.DevicePool], err = getAllocatedDevices(args, checkpoint, netInfo.Spec.Options.DevicePool)
          if err != nil {
            return cniRes, errors.New("failed to get allocated devices due to:" + err.Error())
          }
        }
        nicParams.Device, err = popDevice(netInfo.Spec.Options.DevicePool, allocatedDevices)
        if err != nil {
          return cniRes, errors.New("failed to pop devices due to:" + err.Error())
        }
      }
      go createDelegatedInterface(syncher, danmClient, nicParams, netInfo, args)
    } else {
      go createDanmInterface(syncher, danmClient, nicParams, netInfo, args)
    }
  }
  err = syncher.GetAggregatedResult()
  return syncher.MergeCniResults(), err
}

func createDelegatedInterface(syncher *syncher.Syncher, danmClient danmclientset.Interface, iface danmtypes.Interface, netInfo *danmtypes.DanmNet, args *cniArgs) {
  epIfaceSpec := danmtypes.DanmEpIface {
    Name:        cnidel.CalculateIfaceName(DanmConfig.NamingScheme, netInfo.Spec.Options.Prefix, iface.DefaultIfaceName, iface.SequenceId),
    Address:     iface.Ip,
    AddressIPv6: iface.Ip6,
    Proutes:     iface.Proutes,
    Proutes6:    iface.Proutes6,
    DeviceID:    iface.Device,
  }
  ep, err := createDanmEp(epIfaceSpec, netInfo, args)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("DanmEp object could not be created due to error:" + err.Error()), nil)
    return
  }
  delegatedResult,err := cnidel.DelegateInterfaceSetup(DanmConfig, danmClient, netInfo, &ep)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("CNI delegation failed due to error:" + err.Error()), nil)
    return
  }
  err = putDanmEp(ep)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("DanmEp object could not be PUT to K8s due to error:" + err.Error()), nil)
    return
  }
  err = danmep.SetDanmEpSysctls(ep)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("Sysctls could not be set due to error:" + err.Error()), nil)
    return
  }
  syncher.PushResult(netInfo.ObjectMeta.Name, nil, delegatedResult)
}

func createDanmInterface(syncher *syncher.Syncher, danmClient danmclientset.Interface, iface danmtypes.Interface, netInfo *danmtypes.DanmNet, args *cniArgs) {
  ip4, ip6, macAddr, err := ipam.Reserve(danmClient, *netInfo, iface.Ip, iface.Ip6)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("IP address reservation failed for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error()), nil)
    return
  }
  epSpec := danmtypes.DanmEpIface {
    Name: cnidel.CalculateIfaceName(DanmConfig.NamingScheme, netInfo.Spec.Options.Prefix, iface.DefaultIfaceName, iface.SequenceId),
    Address: ip4,
    AddressIPv6: ip6,
    MacAddress: macAddr,
    Proutes: iface.Proutes,
    Proutes6: iface.Proutes6,
  }
  ep, err := createDanmEp(epSpec, netInfo, args)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("DanmEp object could not be created due to error:" + err.Error()), nil)
    return
  }
  err = putDanmEp(ep)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("EP could not be PUT into K8s due to error:" + err.Error()), nil)
    return
  } 
  err = danmep.AddIpvlanInterface(netInfo, ep)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    deleteEp(danmClient, ep)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("IPVLAN interface could not be created due to error:" + err.Error()), nil)
    return
  }
  err = danmep.SetDanmEpSysctls(ep)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    deleteEp(danmClient, ep)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("Sysctls could not be set due to error:" + err.Error()), nil)
    return
  }
  danmResult := &current.Result{}
  AddIfaceToResult(ep.Spec.EndpointID, epSpec.MacAddress, args.containerId, danmResult)
  if (ip4 != "") {
    AddIpToResult(ip4,"4",danmResult)
  }
  if (ip6 != "") {
    AddIpToResult(ip6,"6",danmResult)
  }
  syncher.PushResult(netInfo.ObjectMeta.Name, nil, danmResult)
}

func createDanmEp(epInput danmtypes.DanmEpIface, netInfo *danmtypes.DanmNet, args *cniArgs) (danmtypes.DanmEp, error) {
  epidInt, err := uuid.NewV4()
  if err != nil {
    return danmtypes.DanmEp{}, errors.New("uuid.NewV4 returned error during EP creation:" + err.Error())
  }
  epid := epidInt.String()
  host, err := os.Hostname()
  if err != nil {
    return danmtypes.DanmEp{}, errors.New("OS.Hostname returned error during EP creation:" + err.Error())
  }
  if netInfo.Spec.NetworkType == "" {
    netInfo.Spec.NetworkType = "ipvlan" 
  }
  epSpec := danmtypes.DanmEpSpec {
    NetworkName: netInfo.ObjectMeta.Name,
    NetworkType: netInfo.Spec.NetworkType,
    EndpointID: epid,
    Iface: epInput,
    Host: host,
    Pod: args.podId,
    CID: args.containerId,
    Netns: args.netns,
  }
  meta := meta_v1.ObjectMeta {
    Name: epid,
    Namespace: args.nameSpace,
    ResourceVersion: "",
    Labels: args.labels,
  }
  typeMeta := meta_v1.TypeMeta {
      APIVersion: danmtypes.SchemeGroupVersion.String(),
      Kind: "DanmEp",
  }
  ep := danmtypes.DanmEp{
    TypeMeta: typeMeta,
    ObjectMeta: meta,
    Spec: epSpec,
  }
  return ep, nil
}

func putDanmEp(ep danmtypes.DanmEp) error {
  danmClient, err := createDanmClient()
  if err != nil {
    return err
  }
  _, err = danmClient.DanmV1().DanmEps(ep.Namespace).Create(&ep)
  if err != nil {
    return err
  }
  return nil
}

func AddIfaceToResult(epid string, macAddress string, sandBox string, cniResult *current.Result) {
  iface := &current.Interface{
    Name: epid,
    Mac: macAddress,
    Sandbox: sandBox,
  }
  cniResult.Interfaces = append(cniResult.Interfaces, iface)
}

func AddIpToResult(ip string, version string, cniResult *current.Result) {
  if ip != "" {
    ip, _ := types.ParseCIDR(ip)
    ipConf := &current.IPConfig {
      Version: version,
      Address: *ip,
    }
    cniResult.IPs = append(cniResult.IPs, ipConf)
  }
}

func DeleteInterfaces(args *skel.CmdArgs) error {
  cniArgs,err := extractCniArgs(args)
  log.Println("CNI DEL invoked with: ns:" + cniArgs.nameSpace + " PID:" + cniArgs.podId + " CID: " + cniArgs.containerId)
  if err != nil {
    log.Println("INFO: DEL: CNI args could not be loaded because" + err.Error())
    return nil
  }
  err = loadNetConf(cniArgs.stdIn)
  if err != nil {
    log.Println("INFO: DEL: cannot load DANM CNI config due to error:" + err.Error())
    return nil
  }
  danmClient, err := createDanmClient()
  if err != nil {
    log.Println("INFO: DEL: DanmEp REST client could not be created because" + err.Error())
    return nil
  }
  eplist, err := danmep.FindByCid(danmClient, cniArgs.containerId)
  if err != nil {
    log.Println("INFO: DEL: Could not interrogate DanmEps from K8s API server because" + err.Error())
    return nil
  }
  syncher := syncher.NewSyncher(len(eplist))
  for _, ep := range eplist {
    go deleteInterface(danmClient, cniArgs, syncher, ep)
  }
  deleteErrors := syncher.GetAggregatedResult()
  if deleteErrors != nil {
    log.Println("INFO: DEL: Following errors happened during interface deletion:" + deleteErrors.Error())
  }
  return nil
}

func deleteInterface(danmClient danmclientset.Interface, args *cniArgs, syncher *syncher.Syncher, ep danmtypes.DanmEp) {
  netInfo, err := danmClient.DanmV1().DanmNets(args.nameSpace).Get(ep.Spec.NetworkName, meta_v1.GetOptions{})
  if err != nil {
    syncher.PushResult(ep.Spec.NetworkName, errors.New("failed to get DanmNet:"+ err.Error()), nil)
    return
  }
  var aggregatedError string
  err = deleteNic(danmClient, netInfo, ep)
  //It can happen that a container was already destroyed at this point in this fully asynch world
  //So we are not interested in errors, but we also can't just return yet, we need to try and clean-up remaining resources, if, any
  if err != nil {
    aggregatedError += "failed to delete container NIC:" + err.Error() + "; "
  }
  err = deleteEp(danmClient, ep)
  if err != nil {
    aggregatedError += "failed to delete DanmEp:" + err.Error() + "; "
  }
  if aggregatedError != "" {
    syncher.PushResult(ep.Spec.NetworkName, errors.New(aggregatedError), nil)
  } else {
    syncher.PushResult(ep.Spec.NetworkName, nil, nil)
  }
}

func deleteNic(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ep danmtypes.DanmEp) error {
  var err error
  if ep.Spec.NetworkType != "ipvlan" {
    err = cnidel.DelegateInterfaceDelete(DanmConfig, danmClient, netInfo, &ep)
  } else {
    err = deleteDanmNet(danmClient, ep, netInfo)
  }
  return err
}

func deleteEp(danmClient danmclientset.Interface, ep danmtypes.DanmEp) error {
  delOpts := meta_v1.DeleteOptions{}
  err := danmClient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &delOpts)
  if err != nil {
    return err
  }
  return nil
}

func deleteDanmNet(danmClient danmclientset.Interface, ep danmtypes.DanmEp, netInfo *danmtypes.DanmNet) error {
  ipam.GarbageCollectIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
  return danmep.DeleteIpvlanInterface(ep)
}
