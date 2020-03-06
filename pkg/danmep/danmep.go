package danmep

import (
  "errors"
  "fmt"
  "log"
  "runtime"
  "strconv"
  "time"
  "github.com/containernetworking/plugins/pkg/ns"
  "github.com/containernetworking/plugins/pkg/utils/sysctl"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  sriov_utils "github.com/intel/sriov-cni/pkg/utils"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/ipam"
)

type sysctlFunction func(danmtypes.DanmEp) bool
type sysctlObject struct {
  sysctlName  string
  sysctlValue string
}
type sysctlTask struct {
  sysctlFunc sysctlFunction
  sysctlData []sysctlObject
}

var sysctls = []sysctlTask {
  {
    sysctlFunc: isIPv6Needed,
    sysctlData: []sysctlObject {
      {"net.ipv6.conf.%s.disable_ipv6", "0"},
      {"net.ipv6.conf.%s.autoconf", "0"},
      {"net.ipv6.conf.%s.accept_ra", "0"},
    },
  },
  {
    sysctlFunc: isIPv6NotNeeded,
    sysctlData: []sysctlObject {
      {"net.ipv6.conf.%s.disable_ipv6", "1"},
    },
  },
}

// DeleteIpvlanInterface deletes a Pod's IPVLAN network interface based on the related DanmEp
func DeleteIpvlanInterface(ep danmtypes.DanmEp) (error) {
  return deleteEp(ep)
}

// FindByCid returns a map of DanmEps which belongs to the same infra container ID
func FindByCid(client danmclientset.Interface, cid string)([]danmtypes.DanmEp, error) {
  var err error
  var result *danmtypes.DanmEpList
  //Critical CNI_DEL calls depends on this function, so we will re-try for one sec to be able to cope with temporary network disruptions
  for i := 0; i < 100; i++ {
    result, err = client.DanmV1().DanmEps("").List(meta_v1.ListOptions{})
    if err == nil {
      break
    }
    time.Sleep(10 * time.Millisecond)
  }
  if err != nil {
    return nil, errors.New("cannot list DanmEps because:" + err.Error())
  }
  ret := make([]danmtypes.DanmEp, 0)
  if result == nil {
    return ret, nil
  }
  eplist := result.Items
  for _, ep := range eplist {
    if ep.Spec.CID == cid {
      ret = append(ret, ep)
    }
  }
  return ret, nil
}

// CidsByHost returns a map of Eps
// The Eps in the map are indexed with the name of the K8s host their Pods are running on
func CidsByHost(client danmclientset.Interface, host string)(map[string]danmtypes.DanmEp, error) {
  result, err := client.DanmV1().DanmEps("").List(meta_v1.ListOptions{})
  if err != nil {
    return nil, errors.New("cannot list DanmEps because:" + err.Error())
  }
  ret := make(map[string]danmtypes.DanmEp, 0)
  if result == nil {
    return ret, nil
  }
  eplist := result.Items
  for _, ep := range eplist {
    if ep.Spec.Host == host {
      ret[ep.Spec.CID] = ep
    }
  }
  return ret, nil
}

// FindByPodName returns a map of DanmEps which belong to the same Pod in a given namespace
// If no Pod name is provided, function returns all DanmEps
func FindByPodName(client danmclientset.Interface, podName, ns string) ([]danmtypes.DanmEp, error) {
  result, err := client.DanmV1().DanmEps(ns).List(meta_v1.ListOptions{})
  if err != nil {
    return nil, errors.New("cannot list DanmEps because:" + err.Error())
  }
  ret := make([]danmtypes.DanmEp, 0)
  if result == nil {
    return ret, nil
  }
  eplist := result.Items
  for _, ep := range eplist {
    if podName != "" && ep.Spec.Pod != podName {
      continue
    }
    ret = append(ret, ep)
  }
  return ret, nil
}


func AddIpvlanInterface(dnet *danmtypes.DanmNet, ep danmtypes.DanmEp) error {
  if ep.Spec.NetworkType != "ipvlan" {
    return nil
  }
  return createIpvlanInterface(dnet, ep)
}

func DetermineHostDeviceName(dnet *danmtypes.DanmNet) string {
  var device string
  isVlanDefined := (dnet.Spec.Options.Vlan!=0)
  isVxlanDefined := (dnet.Spec.Options.Vxlan!=0)
  if isVxlanDefined {
    device = "vx_" + dnet.Spec.NetworkID
  } else if isVlanDefined {
    vlanId := strconv.Itoa(dnet.Spec.Options.Vlan)
    device = dnet.Spec.NetworkID + "." + vlanId
  } else {
    device = dnet.Spec.Options.Device
  }
  return device
}

func PostProcessInterface(ep danmtypes.DanmEp, dnet *danmtypes.DanmNet) error {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  origNs, err := ns.GetCurrentNS()
  if err != nil {
    return errors.New("getting current namespace failed")
  }
  hns, err := ns.GetNS(ep.Spec.Netns)
  if err != nil {
    return errors.New("cannot open network namespace:" + ep.Spec.Netns)
  }
  defer func() {
    hns.Close()
    err = origNs.Set()
    if err != nil {
      log.Println("Could not switch back to default ns during IP route provisioning operation:" + err.Error())
    }
  }()
  err = hns.Set()
  if err != nil {
    return errors.New("failed to enter network namespace of CID:" + ep.Spec.Netns + " with error:" + err.Error())
  }
  isVfAttachedToDpdkDriver,_ := sriov_utils.HasDpdkDriver(ep.Spec.Iface.DeviceID)
  if isVfAttachedToDpdkDriver {
    err = createDummyInterface(ep)
    if err != nil {
      return errors.New("failed to create dummy kernel interface for " + ep.Spec.Iface.Name + " because:" + err.Error())
    }
  }
  err = setDanmEpSysctls(ep)
  if err != nil {
    return errors.New("failed to set kernel configs for interface" + ep.Spec.Iface.Name + " beause:" + err.Error())
  }
  return addIpRoutes(ep,dnet)
}

func setDanmEpSysctls(ep danmtypes.DanmEp) error {
  var err error
  for _, s := range sysctls {
    if s.sysctlFunc(ep) {
      for _, ss := range s.sysctlData {
        sss := fmt.Sprintf(ss.sysctlName, ep.Spec.Iface.Name)
        _, err = sysctl.Sysctl(sss, ss.sysctlValue)
        if err != nil {
          return errors.New("failed to set sysctl due to:" + err.Error())
        }
      }
    }
  }
  return nil
}

func isIPv6Needed(ep danmtypes.DanmEp) bool {
  if ep.Spec.Iface.AddressIPv6 != "" {
    return true
  }
  return false
}

func isIPv6NotNeeded(ep danmtypes.DanmEp) bool {
  if ep.Spec.Iface.AddressIPv6 == "" {
    return true
  }
  return false
}

func PutDanmEp(danmClient danmclientset.Interface, ep danmtypes.DanmEp) error {
  _, err := danmClient.DanmV1().DanmEps(ep.Namespace).Create(&ep)
  if err != nil {
    return errors.New("DanmEp object could not be PUT to K8s API server due to error:" + err.Error())
  }
  //We block the thread until DanmEp is really created in the API server, just in case
  //We achieve this by not returning until Get for the same resource is successful
  //Otherwise garbage collection could leak during CNI ADD if another thread finished unsuccessfully,
  //simply because the DanmEp directing interface deletion does not yet exist
  for i := 0; i < 100; i++ {
    tempEp, err := danmClient.DanmV1().DanmEps(ep.Namespace).Get(ep.ObjectMeta.Name, meta_v1.GetOptions{})
    if err == nil && tempEp.ObjectMeta.Name == ep.ObjectMeta.Name {
      return nil
    }
    time.Sleep(10 * time.Millisecond)
  }
  return errors.New("DanmEp creation was supposedly successful, but the object hasn't really appeared within 1 sec")
}

// ArePodsConnectedToNetwork checks if there are any Pods currently in the system using the particular network.
// If there is at least, it returns true, and the spec of the first matching DanmEp.
func ArePodsConnectedToNetwork(client danmclientset.Interface, dnet *danmtypes.DanmNet)(bool, danmtypes.DanmEp, error) {
  result, err := client.DanmV1().DanmEps("").List(meta_v1.ListOptions{})
  if err != nil {
    return false, danmtypes.DanmEp{}, errors.New("cannot list DanmEps because:" + err.Error())
  }
  if result == nil {
    return false, danmtypes.DanmEp{}, nil
  }
  eplist := result.Items
  for _, ep := range eplist {
    if (ep.Spec.ApiType == dnet.TypeMeta.Kind && ep.Spec.NetworkName == dnet.ObjectMeta.Name) &&
       (dnet.TypeMeta.Kind == "ClusterNetwork" || ep.ObjectMeta.Namespace == dnet.ObjectMeta.Namespace ) {
      return true, ep, nil
    }
  }
  return false, danmtypes.DanmEp{}, nil
}

//DeleteDanmEp is a RAII-like API to automatically free IP allocations whenever the resource holding these allocations is deleted
//It helps making sure IPs are always and only freed when a DanmEp is indeed deleted
func DeleteDanmEp(danmClient danmclientset.Interface, ep danmtypes.DanmEp, dnet *danmtypes.DanmNet) error {
  var err error
  if (ep.Spec.Iface.Address != "" || ep.Spec.Iface.AddressIPv6 != "") && dnet == nil {
    return errors.New("DanmEp:" + ep.ObjectMeta.Name + " cannot be safely deleted because its linked network is not available to free DANM IPAM allocated IPs")
  }
  //We only need to Free an IP if it was allocated by DANM IPAM, and it was allocated by DANM only if it falls into any of the defined subnets
  if ipam.WasIpAllocatedByDanm(ep.Spec.Iface.Address, dnet.Spec.Options.Cidr) || ipam.WasIpAllocatedByDanm(ep.Spec.Iface.AddressIPv6, dnet.Spec.Options.Pool6.Cidr) {
    err = ipam.GarbageCollectIps(danmClient, dnet, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
      return errors.New("DanmEp:" + ep.ObjectMeta.Name + " cannot be safely deleted because freeing its reserved IP addresses failed with error:" + err.Error())
    }
  }
  return danmClient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &meta_v1.DeleteOptions{})
}