package netadmit

import (
  "errors"
  "net"
  "encoding/binary"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/ipam"
)

type Validator func(netInfo *danmtypes.DanmNet) error

type ValidatorConfig struct {
  ValidatorMappings []ValidatorMapping
}

type ValidatorMapping struct {
  ApiType string
  Validators []Validator
}

const (
  MaxNidLength = 12
)

var (
  DanmNetMapping = ValidatorMapping {
    ApiType: "DanmNet",
    Validators: []Validator{validateIpv4Fields,validateIpv6Fields,validateAllocationPool,validateVids,validateNetworkId},
  }
  danmValidationConfig = ValidatorConfig {
    ValidatorMappings: []ValidatorMapping{DanmNetMapping},
  }
)

func validateIpv4Fields(dnet *danmtypes.DanmNet) error {
  return validateIpFields(dnet.Spec.Options.Cidr, dnet.Spec.Options.Routes)
}

func validateIpv6Fields(dnet *danmtypes.DanmNet) error {
  return validateIpFields(dnet.Spec.Options.Net6, dnet.Spec.Options.Routes6)
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
  for _, gw := range routes {
    if !ipnet.Contains(net.ParseIP(gw)) {
      return errors.New("Specified GW address:" + gw + " is not part of CIDR:" + cidr)
    }
  }
  return nil
}

func validateAllocationPool(dnet *danmtypes.DanmNet) error {
  cidr := dnet.Spec.Options.Cidr
  apStart := dnet.Spec.Options.Pool.Start
  apEnd := dnet.Spec.Options.Pool.End
  if cidr == "" {
    if apStart != "" || apEnd != "" {
      return errors.New("Allocation pool cannot be defined without CIDR!")
    }
    return nil
  }
  _, ipnet, err := net.ParseCIDR(dnet.Spec.Options.Cidr)
  if err != nil {
    return errors.New("Invalid CIDR parameter: " + dnet.Spec.Options.Cidr)
  }
  if dnet.Spec.Options.Pool.Start == "" {
    dnet.Spec.Options.Pool.Start = (ipam.Int2ip(ipam.Ip2int(ipnet.IP) + 1)).String()
  }
  if dnet.Spec.Options.Pool.End == "" {
    dnet.Spec.Options.Pool.End = (ipam.Int2ip(ipam.Ip2int(getBroadcastAddress(ipnet)) - 1)).String()
  }
  if !ipnet.Contains(net.ParseIP(dnet.Spec.Options.Pool.Start)) || !ipnet.Contains(net.ParseIP(dnet.Spec.Options.Pool.End)) {
    return errors.New("Allocation pool is outside of defined CIDR")
  }
  if ipam.Ip2int(net.ParseIP(dnet.Spec.Options.Pool.End)) - ipam.Ip2int(net.ParseIP(dnet.Spec.Options.Pool.Start)) <= 0 {
    return errors.New("Allocation pool start:" + dnet.Spec.Options.Pool.Start + " is bigger than end:" + dnet.Spec.Options.Pool.End)
  }
  return nil
}

func getBroadcastAddress(subnet *net.IPNet) (net.IP) {
  ip := make(net.IP, len(subnet.IP.To4()))
  binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(subnet.IP.To4())|^binary.BigEndian.Uint32(net.IP(subnet.Mask).To4()))
  return ip
}

func validateVids(dnet *danmtypes.DanmNet) error {
  isVlanDefined := (dnet.Spec.Options.Vlan!=0)
  isVxlanDefined := (dnet.Spec.Options.Vxlan!=0)
  if isVlanDefined && isVxlanDefined {
    return errors.New("VLAN ID and VxLAN ID parameters are mutually exclusive")
  }
  return nil
}

func validateNetworkId(dnet *danmtypes.DanmNet) error {
  if dnet.Spec.NetworkID == "" {
    return errors.New("Spec.NetworkID mandatory parameter is missing!")
  }
  if len(dnet.Spec.NetworkID) > MaxNidLength {
    return errors.New("Spec.NetworkID cannot be longer than 12 characters (otherwise VLAN and VxLAN host interface creation might fail)!")
  } 
  return nil
}
