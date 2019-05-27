package confman

import (
  "errors"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/metacni"
  "k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

func Reserve(tconf *danmtypes.TenantConfig, iface danmtypes.IfaceProfile) (int,error) {
  allocs := bitarray.NewBitArrayFromBase64(iface.Alloc)
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
  updateIfaceInConfig(tconf, iface)
  err = updateConfigInApi(tconf)
  if err != nil {
    return 0, errors.New("VNI allocation of TenantConfig cannot be updated in the Kubernetes API because:" + err.Error())  
  }
  return chosenVni, nil
}

func updateIfaceInConfig(tconf *danmtypes.TenantConfig, iface danmtypes.IfaceProfile) {
  for index, oldIface := range tconf.HostDevices {
    //As HostDevices is a list, the same interface might be added multiple types but with different VNI types
    //We don't want oto accidentally overwrite the wrong profile
    if oldIface.Name == iface.Name && oldIface.VniType == iface.VniType {
      tconf.HostDevices[index] = iface
    }
  }
}

func updateConfigInApi(tconf *danmtypes.TenantConfig) error {
  danmClient, err := metacni.CreateDanmClient()
  if err != nil {
    return err
  }
  confClient := danmClient.DanmV1().TenantConfigs()
  //TODO: now, do we actually need to manually check for the OptimisticLockErrorMessage when we use a generated client,
  //or that is actually dead code in ipam as well?
  //Let's find out!
  _, err = confClient.Update(tconf)
  return err
}