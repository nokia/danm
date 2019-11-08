package confman

import (
  "errors"
  "log"
  "strings"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/datastructs"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

func GetTenantConfig(danmClient danmclientset.Interface) (*danmtypes.TenantConfig, error) {
  reply, err := danmClient.DanmV1().TenantConfigs().List(metav1.ListOptions{})
  if err != nil {
    return nil, err
  }
  if reply == nil || len(reply.Items) == 0 {
    return nil, errors.New("no TenantConfigs exist int the cluster")
  }
  //TODO: do a namespace based selection later if one generic config does not suffice
  return &reply.Items[0], nil
}

func Reserve(danmClient danmclientset.Interface, tconf *danmtypes.TenantConfig, iface danmtypes.IfaceProfile) (int,error) {
  for {
    chosenVni, err := reserveVni(tconf, iface)
    if err != nil {
      return 0, err
    }
    _, err = danmClient.DanmV1().TenantConfigs().Update(tconf)
    if err != nil && strings.Contains(err.Error(), datastructs.OptimisticLockErrorMsg) {
      tconf, err = danmClient.DanmV1().TenantConfigs().Get(tconf.ObjectMeta.Name, metav1.GetOptions{})
      if err != nil {
        return 0, err
      }
      continue
    }
    return chosenVni, err
  }
}

func reserveVni(tconf *danmtypes.TenantConfig, iface danmtypes.IfaceProfile) (int,error) {
  allocs := bitarray.NewBitArrayFromBase64(iface.Alloc)
  if allocs.Len() == 0 {
    return 0, errors.New("VNI allocations for interface:" + iface.Name + " is corrupt! Are you running without webhook?")
  }
  vnis, err := cpuset.Parse(iface.VniRange)
  if err != nil {
    return 0, errors.New("vniRange for interface:" + iface.Name + " cannot be parsed because:" + err.Error())
  }
  chosenVni := -1
  vniSet := vnis.ToSlice()
  for _, vni := range vniSet {
    if allocs.Get(uint32(vni)) {
      continue
    }
    allocs.Set(uint32(vni))
    iface.Alloc = allocs.Encode()
    chosenVni = vni
    break
  }
  if chosenVni == -1 {
    return 0, errors.New("VNI cannot be allocated from interface profile:" + iface.Name + " because the whole range is already reserved")
  }
  index := getIfaceIndex(tconf, iface.Name, iface.VniType)
  if index == -1 {
    return 0, errors.New("VNI cannot be reserved because selected interface does not exist. You should call for a tech priest, and start praying to the Omnissiah immediately.")
  }
  tconf.HostDevices[index] = iface
  return chosenVni, nil
}

func getIfaceIndex(tconf *danmtypes.TenantConfig, name, vniType string) int {
  for index, iface := range tconf.HostDevices {
    //As HostDevices is a list, the same interface might be added multiple types but with different VNI types
    //We don't want to accidentally overwrite the wrong profile
    if iface.Name == name && iface.VniType == vniType {
      return index
    }
  }
  return -1
}

func Free(danmClient danmclientset.Interface, tconf *danmtypes.TenantConfig, dnet *danmtypes.DanmNet) error {
  if dnet.Spec.Options.Vlan == 0 && dnet.Spec.Options.Vxlan == 0 {
    return nil
  }
  vniType := "vlan"
  if dnet.Spec.Options.Vxlan != 0 {
    vniType = "vxlan"
  }
  ifaceName := dnet.Spec.Options.Device
  if dnet.Spec.Options.DevicePool != "" {
    ifaceName = dnet.Spec.Options.DevicePool
  }
  index := getIfaceIndex(tconf,ifaceName,vniType)
  if index < 0 {
    log.Println("WARNING: There is a data incosistency between TenantNetwork:" + dnet.ObjectMeta.Name + " in namespace:" +
    dnet.ObjectMeta.Namespace + " , and TenantConfig:" + tconf.ObjectMeta.Name +
    " as the used network details (interface name, VNI type) doe not match any entries in TenantConfig. This means your APIs were possibly tampered with!")
    return nil
  }
  for {
    err := freeVni(tconf, dnet, index)
    if err != nil {
      return err
    }
    _, err = danmClient.DanmV1().TenantConfigs().Update(tconf)
    if err != nil && strings.Contains(err.Error(), datastructs.OptimisticLockErrorMsg) {
      tconf, err = danmClient.DanmV1().TenantConfigs().Get(tconf.ObjectMeta.Name, metav1.GetOptions{})
      if err != nil {
        return err
      }
      continue
    }
    return err
  }
}

func freeVni(tconf *danmtypes.TenantConfig, dnet *danmtypes.DanmNet, index int) error {
  vni := dnet.Spec.Options.Vlan
  if dnet.Spec.Options.Vxlan != 0 {
    vni = dnet.Spec.Options.Vxlan
  }
  allocs := bitarray.NewBitArrayFromBase64(tconf.HostDevices[index].Alloc)
  if allocs.Len() == 0 {
    return errors.New("VNI allocations for interface:" + tconf.HostDevices[index].Name + " is corrupt! Are you running without webhook?")
  }
  allocs.Reset(uint32(vni))
  tconf.HostDevices[index].Alloc = allocs.Encode()
  return nil
}