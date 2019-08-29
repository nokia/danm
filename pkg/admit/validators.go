package admit

import (
  "errors"
  "net"
  "strconv"
  "encoding/binary"
  admissionv1 "k8s.io/api/admission/v1beta1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/danmep"
  "github.com/nokia/danm/pkg/ipam"
  "k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

const (
  MaxNidLength = 11
  MaxNetMaskLength = 8
)

var (
  DanmNetMapping = []ValidatorFunc{validateIpv4Fields,validateIpv6Fields,validateAllocationPool,validateVids,validateNetworkId,validateAbsenceOfAllowedTenants,validateNeType,validateVniChange}
  ClusterNetMapping = []ValidatorFunc{validateIpv4Fields,validateIpv6Fields,validateAllocationPool,validateVids,validateNetworkId,validateNeType,validateVniChange}
  TenantNetMapping = []ValidatorFunc{validateIpv4Fields,validateIpv6Fields,validateAllocationPool,validateAbsenceOfAllowedTenants,validateTenantNetRules,validateNeType}
  danmValidationConfig = map[string]ValidatorMapping {
    "DanmNet": DanmNetMapping,
    "ClusterNetwork": ClusterNetMapping,
    "TenantNetwork": TenantNetMapping,
  }
)

type ValidatorFunc func(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error
type ValidatorMapping []ValidatorFunc

func validateIpv4Fields(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  return validateIpFields(newManifest.Spec.Options.Cidr, newManifest.Spec.Options.Routes)
}

func validateIpv6Fields(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  return validateIpFields(newManifest.Spec.Options.Net6, newManifest.Spec.Options.Routes6)
}

func validateIpFields(cidr string, routes map[string]string) error {
  if cidr == "" {
    if routes != nil  {
      return errors.New("IP routes cannot be defined for a L2 network")
    }
    return nil
  }
  _, ipnet, err := net.ParseCIDR(cidr)
  if err != nil {
    return errors.New("Invalid CIDR: " + cidr)
  }
  if ipnet.IP.To4() != nil {
    ones, _ := ipnet.Mask.Size()
    if ones < MaxNetMaskLength {
      return errors.New("Netmask of the IPv4 CIDR is bigger than the maximum allowed /"+ strconv.Itoa(MaxNetMaskLength))
    }
  }
  for _, gw := range routes {
    if !ipnet.Contains(net.ParseIP(gw)) {
      return errors.New("Specified GW address:" + gw + " is not part of CIDR:" + cidr)
    }
  }
  return nil
}

func validateAllocationPool(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if opType == admissionv1.Create && newManifest.Spec.Options.Alloc != "" {
    return errors.New("Allocation bitmask shall not be manually defined upon creation!")
  }
  cidr := newManifest.Spec.Options.Cidr
  if cidr == "" {
    if newManifest.Spec.Options.Pool.Start != "" || newManifest.Spec.Options.Pool.End != "" {
      return errors.New("Allocation pool cannot be defined without CIDR!")
    }
    return nil
  }
  _, ipnet, _ := net.ParseCIDR(cidr)
  if newManifest.Spec.Options.Pool.Start == "" {
    newManifest.Spec.Options.Pool.Start = (ipam.Int2ip(ipam.Ip2int(ipnet.IP) + 1)).String()
  }
  if newManifest.Spec.Options.Pool.End == "" {
    newManifest.Spec.Options.Pool.End = (ipam.Int2ip(ipam.Ip2int(GetBroadcastAddress(ipnet)) - 1)).String()
  }
  if !ipnet.Contains(net.ParseIP(newManifest.Spec.Options.Pool.Start)) || !ipnet.Contains(net.ParseIP(newManifest.Spec.Options.Pool.End)) {
    return errors.New("Allocation pool is outside of defined CIDR")
  }
  if ipam.Ip2int(net.ParseIP(newManifest.Spec.Options.Pool.End)) <= ipam.Ip2int(net.ParseIP(newManifest.Spec.Options.Pool.Start)) {
    return errors.New("Allocation pool start:" + newManifest.Spec.Options.Pool.Start + " is bigger than or equal to allocation pool end:" + newManifest.Spec.Options.Pool.End)
  }
  return nil
}

func GetBroadcastAddress(subnet *net.IPNet) (net.IP) {
  ip := make(net.IP, len(subnet.IP.To4()))
  //Don't ask
  binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(subnet.IP.To4())|^binary.BigEndian.Uint32(net.IP(subnet.Mask).To4()))
  return ip
}

func validateVids(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  isVlanDefined := (newManifest.Spec.Options.Vlan!=0)
  isVxlanDefined := (newManifest.Spec.Options.Vxlan!=0)
  if isVlanDefined && isVxlanDefined {
    return errors.New("VLAN ID and VxLAN ID parameters are mutually exclusive")
  }
  return nil
}

func validateNetworkId(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if newManifest.Spec.NetworkID == "" {
    return errors.New("Spec.NetworkID mandatory parameter is missing!")
  }
  if len(newManifest.Spec.NetworkID) > MaxNidLength && IsTypeDynamic(newManifest.Spec.NetworkType) {
    return errors.New("Spec.NetworkID cannot be longer than " + strconv.Itoa(MaxNidLength) + " characters (otherwise VLAN and VxLAN host interface creation might fail)!")
  }
  return nil
}

func validateAbsenceOfAllowedTenants(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if newManifest.Spec.AllowedTenants != nil {
    return errors.New("AllowedTenants attribute is only valid for the ClusterNetwork API!")
  }
  return nil
}

func validateTenantNetRules(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if opType == admissionv1.Create &&
    (newManifest.Spec.Options.Vxlan  != 0  ||
     newManifest.Spec.Options.Vlan   != 0) {
    return errors.New("Manually configuring Spec.Options.vlan, or Spec.Options.vxlan attributes is not allowed for TenantNetworks!")
  }
  if opType == admissionv1.Update &&
    (newManifest.Spec.Options.Device  != oldManifest.Spec.Options.Device  ||
     newManifest.Spec.Options.DevicePool  != oldManifest.Spec.Options.DevicePool  ||
     newManifest.Spec.Options.Vxlan   != oldManifest.Spec.Options.Vxlan   ||
     newManifest.Spec.Options.Vlan    != oldManifest.Spec.Options.Vlan) {
    return errors.New("Manually changing any one of Spec.Options. host_device, device_pool, vlan, or vxlan attributes is not allowed for TenantNetworks!")
  }
  return nil
}

func validateTenantconfig(oldManifest, newManifest *danmtypes.TenantConfig, opType admissionv1.Operation) error {
  if len(newManifest.HostDevices) == 0 && len(newManifest.NetworkIds) == 0 {
    return errors.New("Either hostDevices, or networkIds must be provided!")
  }
  var err error
  for _, ifaceConf := range newManifest.HostDevices {
    err = validateIfaceConfig(ifaceConf, opType)
    if err != nil {
      return err
    }
  }
  for nType, nId := range newManifest.NetworkIds {
    if nType == "" || nId == "" {
      return errors.New("neither NetworkID, nor NetworkType can be empty in a NetworkID mapping!")
    }
    if len(nId) > MaxNidLength && IsTypeDynamic(nType) {
      return errors.New("NetworkID:" + nId + " cannot be longer than " + strconv.Itoa(MaxNidLength) + " characters (otherwise VLAN and VxLAN host interface creation might fail)!")
    }
  }
  return nil
}

func validateIfaceConfig(ifaceConf danmtypes.IfaceProfile, opType admissionv1.Operation) error {
  if ifaceConf.Name == "" {
    return errors.New("name attribute of a hostDevice must not be empty!")
  }
  if (ifaceConf.VniType == "" && ifaceConf.VniRange != "") ||
     (ifaceConf.VniRange == "" && ifaceConf.VniType != "") {
    return errors.New("vniRange and vniType attributes must be provided together for interface:" + ifaceConf.Name)
  }
  if ifaceConf.VniType != "" && ifaceConf.VniType != "vlan" && ifaceConf.VniType != "vxlan" {
    return errors.New(ifaceConf.VniType + " is not in allowed vniType values: {vlan,vxlan} for interface:" + ifaceConf.Name)
  }
  if opType == admissionv1.Create && ifaceConf.Alloc != "" {
    return errors.New("Allocation bitmask for interface: " + ifaceConf.Name + " shall not be manually defined upon creation!")
  }
  //I know this type is for CPU sets, but isn't it just perfect for handling arbitrarily defined integer ranges?
  vniSet, err := cpuset.Parse(ifaceConf.VniRange)
  if err != nil {
    return errors.New("vniRange for interface:" + ifaceConf.Name + " must be improperly formatted because its parsing fails with:" + err.Error())
  }
  filteredSet := vniSet.Filter(func(vni int) bool {
    return vni > MaxAllowedVni
  })
  if filteredSet.Size() > 0 {
    return errors.New("vniRange for interface:" + ifaceConf.Name + " is invalid, because it cannot contain VNIs over the maximum supported number that is:" + strconv.Itoa(MaxAllowedVni))
  }
  return nil
}

func validateNeType(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if newManifest.Spec.NetworkType == "sriov" {
    if newManifest.Spec.Options.DevicePool == "" || newManifest.Spec.Options.Device != "" {
      return errors.New("Spec.Options.device_pool must, and Spec.Options.host_device cannot be provided for SR-IOV networks!")
    }
  } else if newManifest.Spec.Options.Device != "" && newManifest.Spec.Options.DevicePool != "" {
    return errors.New("Spec.Options.device_pool and Spec.Options.host_device cannot be provided together!")
  }
  return nil
}

func validateVniChange(oldManifest, newManifest *danmtypes.DanmNet, opType admissionv1.Operation, client danmclientset.Interface) error {
  if opType != admissionv1.Update {
    return nil
  }
  isAnyPodConnectedToNetwork, connectedEp, err := danmep.ArePodsConnectedToNetwork(client, oldManifest)
  if err != nil {
    return errors.New("no way to tell if Pods are still using the network due to:" + err.Error())
  }
  if !isAnyPodConnectedToNetwork {
    return nil
  }
  if (oldManifest.Spec.Options.Vlan  != 0 && (oldManifest.Spec.Options.Vlan  != newManifest.Spec.Options.Vlan  || oldManifest.Spec.Options.Device != newManifest.Spec.Options.Device)) ||
     (oldManifest.Spec.Options.Vxlan != 0 && (oldManifest.Spec.Options.Vxlan != newManifest.Spec.Options.Vxlan || oldManifest.Spec.Options.Device != newManifest.Spec.Options.Device)) {
    return errors.New("cannot change VNI/host_device of a network which having any Pods connected to it e.g. Pod:" + connectedEp.Spec.Pod + " in namespace:" + connectedEp.ObjectMeta.Namespace)
  }
  return nil
}