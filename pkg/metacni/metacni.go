package metacni

import (
  "bytes"
  "errors"
  "fmt"
  "log"
  "net"
  "os"
  "runtime"
  "strconv"
  "strings"
  "encoding/json"
  "github.com/satori/go.uuid"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/plugins/pkg/ns"
  "github.com/containernetworking/plugins/pkg/utils/sysctl"
  core_v1 "k8s.io/api/core/v1"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/clientcmd"
  "k8s.io/client-go/kubernetes"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/cnidel"
  "github.com/nokia/danm/pkg/danmep"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/netcontrol"
  "github.com/nokia/danm/pkg/syncher"
  checkpoint_utils "github.com/intel/multus-cni/checkpoint"
  multus_types "github.com/intel/multus-cni/types"
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
  stdIn []byte
  interfaces []datastructs.Interface
  pod *core_v1.Pod
  defaultNetwork *danmtypes.DanmNet
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
  err = getPod(cniArgs)
  if err != nil {
    log.Println("ERROR: ADD: Pod manifest could not be parsed with error:" + err.Error())
    return fmt.Errorf("Pod manifest could not be parsed with error: %v", err)
  }
  err = extractConnections(cniArgs)
  if err != nil {
    log.Println("ERROR: ADD: DANM annotation cannot be parsed:" + err.Error())
    return fmt.Errorf("DANM annotation cannot be parsed: %v", err)
  }
  if len(cniArgs.interfaces) == 0 {
    danmClient, err := CreateDanmClient(DanmConfig.Kubeconfig)
    if err != nil {
      log.Println("ERROR: cannot instantiate K8s client, because:" + err.Error())
      return fmt.Errorf("ERROR: cannot instantiate K8s client: %v", err)
    }
    defaultNet, err := netcontrol.GetDefaultNetwork(danmClient, defaultNetworkName, cniArgs.pod.ObjectMeta.Namespace)
    if err != nil {
      log.Println("ERROR: there are no network connections defined for Pod:" + cniArgs.pod.ObjectMeta.Name + ", and there is no suitable default network configured in the cluster!")
      return errors.New("there are no network connections defined, and there is no suitable default network configured in the cluster")
    }
    cniArgs.defaultNetwork = defaultNet
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

func CreateDanmClient(kubeConfig string) (danmclientset.Interface,error) {
  config, err := getClientConfig(kubeConfig)
  if err != nil {
    return nil, errors.New("Parsing kubeconfig failed with error:" + err.Error())
  }
  client, err := danmclientset.NewForConfig(config)
  if err != nil {
    return nil, errors.New("Creation of K8s Danm REST client failed with error:" + err.Error())
  }
  return client, nil
}

func getClientConfig(kubeConfig string) (*rest.Config, error){
  config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
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
                     args.StdinData,
                     nil,
                     nil,
                     nil,
                    }
  return &cmdArgs, nil
}

func getPod(args *cniArgs) error {
  k8sClient, err := createK8sClient(DanmConfig.Kubeconfig)
  if err != nil {
    return errors.New("cannot create K8s REST client due to error:" + err.Error())
  }
  pod, err := k8sClient.CoreV1().Pods(string(args.nameSpace)).Get(string(args.podId), meta_v1.GetOptions{})
  if err != nil {
    return errors.New("failed to get Pod info from K8s API server due to:" + err.Error())
  }
  args.pod = pod
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
  var ifaces []datastructs.Interface
  for key, val := range args.pod.Annotations {
    if strings.Contains(key, danmIfDefinitionSyntax) {
      decoder := json.NewDecoder(bytes.NewReader([]byte(val)))
      //We are using Decoder interface, because it can notify us if any unknown fields were put into the object
      decoder.DisallowUnknownFields()
      err := decoder.Decode(&ifaces)
      if err != nil {
        return errors.New("Can't create network interfaces for Pod: " + args.pod.ObjectMeta.Name + " due to badly formatted " + danmIfDefinitionSyntax + " definition in Pod annotation:" + err.Error())
      }
      break
    }
  }
  if err := validateAnnotation(ifaces); err!=nil {
    return errors.New("DANM annotation is invalid for Pod: " + args.pod.ObjectMeta.Name + ", because:" + err.Error())
  }
  args.interfaces = ifaces
  return nil
}

func validateAnnotation(ifaces []datastructs.Interface) error {
  for ifaceId, iface := range ifaces {
    var definedNetworks int
    if iface.Network        != "" {definedNetworks++}
    if iface.TenantNetwork  != "" {definedNetworks++}
    if iface.ClusterNetwork != "" {definedNetworks++}
    if definedNetworks != 1 {
      return errors.New("network connection no.:" + strconv.Itoa(ifaceId)+ " contains invalid number of network references:" + strconv.Itoa(definedNetworks))
    }
  }
  return nil
}

func setupNetworking(args *cniArgs) (*current.Result, error) {
  err := preparePodForIpv6(args)
  if err != nil {
    return nil, errors.New("failed to prepare Pod for IPv6 due to:" + err.Error())
  }
  allocatedDevices := make(map[string]*[]string)
  syncher := syncher.NewSyncher(len(args.interfaces))
  danmClient, err := CreateDanmClient(DanmConfig.Kubeconfig)
  if err != nil {
    return nil, err
  }
  if args.defaultNetwork != nil {
    syncher.ExpectedNumOfResults++
    defParam := datastructs.Interface{SequenceId: 0, Ip: "dynamic",}
    err = createIface(args, danmClient, args.defaultNetwork, defParam, syncher, allocatedDevices)
    if err != nil {
      syncher.PushResult(args.defaultNetwork.ObjectMeta.Name, err, nil)
    }
  }
  for nicID, nicParams := range args.interfaces {
    nicParams.SequenceId = nicID
    nicParams.DefaultIfaceName = defaultIfName
    netInfo, err := netcontrol.GetNetworkFromInterface(danmClient, nicParams, args.pod.ObjectMeta.Namespace)
    if err != nil {
      syncher.PushResult("", errors.New("failed to get network object for Pod:" + args.pod.ObjectMeta.Name +
                             "'s connection no.:" + strconv.Itoa(nicID) + " due to:" + err.Error()), nil)
      continue
    }
    err = createIface(args, danmClient, netInfo, nicParams, syncher, allocatedDevices)
    if err != nil {
      syncher.PushResult(netInfo.ObjectMeta.Name, err, nil)
      continue
    }
  }
  err = syncher.GetAggregatedResult()
  return syncher.MergeCniResults(), err
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

func createIface(args *cniArgs, danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, nicParams datastructs.Interface, syncher *syncher.Syncher, allocatedDevices map[string]*[]string) error {
  if !isTenantAllowed(args, netInfo) {
    return errors.New("Pod:" + args.pod.ObjectMeta.Name + "'s namespace:" + args.pod.ObjectMeta.Namespace + " is not in the AllowedTenants whitelist of network:" + netInfo.ObjectMeta.Name)
  }
  var err error
  if cnidel.IsDelegationRequired(netInfo) {
    if cnidel.IsDeviceNeeded(netInfo.Spec.NetworkType) {
      if _, ok := allocatedDevices[netInfo.Spec.Options.DevicePool]; !ok {
        checkpoint, err := checkpoint_utils.GetCheckpoint()
        if err != nil {
          return errors.New("failed to instantiate checkpoint object due to:" + err.Error())
        }
        allocatedDevices[netInfo.Spec.Options.DevicePool], err = getAllocatedDevices(args, checkpoint, netInfo.Spec.Options.DevicePool)
        if err != nil {
          return errors.New("failed to get allocated devices due to:" + err.Error())
        }
      }
      nicParams.Device, err = popDevice(netInfo.Spec.Options.DevicePool, allocatedDevices)
      if err != nil {
        return errors.New("failed to pop devices due to:" + err.Error())
      }
    }
    go createDelegatedInterface(syncher, danmClient, nicParams, netInfo, args)
  } else {
    go createDanmInterface(syncher, danmClient, nicParams, netInfo, args)
  }
  return nil
}

func isTenantAllowed(args *cniArgs, netInfo *danmtypes.DanmNet) bool {
  if len(netInfo.Spec.AllowedTenants) == 0 {
    return true
  }
  var isTenantAllowed bool
  for _, tenantName := range netInfo.Spec.AllowedTenants {
    if tenantName == args.pod.ObjectMeta.Namespace {
      isTenantAllowed = true
      break
    }
  }
  return isTenantAllowed
}

func getAllocatedDevices(args *cniArgs, checkpoint multus_types.ResourceClient, devicePool string)(*[]string, error){
  resourceMap, err := checkpoint.GetPodResourceMap(args.pod)
  if err != nil {
    return nil, errors.New("failed to retrieve Pod info from checkpoint object due to:" + err.Error())
  }
  if len(resourceMap) == 0 {
    return nil, errors.New("there were no Devices allocated for the Pod")
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

func createDelegatedInterface(syncher *syncher.Syncher, danmClient danmclientset.Interface, iface datastructs.Interface, netInfo *danmtypes.DanmNet, args *cniArgs) {
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
    cnidel.FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("CNI delegation failed due to error:" + err.Error()), nil)
    return
  }
  err = danmep.PostProcessInterface(ep, netInfo)
  if err != nil {
    cnidel.FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("Post-processing failed for interface:" + ep.Spec.Iface.Name + " because:" + err.Error()), nil)
    return
  }
  err = danmep.PutDanmEp(danmClient, ep)
  if err != nil {
    cnidel.FreeDelegatedIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("DanmEp object could not be PUT to K8s due to error:" + err.Error()), nil)
    return
  }
  syncher.PushResult(netInfo.ObjectMeta.Name, nil, delegatedResult)
}

func createDanmInterface(syncher *syncher.Syncher, danmClient danmclientset.Interface, iface datastructs.Interface, netInfo *danmtypes.DanmNet, args *cniArgs) {
  ip4, ip6, macAddr, err := ipam.Reserve(danmClient, *netInfo, iface.Ip, iface.Ip6)
  if err != nil {
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("IP address reservation failed for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error()), nil)
    return
  }
  //As netInfo is only copied to IPAM above, the IP allocation is not refreshed in the original copy.
  //Without re-reading the network body we risk leaking IPs if an error happens later on within the same thread!
  netInfo,_ = netcontrol.GetNetworkFromInterface(danmClient, iface, netInfo.ObjectMeta.Namespace)
  epSpec := danmtypes.DanmEpIface {
    Name: cnidel.CalculateIfaceName(DanmConfig.NamingScheme, netInfo.Spec.Options.Prefix, iface.DefaultIfaceName, iface.SequenceId),
    Address:     ip4,
    AddressIPv6: ip6,
    MacAddress:  macAddr,
    Proutes:     iface.Proutes,
    Proutes6:    iface.Proutes6,
  }
  ep, err := createDanmEp(epSpec, netInfo, args)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("DanmEp object could not be created due to error:" + err.Error()), nil)
    return
  }
  err = danmep.PutDanmEp(danmClient, ep)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("EP could not be PUT into K8s due to error:" + err.Error()), nil)
    return
  }
  err = danmep.AddIpvlanInterface(netInfo, ep)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    danmClient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &meta_v1.DeleteOptions{})
    syncher.PushResult(netInfo.ObjectMeta.Name, errors.New("IPVLAN interface could not be created due to error:" + err.Error()), nil)
    return
  }
  err = danmep.PostProcessInterface(ep, netInfo)
  if err != nil {
    ipam.GarbageCollectIps(danmClient, netInfo, ip4, ip6)
    danmClient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &meta_v1.DeleteOptions{})
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
    EndpointID:  epid,
    Iface:       epInput,
    Host:        host,
    Pod:         args.podId,
    CID:         args.containerId,
    Netns:       args.netns,
    ApiType:     netInfo.TypeMeta.Kind,
  }
  meta := meta_v1.ObjectMeta {
    Name: epid,
    Namespace: args.nameSpace,
    ResourceVersion: "",
    Labels: args.pod.Labels,
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
  danmClient, err := CreateDanmClient(DanmConfig.Kubeconfig)
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
  //During delete we are not that interested in errors, but we also can't just return yet.
  //We need to try and clean-up as many remaining resources as possible
  var aggregatedError string
  netInfo, err := netcontrol.GetNetworkFromEp(danmClient, ep)
  if err != nil {
    aggregatedError += "failed to get network:"+ err.Error() + "; "
  }
  if netInfo != nil {
    err = deleteNic(danmClient, netInfo, ep)
    if err != nil {
      aggregatedError += "failed to delete container NIC:" + err.Error() + "; "
    }
  }
  err = danmClient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &meta_v1.DeleteOptions{})
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

func deleteDanmNet(danmClient danmclientset.Interface, ep danmtypes.DanmEp, netInfo *danmtypes.DanmNet) error {
  ipam.GarbageCollectIps(danmClient, netInfo, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
  return danmep.DeleteIpvlanInterface(ep)
}

func GetInterfaces(args *skel.CmdArgs) error {
  return nil
}
