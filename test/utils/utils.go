package utils

import (
  "net"
  "strings"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/netcontrol"
)

func SetupAllocationPools(nets []danmtypes.DanmNet) error {
  for index, net := range nets {
    if net.Spec.Options.Cidr != "" {
      bitArray, err := netcontrol.CreateAllocationArray(&net)
      if err != nil {
        return err
      }
      net.Spec.Options.Alloc = bitArray.Encode()
      err = netcontrol.ValidateAllocationPool(&net)
      if err != nil {
        return err
      }
      if strings.HasPrefix(net.Spec.NetworkID, "full") {
        exhaustNetwork(&net)
      }
      nets[index].Spec = net.Spec
    }
  }
  return nil
}

func exhaustNetwork(netInfo *danmtypes.DanmNet) {
    ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
    _, ipnet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
    ipnetNum := netcontrol.Ip2int(ipnet.IP)
    begin := netcontrol.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.Start)) - ipnetNum
    end := netcontrol.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.End)) - ipnetNum
    for i:=begin;i<=end;i++ {
        ba.Set(uint32(i))
    }
    netInfo.Spec.Options.Alloc = ba.Encode()
}
