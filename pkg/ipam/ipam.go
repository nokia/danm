package ipam

import (
  "errors"
  "fmt"
  "math"
  "net"
  "reflect"
  "strconv"
  "strings"
  "time"
  "encoding/binary"
  "math/big"
  "math/rand"
  "github.com/apparentlymart/go-cidr/cidr"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/netcontrol"
)

const (
  NoneAllocType = "none"
  DynamicAllocType = "dynamic"
)

// Reserve inspects the network object received as an input, and allocates an IPv4 or IPv6 address from the appropriate allocation pool
// In case static IP allocation is requested, it will try reserver the requested error. If it is not possible, it returns an error
// The reserved IP addresses are represented by setting a bit in the network's BitArray type allocation matrices
// The refreshed network object is modified in the K8s API server at the end
func Reserve(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, req4, req6 string) (string, string, error) {
  origSpec := netInfo.Spec
  tempNet := netInfo
  for {
    ip4, ip6, err := allocateIps(&tempNet, req4, req6)
    if err != nil {
      return "", "", errors.New("failed to allocate IP address for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
    }
    //There is nothing to update in the API server if the network is unchanged after IP reservation
    if reflect.DeepEqual(origSpec, tempNet.Spec) {
      return ip4, ip6, nil
    }
    retryNeeded, err, newNetSpec := updateIpAllocation(danmClient, tempNet)
    if err != nil {
      return "", "", err
    }
    if retryNeeded {
      tempNet = newNetSpec
      continue
    }
    return ip4, ip6, nil
  }
}

// Free inspects the network object received as an input, and releases an IPv4 or IPv6 address from the appropriate allocation pool
// The IP address liberation is represented by unsetting a bit in the network's BitArray type allocation matrix
// The refreshed network object is modified in the K8s API server at the end
func Free(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, ip string) error {
  if netInfo.Spec.Options.Alloc == "" || ip == "" || ip == NoneAllocType {
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

func allocateIps(netInfo *danmtypes.DanmNet, req4, req6 string) (string, string, error) {
  ip4 := ""
  ip6 := ""
  var err error
  if req4 != "" {
    netInfo.Spec.Options.Alloc, err = allocateAddress(netInfo.Spec.Options.Pool, netInfo.Spec.Options.Alloc, req4, netInfo.Spec.Options.Cidr, &ip4)
    if err != nil {
      return "", "", err
    }
  }
  if req6 != "" {
    err = allocIPv6(req6, netInfo, &ip6)
    if err != nil {
      return "", "", err
    }
  }
  return ip4, ip6, err
}

func allocateAddress(pool danmtypes.IpPool, alloc, reqType, cidr string, ip *string) (string,error) {
  if reqType == NoneAllocType {
    *ip = NoneAllocType
    return alloc, nil
  }
  if alloc == "" {
    return alloc, errors.New("IP address cannot be allocated for an L2 network!")
  }
  ba := bitarray.NewBitArrayFromBase64(alloc)
  _, subnet, _   := net.ParseCIDR(cidr)
  var allocatedIndex uint32
  if reqType == DynamicAllocType {
    begin, end := getAllocRangeBasedOnCidr(pool, subnet)
    var doesAnyFreeIpExist bool
    for i:=begin; i<=end; i++ {
      if !ba.Get(i) {
        ba.Set(i)
        allocatedIndex = i
        doesAnyFreeIpExist = true
        break
      }
    }
    if !doesAnyFreeIpExist {
      return alloc, errors.New("IPv4 address cannot be dynamically allocated, all addresses are reserved!")
    }
  } else {
    //I guess we are doing backward compatibility now :)
    //You used to be able to define a static IP in CIDR format, so now we need to trim the suffix if it is there
    requestParts := strings.Split(reqType, "/")
    ip := net.ParseIP(requestParts[0])
    if ip == nil {
      return alloc, errors.New("static IP allocation failed, requested static IP:" + reqType + " is not a valid IP")
    }
    if !(subnet.Contains(ip)) {
      return alloc, errors.New("static IP allocation failed, requested static IP:" + reqType + " is outside the network's CIDR:" + cidr)
    }
    allocatedIndex = getIndexOfIp(ip, subnet)
    if ba.Get(allocatedIndex) {
      return alloc, errors.New("static IP allocation failed, requested IP address:" + reqType + " is already in use")
    }
    ba.Set(allocatedIndex)
  }
  *ip = getIpFromIndex(allocatedIndex, subnet)
  return ba.Encode(), nil
}

func getAllocRangeBasedOnCidr(pool danmtypes.IpPool, cidr *net.IPNet) (uint32,uint32) {
  var beginAsInt, endAsInt uint32
  if cidr.IP.To4() != nil {
    firstIpAsInt := Ip2int(cidr.IP)
    beginAsInt   = Ip2int(net.ParseIP(pool.Start)) - firstIpAsInt
    endAsInt     = Ip2int(net.ParseIP(pool.End)) - firstIpAsInt
  }
  return beginAsInt, endAsInt
}

func getIndexOfIp(ip net.IP, subnet *net.IPNet) uint32 {
  if ip.To4() != nil {
    firstIpAsInt := Ip2int(subnet.IP)
    ipAsInt      := Ip2int(ip)
    return ipAsInt - firstIpAsInt
  }
  return 0
}

func getIpFromIndex(index uint32, subnet *net.IPNet) string {
  prefix, _ := subnet.Mask.Size()
  var ip net.IP
  if subnet.IP.To4() != nil {
    firstIpAsInt := Ip2int(subnet.IP)
    ip = Int2ip(firstIpAsInt + index)
  }
  return ip.String() + "/" + strconv.Itoa(prefix)
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

func allocIPv6(reqType string, netInfo *danmtypes.DanmNet, ip6 *string) (error) {
  if reqType == NoneAllocType {
    *ip6 = NoneAllocType
    return nil
  } else if reqType == DynamicAllocType {
    net6 := netInfo.Spec.Options.Net6
    if net6 == "" {
      return errors.New("ipv6 dynamic address requested without defined IPv6 prefix")
    }
    numMac, _ := strconv.ParseInt(strings.Replace("00:00:11:22:33:44:66:77", ":", "", -1), 16, 0)
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

func CreateAllocationArray(subnet *net.IPNet, routes map[string]string) string {
  bitArray,_ := bitarray.CreateBitArrayFromIpnet(subnet)
  reserveGatewayIps(routes, bitArray, subnet)
  return bitArray.Encode()
}

func reserveGatewayIps(routes map[string]string, bitArray *bitarray.BitArray, ipnet *net.IPNet) {
  for _, gw := range routes {
    var gatewayPosition uint32
    if ipnet.IP.To4() != nil {
      gatewayPosition = Ip2int(net.ParseIP(gw)) - Ip2int(ipnet.IP)
    } else {
    //TODO: IPv6 specific allocation algorithm comes here in the next PR
    }
    bitArray.Set(gatewayPosition)
  }
}


func DoV6CidrsIntersect(masterCidr, subCidr *net.IPNet) bool {
  firstAllocIp, lastAllocIp := cidr.AddressRange(subCidr)
  //Brute force: if the Alloc6 CIDR's first, and last IP both belongs to Net6, we assume the whole CIDR also does
  if masterCidr.Contains(firstAllocIp) && masterCidr.Contains(lastAllocIp) {
    return true
  }
  return false
}

func GetMaxUsableV6Prefix(dnet *danmtypes.DanmNet) int {
  if dnet.Spec.Options.Cidr == "" {
    return datastructs.MaxV6PrefixLength
  }
  _, v4Cidr, _ := net.ParseCIDR(dnet.Spec.Options.Cidr)
  sizeOfV4AllocPool := cidr.AddressCount(v4Cidr)
  maxRemainingCapacity := uint64(math.Pow(2,float64(bitarray.MaxSupportedAllocLength))) - sizeOfV4AllocPool
  maxUsableV6Prefix := datastructs.MinV6PrefixLength
  for pref := datastructs.MaxV6PrefixLength; pref <= datastructs.MinV6PrefixLength; pref++ {
    _, testCidr, _ := net.ParseCIDR("2a00:8a00:a000:1193:f816:3eff:fe24:e348/" + strconv.Itoa(pref))
    if cidr.AddressCount(testCidr) < maxRemainingCapacity {
      maxUsableV6Prefix = pref
      break
    }
  }
  return maxUsableV6Prefix
}