package ipam

import (
  "errors"
  "fmt"
  "net"
  "strconv"
  "strings"
  "time"
  "math/big"
  "math/rand"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  client "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  "github.com/nokia/danm/pkg/netcontrol"
  "github.com/nokia/danm/pkg/bitarray"
)

// Reserve inspects the DanmNet object received as an input, and allocates an IPv4 or IPv6 address from the appropriate allocation pool
// In case static IP allocation is requested, it will try reserver the requested error. If it is not possible, it returns an error
// The reserved IP address is represented by setting a bit in the network's BitArray type allocation matrix
// The refreshed DanmNet object is modified in the K8s API server at the end
func Reserve(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, req4, req6 string) (string, string, string, error) {
  if netInfo.Spec.Validation != true {
    return "", "", "", errors.New("Invalid network: " + netInfo.Spec.NetworkID)
  }
  tempNetSpec := netInfo
  netClient := danmClient.DanmV1().DanmNets(netInfo.ObjectMeta.Namespace)
  for {
    ip4, ip6, macAddr, err := allocateIP(&tempNetSpec, req4, req6)
    if err != nil {
      return "", "", "", errors.New("failed to allocate IP address for network:" + netInfo.Spec.NetworkID + " with error:" + err.Error())
    }
    retryNeeded, err, newNetSpec := updateDanmNetAllocation(netClient, tempNetSpec)
    if err != nil {
      return "", "", "", err
    }
    if retryNeeded {
      tempNetSpec = newNetSpec
      continue
    }
    return ip4, ip6, macAddr, nil
  }
}

// Free inspects the DanmNet object received as an input, and releases an IPv4 or IPv6 address from the appropriate allocation pool
// The IP address liberation is represented by unsetting a bit in the network's BitArray type allocation matrix
// The refreshed DanmNet object is modified in the K8s API server at the end
func Free(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, ip string) error {
  if netInfo.Spec.Options.Alloc == "" || ip == "" {
    // Nothing to return here: either network, or the interface is an L2
    return nil
  }
  tempNetSpec := netInfo
  netClient := danmClient.DanmV1().DanmNets(netInfo.ObjectMeta.Namespace)
  for {
    resetIP(&tempNetSpec, ip)
    retryNeeded, err, newNetSpec := updateDanmNetAllocation(netClient, tempNetSpec)
    if err != nil {
      return err
    }
    if retryNeeded {
      tempNetSpec = newNetSpec
      continue
    }
    return nil
  }
}

func updateDanmNetAllocation (netClient client.DanmNetInterface, netInfo danmtypes.DanmNet) (bool,error,danmtypes.DanmNet) {
  resourceConflicted, err := netcontrol.PutDanmNet(netClient, &netInfo)
  if err != nil {
    return false, errors.New("DanmNet update failed with error:" + err.Error()), danmtypes.DanmNet{}
  }
  if resourceConflicted {
    newNetSpec, err := netClient.Get(netInfo.Spec.NetworkID, meta_v1.GetOptions{})
    if err != nil {
      return false, errors.New("After IP address reservation conflict, network cannot be read again!"), danmtypes.DanmNet{}
    }
    return true, nil, *newNetSpec
  }
  return false, nil, danmtypes.DanmNet{}
}


func resetIP(netInfo *danmtypes.DanmNet, rip string) {
  ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
  _, ipnet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
  ipnetNum := netcontrol.Ip2int(ipnet.IP)
  ip, _, err := net.ParseCIDR(rip)
  if err != nil {
    //Invalid IP, nothing to do here. Next call would crash if we wouldn't return
    return
  }
  reserved := netcontrol.Ip2int(ip)
  if !ipnet.Contains(ip) {
    //IP is outside of CIDR, nothing to do here. Next call would crash if we wouldn't return
    return
  }
  ba.Reset(reserved - ipnetNum)
  netInfo.Spec.Options.Alloc = ba.Encode()
}

func allocateIP(netInfo *danmtypes.DanmNet, req4, req6 string) (string, string, string, error) {
  var ip4 = ""
  var ip6 = ""
  macAddr := generateMac()
  var err error
  err = nil
  if req4 != "" {
    err = allocIPv4(req4, netInfo, &ip4)
    if err != nil {
      return "", "", "", err
    }
  }
  if req6 != "" {
    err = allocIPv6(req6, netInfo, &ip6, macAddr)
    if err != nil {
      return "", "", "", err
    }
  }
  return ip4, ip6, macAddr, err
}

func allocIPv4(reqType string, netInfo *danmtypes.DanmNet, ip4 *string) (error) {
  if reqType == "none" {
    return nil
  } else if reqType == "dynamic" {
    if netInfo.Spec.Options.Alloc == "" {
      return errors.New("Ipv4 address cannot be dynamically allocated for an L2 network!")
    }
    ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
    _, ipnet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
    ipnetNum := netcontrol.Ip2int(ipnet.IP)
    begin := netcontrol.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.Start)) - ipnetNum
    end := netcontrol.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.End)) - ipnetNum
    for i:=begin; i<=end; i++ {
      if !ba.Get(uint32(i)) {
        ones, _ := ipnet.Mask.Size()
        *ip4 = (netcontrol.Int2ip(ipnetNum + i)).String() + "/" + strconv.Itoa(ones)
        ba.Set(uint32(i))
        netInfo.Spec.Options.Alloc = ba.Encode()
        break
      }
    }
    if *ip4 == "" {
      return errors.New("IPv4 address cannot be dynamically allocated, all addresses are reserved!")
    }
  } else {
    ip, ipnet, _ := net.ParseCIDR(reqType)
    if ip == nil {
      return errors.New("IPv4 allocation failure, invalid static IP requested:" + reqType)
    }
    if netInfo.Spec.Options.Alloc == "" {
      return errors.New("static IP cannot be allocated for a L2 network!")
    }
    _, ipnetFromNet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
    if !(ipnetFromNet.Contains(ip) && ipnetFromNet.Mask.String() == ipnet.Mask.String()) {
      return errors.New("static ip is not part of network CIDR/allocation pool")
    }
    ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
    ipnetNum := netcontrol.Ip2int(ipnetFromNet.IP)
    requested := netcontrol.Ip2int(ip)
    if ba.Get(requested - ipnetNum) {
      return errors.New("requested fix ip address is already in use")
    }
    ba.Set(requested - ipnetNum)
    netInfo.Spec.Options.Alloc = ba.Encode()
    *ip4 = reqType
    return nil
  }
  return nil
}

func allocIPv6(reqType string, netInfo *danmtypes.DanmNet, ip6 *string, macAddr string) (error) {
  if reqType == "none" {
    return nil
  } else if reqType == "dynamic" {
    net6 := netInfo.Spec.Options.Net6
    if net6 == "" {
      return errors.New("ipv6 dynamic address requested without defined IPv6 prefix")
    }
    numMac, _ := strconv.ParseInt(strings.Replace(macAddr, ":", "", -1), 16, 0)
    numMac = numMac^0x020000000000
    hexMac := fmt.Sprintf("%X",numMac)
    eui := fmt.Sprintf("%s%sfffe%s%s", hexMac[:4],hexMac[4:6],hexMac[6:8],hexMac[8:12])
    bigeui := big.NewInt(0)
    bigeui.SetString(eui, 16)
    ip6addr, ip6net, _ := net.ParseCIDR(net6)
    ss := big.NewInt(0)
    ss.Add(netcontrol.Ip62int(ip6addr), bigeui)
    maskLen, _ := ip6net.Mask.Size()
    if maskLen>64 {
      return errors.New("IPv6 subnets smaller than /64 are not supported at the moment!")
    }
    *ip6 = (netcontrol.Int2ip6(ss)).String() + "/" + strconv.Itoa(maskLen)
  } else {
    net6 := netInfo.Spec.Options.Net6
    if net6 == "" {
      return errors.New("Static IPv6 address cannot be allocated for an L2 network!")
    }
    _, ip6net, _ := net.ParseCIDR(net6)
    ip6addr, _, err := net.ParseCIDR(reqType)
    if err != nil {
      return errors.New("Static IPv6 address allocation failed, requested IP is malformed:" + reqType)
    }
    if !ip6net.Contains(ip6addr) {
      return errors.New("Requested static IPv6 address is not part of net6 prefix!")
    }
    *ip6 = reqType
  }
  return nil
}

func generateMac()(string) {
  s1 := rand.NewSource(time.Now().UnixNano())
  r1 := rand.New(s1)
  return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0xfa, 0x16, 0x3e, r1.Intn(127), r1.Intn(255), r1.Intn(255))
}

func GarbageCollectIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, ip4, ip6 string) {
  Free(danmClient, *netInfo, ip4)
  Free(danmClient, *netInfo, ip6)
}