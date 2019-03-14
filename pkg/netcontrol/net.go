package netcontrol

import (
  "encoding/binary"
  "errors"
  "math"
  "math/big"
  "net"
  "strconv"
  "syscall"
  "github.com/apparentlymart/go-cidr/cidr"
  "github.com/vishvananda/netlink"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
)

const (
  ip4MulticastCidr = "239.0.0.0/8"
  ip6MulticastCidr = "ff02::0/16"
  maxSupportedNetmask = 32
  maxVlanId = 4094
  maxVxlanId = 16777214
)

// LinkInfo is an absract struct to represent a host NIC of a special type: either VLAN, or VxLAN
// The ID of the link is stored together with its Golang representation
type LinkInfo struct {
  interfaceId int
  link netlink.Link
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

func getBroadcastAddress(subnet *net.IPNet) (net.IP) {
  ip := make(net.IP, len(subnet.IP.To4()))
  binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(subnet.IP.To4())|^binary.BigEndian.Uint32(net.IP(subnet.Mask).To4()))
  return ip
}

func validateNetwork(dnet *danmtypes.DanmNet) error {
  err := validateDanmNet(dnet)
  if err != nil {
    return err
  }
  validate(dnet)
  return nil
}

func validateDanmNet(dnet *danmtypes.DanmNet) error {
  err := validateIpv4Fields(dnet)
  if err != nil {
    return err
  }
  err = validateIpv6Fields(dnet)
  if err != nil {
    return err
  }
  err = validateVids(dnet)
  if err != nil {
    return err
  }
  return nil
}

func validateIpv4Fields(dnet *danmtypes.DanmNet) error {
  cidr := dnet.Spec.Options.Cidr
  apStart := dnet.Spec.Options.Pool.Start
  apEnd := dnet.Spec.Options.Pool.End
  if cidr == "" {
    if apStart != "" || apEnd != "" {
      return errors.New("Allocation pool cannot be defined without CIDR!")
    }
    return nil
  }
  err := ValidateAllocationPool(dnet)
  if err != nil {
    return err
  }
  bitArray, err := CreateAllocationArray(dnet)
  if err != nil {
    return err
  }
  dnet.Spec.Options.Alloc = bitArray.Encode()
  return nil
}

func CreateAllocationArray(dnet *danmtypes.DanmNet) (*bitarray.BitArray,error) {
  _, ipnet, err := net.ParseCIDR(dnet.Spec.Options.Cidr)
  if err != nil {
    return nil, errors.New("Invalid CIDR parameter: " + dnet.Spec.Options.Cidr)
  }
  bitArray, err := createBitArray(ipnet)
  if err != nil {
    return nil, err
  }
  err = reserveGatewayIps(dnet.Spec.Options.Routes, bitArray, ipnet)
  if err != nil {
    return nil, err
  }
  return bitArray, nil
}

func createBitArray(ipnet *net.IPNet) (*bitarray.BitArray,error) {
  ones, _ := ipnet.Mask.Size()
  if ones > maxSupportedNetmask {
    return nil, errors.New("DANM does not support networks with more than 2^32 IP addresses")
  }
  bitArray,err := bitarray.NewBitArray(int(math.Pow(2,float64(maxSupportedNetmask-ones))))
  if err != nil {
    return nil,errors.New("BitArray allocation failed because:" + err.Error())
  }
  bitArray.Set(uint32(math.Pow(2,float64(maxSupportedNetmask-ones))-1))
  return bitArray,nil
}

func reserveGatewayIps(routes map[string]string, bitArray *bitarray.BitArray, ipnet *net.IPNet) error {
  for _, gw := range routes {
    if !ipnet.Contains(net.ParseIP(gw)) {
      return errors.New("Gateway:" + gw + " is not part of cidr")
    }
    gatewayPosition := Ip2int(net.ParseIP(gw)) - Ip2int(ipnet.IP)
    bitArray.Set(gatewayPosition)
  }
  return nil
}

func ValidateAllocationPool(dnet *danmtypes.DanmNet) error {
  _, ipnet, err := net.ParseCIDR(dnet.Spec.Options.Cidr)
  if err != nil {
    return errors.New("Invalid CIDR parameter: " + dnet.Spec.Options.Cidr)
  }
  if dnet.Spec.Options.Pool.Start == "" {
    dnet.Spec.Options.Pool.Start = (Int2ip(Ip2int(ipnet.IP) + 1)).String()
  }
  if dnet.Spec.Options.Pool.End == "" {
    dnet.Spec.Options.Pool.End = (Int2ip(Ip2int(getBroadcastAddress(ipnet)) - 1)).String()
  }
  if !ipnet.Contains(net.ParseIP(dnet.Spec.Options.Pool.Start)) || !ipnet.Contains(net.ParseIP(dnet.Spec.Options.Pool.End)) {
    return errors.New("Allocation pool is outside of defined CIDR")
  }
  if Ip2int(net.ParseIP(dnet.Spec.Options.Pool.End)) - Ip2int(net.ParseIP(dnet.Spec.Options.Pool.Start)) <= 0 {
    return errors.New("Allocation pool start:" + dnet.Spec.Options.Pool.Start + " is bigger than end:" + dnet.Spec.Options.Pool.End)
  }
  return nil
}

func validateIpv6Fields(dnet *danmtypes.DanmNet) error {
  if dnet.Spec.Options.Net6 == "" {
    return nil
  }
  net6 := dnet.Spec.Options.Net6
  _, ipnet6, err := net.ParseCIDR(net6)
  if err != nil {
    return errors.New("Invalid IPv6 CIDR: " + net6)
  }
  routes6 := dnet.Spec.Options.Routes6
  for _, gw6 := range routes6 {
    if !ipnet6.Contains(net.ParseIP(gw6)) {
      return errors.New("IPv6 GW address:" + gw6 + " is not part of IPv6 CIDR")
    }
  }
  return nil
}

func validateVids(dnet *danmtypes.DanmNet) error {
  isVlanDefined := (dnet.Spec.Options.Vlan!=0)
  isVxlanDefined := (dnet.Spec.Options.Vxlan!=0)
  if isVlanDefined && isVxlanDefined {
    return errors.New("VLAN ID and VxLAN ID parameters are mutually exclusive")
  }
  return nil
}

func deleteNetworks(dnet *danmtypes.DanmNet) error {
  var combinedErrorMessage string
  vxlanId := dnet.Spec.Options.Vxlan
  netId := dnet.Spec.NetworkID
  tempErr := deleteHostInterface(vxlanId, "vx_" + netId)
  if tempErr != nil {
    combinedErrorMessage = tempErr.Error() + "\n"
  }
  vlanId := dnet.Spec.Options.Vlan
  tempErr = deleteHostInterface(vlanId, determineVlanHdev(vlanId, netId, dnet.Spec.Options.Device))
  if tempErr != nil {
    combinedErrorMessage += tempErr.Error()
  }
  if combinedErrorMessage != "" {
    return errors.New(combinedErrorMessage)
  }
  return nil
}

func deleteHostInterface(ifId int, ifName string) error {
  if ifId == 0 {
    return nil
  }
  iface, err := netlink.LinkByName(ifName)
  if err != nil {
    return nil
  }
  err = netlink.LinkDel(iface)
  if err != nil {
    return errors.New("Deletion of interface:" + ifName + " failed with error:"+err.Error())
  }
  return nil
}

func invalidate(dnet *danmtypes.DanmNet) {
  dnet.Spec.Validation = false
}

func validate(dnet *danmtypes.DanmNet) {
  dnet.Spec.Validation = true
}

func setupHost(dnet *danmtypes.DanmNet) error {
  netId := dnet.Spec.NetworkID
  hdev := dnet.Spec.Options.Device
  vxlanId := dnet.Spec.Options.Vxlan
  vlanId := dnet.Spec.Options.Vlan
  // Nothing to do here
  if vxlanId == 0 && vlanId == 0 {
    return nil
  }
  err := setupVlan(vlanId, netId, hdev)
  if err != nil {
    return err
  }
  return setupVxlan(vxlanId, netId, hdev)
}

func setupVlan(vlanId int, netId, hdev string) error {
  vlanName := determineVlanHdev(vlanId, netId, hdev)
  shouldInterfaceBeCreated, hostLink, err := shouldInterfaceBeCreated(vlanId, vlanName, hdev)
  if err != nil {
    return errors.New("cannot set-up host VLAN interface:" + err.Error())
  } else if !shouldInterfaceBeCreated {
    return nil
  }
  vlan := &netlink.Vlan {
    LinkAttrs: netlink.LinkAttrs {
      Name: vlanName,
      ParentIndex: hostLink.link.Attrs().Index,
    },
    VlanId:  hostLink.interfaceId,
  }
  err = addLink(vlan)
  if err != nil {
    return errors.New("cannot add VLAN interface to host due to:"+err.Error())
  }
  return nil
}

func shouldInterfaceBeCreated(ifId int, ifName string, hostDevice string) (bool, LinkInfo, error) {
  hostLink := LinkInfo{}
  if ifId == 0 {
    return false, hostLink, nil
  }
  _, err := netlink.LinkByName(ifName)
  if err == nil {
    return false, hostLink, nil
  }
  dev, err := netlink.LinkByName(hostDevice)
  if err != nil {
    return false, hostLink, errors.New("host device:" + hostDevice + " is not present in the system")
  }
  hostLink.interfaceId = ifId
  hostLink.link = dev
  return true, hostLink, nil
}

func addLink(link netlink.Link) error {
  err := netlink.LinkAdd(link)
  if err != nil {
    return err
  }
  err = netlink.LinkSetUp(link)
  if err != nil {
    return err
  }
  return nil
}

// DetermineVlanHdev returns to which interface a Pod NIC should be connected to in-case VLANs can be in use
// In case VLANs are defined, it returns it in a uniform name, used commonly across DANM
// If the VLAN ID is not defined, then it returns the host device
func determineVlanHdev(vlanId int, netId, hdev string) string {
  if vlanId == 0 {
    return hdev
  }
  return netId + "." + strconv.Itoa(vlanId)
}

func setupVxlan(vxlanId int, netId, hdev string) error {
  vxlanName := "vx_"+netId
  shouldInterfaceBeCreated, hostLink, err := shouldInterfaceBeCreated(vxlanId, vxlanName, hdev)
  if err != nil {
    return errors.New("cannot set-up host VxLAN interface:" + err.Error())
  } else if !shouldInterfaceBeCreated {
    return nil
  }
  mcastIP, err := getMulticastIp(netlink.FAMILY_V4, strconv.Itoa(vxlanId))
  if err != nil {
    return err
  }
  addr, mcast := parseVxlanHostIp(netlink.FAMILY_V4, hostLink.link, mcastIP)
  if addr.String() == "<nil>" {
    mcastIP, tempErr := getMulticastIp(netlink.FAMILY_V6, strconv.Itoa(vxlanId))
    if tempErr != nil {
      return tempErr
    }
    addr, mcast = parseVxlanHostIp(netlink.FAMILY_V6, hostLink.link, mcastIP)
  }
  if addr.String() == "<nil>" {
    return errors.New("VxLAN interface cannot be set-up on top of a host interface:" + hdev + ", which does not have an IP")
  }
  vxlan := &netlink.Vxlan {
    LinkAttrs: netlink.LinkAttrs {
      Name: vxlanName,
    },
    VxlanId:      hostLink.interfaceId,
    VtepDevIndex: hostLink.link.Attrs().Index,
    Port:         4789,
    Group:        mcast,
    SrcAddr:      addr,
    Learning:     true,
    L2miss:       true,
    L3miss:       true,
  }
  err = addLink(vxlan)
  if err != nil {
    return errors.New("cannot add VxLAN interface to the host due to:"+err.Error())
  }
  return nil
}

func getMulticastIp(ipFamily int, vxlanId string ) (net.IP, error) {
  vxlanIdInt, err := strconv.Atoi(vxlanId)
  if err != nil {
    return nil, err
  }
  multicastCidr := ""
  if ipFamily == netlink.FAMILY_V4 {
    multicastCidr = ip4MulticastCidr
  } else if ipFamily == netlink.FAMILY_V6 {
    multicastCidr = ip6MulticastCidr
  }
  _, mcastNet, err := net.ParseCIDR(multicastCidr)
  if err != nil {
    return nil, errors.New("Unable to parse multicast CIDR " + multicastCidr + " due to " + err.Error())
  }
  mcastIP, err := cidr.Host(mcastNet, vxlanIdInt)
  if err != nil {
    return nil, errors.New("Unable to parse multicast IP due to:" + err.Error())
  }
  return mcastIP, nil
}

func parseVxlanHostIp(ipFamily int, hdev netlink.Link, mcastFilter net.IP) (net.IP, net.IP) {
  var hostAddr net.IP
  var hostMultiCastAddr net.IP
  addresses, err := netlink.AddrList(hdev, ipFamily)
  if err != nil {
    return hostAddr, hostMultiCastAddr
  }
  for _, x := range addresses {
    if x.Scope == syscall.RT_SCOPE_UNIVERSE {
      hostAddr = x.IPNet.IP
      hostMultiCastAddr = mcastFilter
      return hostAddr, hostMultiCastAddr
    }
  }
  return hostAddr, hostMultiCastAddr
}
