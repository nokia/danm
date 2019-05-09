package danmep

import (
  "errors"
  "fmt"
  "log"
  "runtime"
  "strconv"
  "github.com/containernetworking/plugins/pkg/ns"
  "github.com/containernetworking/plugins/pkg/utils/sysctl"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
)

const (
  defaultNamespace = "default"
  dockerApiVersion = "1.22"
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

// FindByCid returns a map of Eps which belong to the same Pod
func FindByCid(client danmclientset.Interface, cid string)([]danmtypes.DanmEp, error) {
  result, err := client.DanmV1().DanmEps("").List(meta_v1.ListOptions{})
  if err != nil {
    log.Println("cannot get list of eps:" + err.Error())
    return nil, err
  }
  eplist := result.Items
  var ret = make([]danmtypes.DanmEp, 0)
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
    log.Println("cannot get list of eps")
    return nil, err
  }
  eplist := result.Items
  var ret = make(map[string]danmtypes.DanmEp, 0)
  for _, ep := range eplist {
    if ep.Spec.Host == host {
      ret[ep.Spec.CID] = ep
    }
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

func CreateRoutesInNetNs(ep danmtypes.DanmEp, dnet *danmtypes.DanmNet, ) error {
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
    return errors.New("failed to enter network namespace of CID:"+ep.Spec.Netns+" with error:"+err.Error())
  }
  return addIpRoutes(ep,dnet)
}

func SetDanmEpSysctls(ep danmtypes.DanmEp) error {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  // save the current namespace
  origNs, err := ns.GetCurrentNS()
  if err != nil {
    return errors.New("failed to get current network namespace due to:" + err.Error())
  }
  // enter to the Pod's network namespace
  podNs, err := ns.GetNS(ep.Spec.Netns)
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
  // set sysctls
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
