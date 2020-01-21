package danmep

import (
  "errors"
  "log"
  "net"
  "os"
  "runtime"
  "strconv"
  "github.com/vishvananda/netlink"
  "github.com/containernetworking/plugins/pkg/ns"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/j-keck/arping"
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
  err = configureLink(iface, ep)
  if err != nil {
    return err
  }
  if ep.Spec.Iface.Address != "" && ep.Spec.Iface.Address != ipam.NoneAllocType {
    addr,_,_ := net.ParseCIDR(ep.Spec.Iface.Address)
    err = arping.GratuitousArpOverIfaceByName(addr, ep.Spec.Iface.Name)
    if err != nil {
      log.Println("WARNING: sending gARP failed with error:" + err.Error(), ", but we will ignore that for now!")
    }
  }
  return nil
}

func configureLink(iface netlink.Link, ep danmtypes.DanmEp) error {
  var err error
  if ep.Spec.Iface.Address != "" && ep.Spec.Iface.Address != ipam.NoneAllocType {
    err = addIpToLink(ep.Spec.Iface.Address, iface)
    if err != nil {
      return err
    }
  }
  if ep.Spec.Iface.AddressIPv6 != "" && ep.Spec.Iface.AddressIPv6 != ipam.NoneAllocType {
    err = addIpToLink(ep.Spec.Iface.AddressIPv6, iface)
    if err != nil {
      return err
    }
  }
  err = netlink.LinkSetName(iface, ep.Spec.Iface.Name)
  if err != nil {
    return errors.New("cannot rename link:" + ep.Spec.Iface.Name + " because:" + err.Error())
  }
  err = netlink.LinkSetUp(iface)
  if err != nil {
    return errors.New("cannot set link:" + ep.Spec.Iface.Name + " UP because:" + err.Error())
  }
  return nil
}

func addIpToLink(ip string, iface netlink.Link) error {
  addr, pref, err := net.ParseCIDR(ip)
  if err != nil {
    return errors.New("cannot parse IP address because:" + err.Error())
  }
  ipAddr := &netlink.Addr{IPNet: &net.IPNet{IP: addr, Mask: pref.Mask}}
  err = netlink.AddrAdd(iface, ipAddr)
  if err != nil {
    return errors.New("cannot add IP address to link because:" + err.Error())
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
  if rtable == 0 || cidr == ""  || cidr == ipam.NoneAllocType || proutes == nil {
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

func createDummyInterface(ep danmtypes.DanmEp) error {
  dummy := &netlink.Dummy {
    LinkAttrs: netlink.LinkAttrs {
      Name: ep.Spec.Iface.Name,
    },
  }
  err := netlink.LinkAdd(dummy)
  if err != nil {
    return errors.New("cannot create dummy interface for DPDK because:" + err.Error())
  }
  iface, err := netlink.LinkByName(ep.Spec.Iface.Name)
  if err != nil {
    return errors.New("cannot find freshly created dummy interface because:" + err.Error())
  }
  return configureLink(iface, ep)
}