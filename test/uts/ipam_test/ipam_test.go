package ipam_test

import (
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
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "emptyVal", }},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "falseVal", Validation: false}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "trueVal", Validation: true}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "cidr", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "fullIpv4", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.0/30"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "net6", Validation: true, Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64", Cidr: "192.168.1.64/26",}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "smallNet6", Validation: true, Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/69"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "conflict", Validation: true, Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "conflicterror", Validation: true, Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "conflictFree", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "conflicterrorFree", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "error", Validation: true, Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
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
  {"dynamicIPv6Success", 5, "", "dynamic", "", "2a00:8a00:a000:1193", false, true},
  {"dynamicNotSupportedCidrSizeIPv6", 6, "", "dynamic", "", "", true, false}, //basically anything smaller than /64. Restriction must be fixed some day!
  {"staticL2IPv6", 4, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "", "", true, false},
  {"staticInvalidIPv6", 5, "", "2a00:8a00:a000:1193:hulu:lulu:lulu:lulu/64", "", "", true, false},
  {"staticNetmaskMismatchIPv6", 5, "", "2a00:8a00:a000:2193:f816:3eff:fe24:e348/64", "", "", true, false},
  {"staticIPv6Success", 5, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false, false},
  {"dynamicDualStackSuccess", 5, "dynamic", "dynamic", "192.168.1.65/26", "2a00:8a00:a000:1193", false, true},
  {"staticDualStackSuccess", 5, "192.168.1.115/26", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "192.168.1.115/26", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false, true},
  {"resolvedConflictDuringUpdate", 7, "", "dynamic", "", "2a00:8a00:a000:1193", false, true},
  {"unresolvedConflictDuringUpdate", 8, "", "dynamic", "", "", true, false},
  {"errorUpdate", 11, "", "dynamic", "", "", true, false},
}

var freeTcs = []struct {
  netName string
  netIndex int
  allocatedIp string
  isErrorExpected bool
}{
  {"l2Network", 2, "192.168.1.126/26", false},
  {"noAssignedIp", 3, "", false},
  {"successfulFree", 4, "192.168.1.2/30", false},
  {"noNetmask", 4, "192.168.1.2", false},
  {"outOfRange", 4, "192.168.1.10/30", false},
  {"invalidIp", 4, "192.168.hululu/30", false},
  {"ipv6", 4, "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false},
  {"resolvedConflictDuringUpdate", 9, "192.168.1.69/26", false},
  {"unresolvedConflictDuringUpdate", 10, "192.168.1.69/26", true},
  {"errorUpdate", 11, "192.168.1.69/26", true},
}

var gcTcs = []struct {
  netName string
  netIndex int
  allocatedIp4 string
  allocatedIp6 string
}{
  {"ip4OnlyGc", 5, "192.168.1.110/26", ""},
  {"ip6OnlyGc", 5, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64"},
  {"dualStackGc", 5, "192.168.1.108/26", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64"},
}

func TestReserve(t *testing.T) {
  err := setupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range reserveTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.expectedIp4,true,tc.netIndex)
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
      if !strings.HasPrefix(ip6,tc.expectedIp6) {
        t.Errorf("Allocated IP6 address:%s does not prefixed with the expected CIDR:%s", ip6, tc.expectedIp6)
      }
    })
  }
}

func TestFree(t *testing.T) {
  err := setupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range freeTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.allocatedIp,false,tc.netIndex)
      netClientStub := stubs.NewClientSetStub(testNets, nil, ips)
      err := ipam.Free(netClientStub, testNets[tc.netIndex], tc.allocatedIp)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%v does not match with expectation", err)
        return
      }
    })
  }
}

func TestGarbageCollectIps(t *testing.T) {
  err := setupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range gcTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.allocatedIp4,false,tc.netIndex)
      netClientStub := stubs.NewClientSetStub(testNets, nil, ips)
      ipam.GarbageCollectIps(netClientStub, &testNets[tc.netIndex], tc.allocatedIp4, tc.allocatedIp6)
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
        exhaustNetwork(&net)
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

func createExpectedAllocationsList(ip string, isExpectedToBeSet bool, index int) []stubs.ReservedIpsList {
  var ips []stubs.ReservedIpsList
  if ip != "" {
    strippedIp := strings.Split(ip, "/")
    reservation := stubs.Reservation {Ip: strippedIp[0], Set: isExpectedToBeSet,}
    expectedAllocation := stubs.ReservedIpsList{NetworkId: testNets[index].Spec.NetworkID, Reservations: []stubs.Reservation {reservation,},}
    ips = append(ips, expectedAllocation)
  }
  return ips
}

func TestMain(m *testing.M) {
  code := m.Run()
  os.Exit(code)
}

