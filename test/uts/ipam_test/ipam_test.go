package ipam_test

import (
  "log"
  "net"
  "os"
  "strings"
  "testing"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/netcontrol"
  "github.com/nokia/danm/test/stubs"
)

var testNets = []danmtypes.DanmNet {
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "emptyVal", } },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "falseVal", Validation: false} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "trueVal", Validation: true} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "cidr", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "fullIpv4", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.0/30"}} },

}

var reserveTcs = []struct {
  netName string
  netIndex int
  requestedIp4 string
  requestedIp6 string
  expectedIp4 string
  expectedIp6 string
  isErrorExpected bool
  isMacExpected bool
}{
  {"emptyVal", 0, "", "", "", "", true, false},
  {"falseVal", 1, "", "", "", "", true, false},
  {"noIpsRequested", 2, "", "", "", "", false, true},
  {"noneIPv4", 2, "none", "", "", "", false, true},
  {"noneIPv6", 2, "", "none", "", "", false, true},
  {"noneDualStack", 2, "none", "none", "", "", false, true},
  {"dynamicErrorIPv4", 2, "dynamic", "", "", "", true, false},
  {"dynamicErrorIPv6", 2, "", "dynamic", "", "", true, false},
  {"dynamicErrorDualStack", 2, "dynamic", "dynamic", "", "", true, false},
  {"dynamicIPv4Success", 3, "dynamic", "", "192.168.1.65/26", "", false, true},
  {"dynamicIPv4Exhausted", 4, "dynamic", "", "", "", true, false},
  {"staticInvalidIPv4", 4, "hululululu", "", "", "", true, false},
  {"staticInvalidNoCidrIPv4", 4, "192.168.1.1", "", "", "", true, false},
  {"staticL2IPv4", 2, "192.168.1.1/26", "", "", "", true, false},
  {"staticNetmaskMismatchIPv4", 4, "192.168.1.1/32", "", "", "", true, false},
  {"staticAlreadyUsedIPv4", 4, "192.168.1.2/30", "", "", "", true, false},
  {"staticSuccessLastIPv4", 3, "192.168.1.126/26", "", "192.168.1.126/26", "", false, true},
  {"staticSuccessFirstIPv4", 3, "192.168.1.65/26", "", "192.168.1.65/26", "", false, true},
  {"staticFailAfterLastIPv4", 3, "192.168.1.127/26", "", "", "", true, false},
  {"staticFailBeforeFirstIPv4", 3, "192.168.1.64/26", "", "", "", true, false},
}

func TestReserve(t *testing.T) {
  err := setupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range reserveTcs {
    t.Run(tc.netName, func(t *testing.T) {
      var ips []stubs.ReservedIpsList
      if tc.expectedIp4 != "" {
        strippedId := strings.Split(tc.expectedIp4, "/")
        expectedAllocation := stubs.ReservedIpsList{NetworkId: testNets[tc.netIndex].Spec.NetworkID, Ips: []string {strippedId[0],},}
        ips = append(ips, expectedAllocation)
      }
      netClientStub := stubs.NewClientSetStub(testNets, nil, ips)
      ip4, ip6, mac, err := ipam.Reserve(netClientStub, testNets[tc.netIndex], tc.requestedIp4, tc.requestedIp6)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%v does not match with expectation", err)
        return
      }
      if tc.isMacExpected {
        if mac == "" {
          t.Errorf("MAC address was expected to be returned, however it was not")
        }
      }
      if ip4 != tc.expectedIp4 {
        t.Errorf("Allocated IP4 address:%s does not match with expected:%s", ip4, tc.expectedIp4)
      }
      if ip6 != tc.expectedIp6 {
        t.Errorf("Allocated IP6 address:%s does not match with expected:%s", ip6, tc.expectedIp6)
      }
    })
  }
}

func setupAllocationPools(nets []danmtypes.DanmNet) error {
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
        log.Println("lofaaaasz before:" + net.Spec.Options.Alloc)
        exhaustNetwork(&net)
        log.Println("lofaaaasz after:" + net.Spec.Options.Alloc)
      }
      testNets[index].Spec = net.Spec
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

func TestMain(m *testing.M) {
  code := m.Run() 
  os.Exit(code)
}

