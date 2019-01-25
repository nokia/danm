package danmep

import (
  "errors"
  "log"
  "net"
  "os"
  "runtime"
  "strconv"
  "strings"
  "os/exec"
  "github.com/vishvananda/netlink"
  "github.com/containernetworking/plugins/pkg/ns"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
)

func createIpvlanInterface(dnet *danmtypes.DanmNet, ep danmtypes.DanmEp) error {
  host, err := os.Hostname()
  if err != nil {
    return errors.New("cannot get hostname because:" + err.Error())
  }
  if host != ep.Spec.Host {
    //It should never happen that an interface is created from an Ep belonging to another host
    return nil
  }
  if ns.IsNSorErr(ep.Spec.Netns) != nil {
    return errors.New("Cannot get container pid!")
  }
  device := DetermineHostDeviceName(dnet)
  return createContainerIface(ep, dnet, device)
}

// TODO: Refactor this, as cyclomatic complexity is too high
func createContainerIface(ep danmtypes.DanmEp, dnet *danmtypes.DanmNet, device string) error {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  origns, err := ns.GetCurrentNS()
  if err != nil {
    return errors.New("getting current namespace failed")
  }
  hns, err := ns.GetNS(ep.Spec.Netns)
  if err != nil {
    return errors.New("cannot open network namespace:" + ep.Spec.Netns)
  }
  defer func() {
    hns.Close()
    err = origns.Set()
    if err != nil {
      log.Println("Could not switch back to default ns during IPVLAN interface creation:" + err.Error())
    }
  }()
  //cns,_ := ns.GetCurrentNS()
  cpath := origns.Path()
  log.Println("EP NS BASE PATH:" + cpath)
  iface, err := netlink.LinkByName(device)
  if err != nil {
    return errors.New("cannot find host device because:" + err.Error())
  }
  outer := ep.Spec.EndpointID
  ipvlan := &netlink.IPVlan {
    LinkAttrs: netlink.LinkAttrs {
      Name:        outer[0:15],
      ParentIndex: iface.Attrs().Index,
      MTU:         iface.Attrs().MTU,
    },
    Mode: netlink.IPVLAN_MODE_L2,
  }
  err = netlink.LinkAdd(ipvlan)
  if err != nil {
    return errors.New("cannot create IPVLAN interface because:" + err.Error())
  }
  peer, err := netlink.LinkByName(outer[0:15])
  if err != nil {
    return errors.New("cannot find created IPVLAN interface because:" + err.Error())
  }
  err = netlink.LinkSetNsFd(peer, int(hns.Fd()))
  if err != nil {
    netlink.LinkDel(peer)
    return errors.New("cannot move IPVLAN interface to netns because:" + err.Error())
  }
  // now change to network namespace
  err = hns.Set()
  if err != nil {
    return errors.New("failed to enter network namespace of CID:"+ep.Spec.Netns+" with error:"+err.Error())
  }
  iface, err = netlink.LinkByName(outer[0:15])
  if err != nil {
    return errors.New("cannot find IPVLAN interface in network namespace:" + err.Error())
  }
  ip := ep.Spec.Iface.Address
  if ip != "" {
    addr, pref, err := net.ParseCIDR(ip)
    if err != nil {
      return errors.New("cannot parse ip4 address because:" + err.Error())
    }
    ipAddr := &netlink.Addr{IPNet: &net.IPNet{IP: addr, Mask: pref.Mask}}
    err = netlink.AddrAdd(iface, ipAddr)
    if err != nil {
      return errors.New("Cannot add ip4 address to IPVLAN interface because:" + err.Error())
    }
  }
  ip6 := ep.Spec.Iface.AddressIPv6
  // TODO: Refactor, duplicate of 87-98
  if ip6 != "" {
    addr6, pref,  err := net.ParseCIDR(ip6)
    if err != nil {
      return errors.New("cannot parse ip6 address because:" + err.Error())
    }
    ipAddr6 := &netlink.Addr{IPNet: &net.IPNet{IP: addr6, Mask: pref.Mask}}
    err = netlink.AddrAdd(iface, ipAddr6)
    if err != nil {
      return errors.New("Cannot add ip6 address to IPVLAN interface because:" + err.Error())
    }
  }
  dstPrefix := ep.Spec.Iface.Name
  err = netlink.LinkSetName(iface, dstPrefix)
  if err != nil {
    return errors.New("cannot rename IPVLAN interface because:" + err.Error())
  }
  err = netlink.LinkSetUp(iface)
  if err != nil {
    return errors.New("cannot set renamed IPVLAN interface to up because:" + err.Error())
  }
  sendGratArps(ip, ip6, dstPrefix)
  err = addIpRoutes(ep, dnet)
  if err != nil {
    return errors.New("IP routes could not be provisioned, because:" + err.Error())
  }
  return nil
}

func sendGratArps(srcAddrIpV4, srcAddrIpV6, ifaceName string) {
  var err error
  if srcAddrIpV4!="" {
    err = executeArping(srcAddrIpV4, ifaceName)
  }
  if srcAddrIpV6!="" {
    err = executeArping(srcAddrIpV6, ifaceName)
  }
  if err != nil {
    log.Println(err.Error())
  }
}

func executeArping(srcAddr, ifaceName string) error {
  address,_,err := net.ParseCIDR(srcAddr)
  if err != nil {
    return errors.New("IP address parsing during gARP update was unsuccessful:" + err.Error())
  }
  iface := strings.TrimSpace(ifaceName)
  ip := strings.TrimSpace(address.String())
  cmd := exec.Command("arping","-c1","-A","-I"+iface,ip) // #nosec
  err = cmd.Run()
  if err != nil {
    return errors.New("gARP update for IP address: " + address.String() + " was unsuccessful:" + err.Error())
  }
  return nil
}

func addIpRoutes(ep danmtypes.DanmEp, dnet *danmtypes.DanmNet) error {
  defaultRoutingTable := 0
  err := addRoutes(dnet.Spec.Options.Routes, defaultRoutingTable)
  if err != nil {
    return err
  }
  err = addRoutes(dnet.Spec.Options.Routes6, defaultRoutingTable)
  if err != nil {
    return err
  }
  err = addPolicyRoute(dnet.Spec.Options.RTables, ep.Spec.Iface.Address, ep.Spec.Iface.Proutes)
  if err != nil {
    return err
  }
  err = addPolicyRoute(dnet.Spec.Options.RTables, ep.Spec.Iface.AddressIPv6, ep.Spec.Iface.Proutes6)
  if err != nil {
    return err
  }
  return nil
}

func addRoutes(routes map[string]string, rtable int) error {
  if routes == nil {
    return nil
  }
  for key, value := range routes {
    _, ipnet, err := net.ParseCIDR(key)
    if err != nil {
      //Bad destination in IP route, ignoring the route
      continue
    }
    ip := net.ParseIP(value)
    if ip == nil {
      //Bad gateway in IP route, ignoring the route
      continue
    }
    route := netlink.Route{
      Dst:   ipnet,
      Gw:    ip,
    }
    if rtable == 0 {
      route.Scope = netlink.SCOPE_UNIVERSE
    } else {
      route.Table = rtable
    }
    err = netlink.RouteAdd(&route)
    if err != nil {
      return errors.New("Adding IP route with destination:" + ipnet.String() + " and gateway:" + ip.String() + "failed with error:" + err.Error())
    }
  }
  return nil
}

func addPolicyRoute(rtable int, cidr string, proutes map[string]string) error {
  if rtable == 0 || cidr == "" || proutes == nil {
    return nil
  }
  srcIp, srcNet, _ := net.ParseCIDR(cidr)
  srcPref := &net.IPNet{IP: srcIp, Mask: srcNet.Mask}
  rule := netlink.NewRule()
  rule.Src = srcPref
  rule.Table = rtable
  err := netlink.RuleAdd(rule)
  if err != nil {
    return errors.New("cannot add rule for policy-based IP routes because:" + err.Error())
  }
  err = addRoutes(proutes, rtable)
  if err != nil {
    return err
  }
  return nil
}

func deleteContainerIface(ep danmtypes.DanmEp) error {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  origns, err := ns.GetCurrentNS()
  if err != nil {
    return errors.New("getting the current netNS failed")  
  }
  hns, err := ns.GetNS(ep.Spec.Netns)
  if err != nil {
    return errors.New("cannot open network namespace:" + ep.Spec.Netns)
  }
  defer func() {
    hns.Close()
    origns.Set()
  }()
  err = hns.Set()
  if err != nil {
    return errors.New("failed to enter network namespace" + ep.Spec.Netns)
  }
  device := ep.Spec.Iface.Name
  iface, err := netlink.LinkByName(device)
  if err != nil {
    return errors.New("cannot find device:" + device)
  }
  err = netlink.LinkDel(iface)
  if err != nil {
    return errors.New("cannot delete device:" + device)
  }
  return nil
}

func determineIfName(dnet *danmtypes.DanmNet) string {
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

func deleteEp(ep danmtypes.DanmEp) error {
  if ns.IsNSorErr(ep.Spec.Netns) != nil {
    return errors.New("Cannot find netns")
  }
  return deleteContainerIface(ep)
}
