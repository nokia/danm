package ipam

import (
  "errors"
  "fmt"
  "net"
  "strconv"
  "strings"
  "time"
  "encoding/binary"
  "math/big"
  "math/rand"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/netcontrol"
  "github.com/nokia/danm/pkg/bitarray"
)

// Reserve inspects the network object received as an input, and allocates an IPv4 or IPv6 address from the appropriate allocation pool
// In case static IP allocation is requested, it will try reserver the requested error. If it is not possible, it returns an error
// The reserved IP address is represented by setting a bit in the network's BitArray type allocation matrix
// The refreshed network object is modified in the K8s API server at the end
func Reserve(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, req4, req6 string) (string, string, string, error) {
  origAlloc := netInfo.Spec.Options.Alloc
  tempNetSpec := netInfo
  for {
    ip4, ip6, macAddr, err := allocateIP(&tempNetSpec, req4, req6)
    if err != nil {
      return "", "", "", errors.New("failed to allocate IP address for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
    }
    //Right now we only store IPv4 allocations in the API. If this bitmask is unchanged, there is nothing to update in the API server
    if tempNetSpec.Spec.Options.Alloc == origAlloc {
      return ip4, ip6, macAddr, nil
    }
    retryNeeded, err, newNetSpec := updateIpAllocation(danmClient, tempNetSpec)
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

// Free inspects the network object received as an input, and releases an IPv4 or IPv6 address from the appropriate allocation pool
// The IP address liberation is represented by unsetting a bit in the network's BitArray type allocation matrix
// The refreshed network object is modified in the K8s API server at the end
func Free(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, ip string) error {
  if netInfo.Spec.Options.Alloc == "" || ip == "" {
    // Nothing to return here: either network, or the interface is an L2
    return nil
  }
  origAlloc := netInfo.Spec.Options.Alloc
  tempNetSpec := netInfo
  for {
    resetIP(&tempNetSpec, ip)
    //Right now we only store IPv4 allocations in the API. If this bitmask is unchanged, there is nothing to update in the API server
    if tempNetSpec.Spec.Options.Alloc == origAlloc {
      return nil
    }
    retryNeeded, err, newNetSpec := updateIpAllocation(danmClient, tempNetSpec)
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

func updateIpAllocation (danmClient danmclientset.Interface, netInfo danmtypes.DanmNet) (bool,error,danmtypes.DanmNet) {
  resourceConflicted, err := netcontrol.PutNetwork(danmClient, &netInfo)
  if err != nil {
    return false, errors.New("DanmNet update failed with error:" + err.Error()), danmtypes.DanmNet{}
  }
  if resourceConflicted {
    newNetSpec, err := netcontrol.RefreshNetwork(danmClient, netInfo)
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
  ipnetNum := Ip2int(ipnet.IP)
  ip, _, err := net.ParseCIDR(rip)
  if err != nil {
    //Invalid IP, nothing to do here. Next call would crash if we wouldn't return
    return
  }
  reserved := Ip2int(ip)
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
      return errors.New("IPv4 address cannot be dynamically allocated for an L2 network!")
    }
    ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
    _, ipnet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
    ipnetNum := Ip2int(ipnet.IP)
    begin := Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.Start)) - ipnetNum
    end := Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.End)) - ipnetNum
    for i:=begin; i<=end; i++ {
      if !ba.Get(uint32(i)) {
        ones, _ := ipnet.Mask.Size()
        *ip4 = (Int2ip(ipnetNum + i)).String() + "/" + strconv.Itoa(ones)
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
    ipnetNum := Ip2int(ipnetFromNet.IP)
    requested := Ip2int(ip)
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
    ss.Add(Ip62int(ip6addr), bigeui)
    maskLen, _ := ip6net.Mask.Size()
    if maskLen>64 {
      return errors.New("IPv6 subnets smaller than /64 are not supported at the moment!")
    }
    *ip6 = (Int2ip6(ss)).String() + "/" + strconv.Itoa(maskLen)
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

// Ip2int converts an IP address stored according to the Golang net package to a native Golang big endian, 32-bit integer
func Ip2int(ip net.IP) uint32 {
  if len(ip) == 16 {
    return binary.BigEndian.Uint32(ip[12:16])
  }
  return binary.BigEndian.Uint32(ip)
}

// Ip62int converts an IPv6 address stored according to the Golang net package to a native Golang big endian, 64-bit integer
func Ip62int(ip6 net.IP) *big.Int {
  Ip6Int := big.NewInt(0)
  Ip6Int.SetBytes(ip6.To16())
  return Ip6Int
}

// Int2ip converts an IP address stored as a native Golang big endian, 32-bit integer to an IP
// represented according to the Golang net package
func Int2ip(nn uint32) net.IP {
  ip := make(net.IP, 4)
  binary.BigEndian.PutUint32(ip, nn)
  return ip
}

// Int2ip6 converts an IP address stored as a native Golang big endian, 64-bit integer to an IP
// represented according to the Golang net package
func Int2ip6(nn *big.Int) net.IP {
  ip := nn.Bytes()
  return ip
}