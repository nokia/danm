package ipam_test

import (
  "os"
  "strings"
  "testing"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/ipam"
  stubs "github.com/nokia/danm/test/stubs/danm"
  "github.com/nokia/danm/test/utils"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testNets = []danmtypes.DanmNet {
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "l2"},Spec: danmtypes.DanmNetSpec{NetworkID: "l2"}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "cidr"},Spec: danmtypes.DanmNetSpec{NetworkID: "cidr", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "fullIpv4"},Spec: danmtypes.DanmNetSpec{NetworkID: "fullIpv4", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.0/30"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "net6"},Spec: danmtypes.DanmNetSpec{NetworkID: "net6", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64", Cidr: "192.168.1.64/26",}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "smallNet6"},Spec: danmtypes.DanmNetSpec{NetworkID: "smallNet6", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/69"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "conflict"},Spec: danmtypes.DanmNetSpec{NetworkID: "conflict", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "conflicterror"},Spec: danmtypes.DanmNetSpec{NetworkID: "conflicterror", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "conflictFree"},Spec: danmtypes.DanmNetSpec{NetworkID: "conflictFree", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "conflicterrorFree"},Spec: danmtypes.DanmNetSpec{NetworkID: "conflicterrorFree", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
  danmtypes.DanmNet {ObjectMeta: meta_v1.ObjectMeta {Name: "error"},Spec: danmtypes.DanmNetSpec{NetworkID: "error", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
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
  {"noIpsRequested", 0, "", "", "", "", false, true},
  {"noneIPv4", 0, "none", "", "", "", false, true},
  {"noneIPv6", 0, "", "none", "", "", false, true},
  {"noneDualStack", 0, "none", "none", "", "", false, true},
  {"dynamicErrorIPv4", 0, "dynamic", "", "", "", true, false},
  {"dynamicErrorIPv6", 0, "", "dynamic", "", "", true, false},
  {"dynamicErrorDualStack", 0, "dynamic", "dynamic", "", "", true, false},
  {"dynamicIPv4Success", 1, "dynamic", "", "192.168.1.65/26", "", false, true},
  {"dynamicIPv4Exhausted", 2, "dynamic", "", "", "", true, false},
  {"staticInvalidIPv4", 2, "hululululu", "", "", "", true, false},
  {"staticInvalidNoCidrIPv4", 2, "192.168.1.1", "", "", "", true, false},
  {"staticL2IPv4", 0, "192.168.1.1/26", "", "", "", true, false},
  {"staticNetmaskMismatchIPv4", 2, "192.168.1.1/32", "", "", "", true, false},
  {"staticAlreadyUsedIPv4", 2, "192.168.1.2/30", "", "", "", true, false},
  {"staticSuccessLastIPv4", 1, "192.168.1.126/26", "", "192.168.1.126/26", "", false, true},
  {"staticSuccessFirstIPv4", 1, "192.168.1.65/26", "", "192.168.1.65/26", "", false, true},
  {"staticFailAfterLastIPv4", 1, "192.168.1.127/26", "", "", "", true, false},
  {"staticFailBeforeFirstIPv4", 1, "192.168.1.64/26", "", "", "", true, false},
  {"dynamicIPv6Success", 3, "", "dynamic", "", "2a00:8a00:a000:1193", false, true},
  {"dynamicNotSupportedCidrSizeIPv6", 4, "", "dynamic", "", "", true, false}, //basically anything smaller than /64. Restriction must be fixed some day!
  {"staticL2IPv6", 2, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "", "", true, false},
  {"staticInvalidIPv6", 3, "", "2a00:8a00:a000:1193:hulu:lulu:lulu:lulu/64", "", "", true, false},
  {"staticNetmaskMismatchIPv6", 3, "", "2a00:8a00:a000:2193:f816:3eff:fe24:e348/64", "", "", true, false},
  {"staticIPv6Success", 3, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false, false},
  {"dynamicDualStackSuccess", 3, "dynamic", "dynamic", "192.168.1.65/26", "2a00:8a00:a000:1193", false, true},
  {"staticDualStackSuccess", 3, "192.168.1.115/26", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "192.168.1.115/26", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false, true},
  {"resolvedConflictDuringUpdate", 5, "", "dynamic", "", "2a00:8a00:a000:1193", false, true},
  {"unresolvedConflictDuringUpdate", 6, "", "dynamic", "", "", true, false},
  {"errorUpdate", 9, "", "dynamic", "", "", true, false},
}

var freeTcs = []struct {
  netName string
  netIndex int
  allocatedIp string
  isErrorExpected bool
}{
  {"l2Network", 0, "192.168.1.126/26", false},
  {"noAssignedIp", 1, "", false},
  {"successfulFree", 2, "192.168.1.2/30", false},
  {"noNetmask", 2, "192.168.1.2", false},
  {"outOfRange", 2, "192.168.1.10/30", false},
  {"invalidIp", 2, "192.168.hululu/30", false},
  {"ipv6", 2, "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false},
  {"resolvedConflictDuringUpdate", 7, "192.168.1.69/26", false},
  {"unresolvedConflictDuringUpdate", 8, "192.168.1.69/26", true},
  {"errorUpdate", 9, "192.168.1.69/26", true},
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
  err := utils.SetupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range reserveTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.expectedIp4,true,tc.netIndex)
      testArtifacts := stubs.TestArtifacts{TestNets: testNets, ReservedIps: ips}
      netClientStub := stubs.NewClientSetStub(testArtifacts)
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
  err := utils.SetupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range freeTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.allocatedIp,false,tc.netIndex)
      testArtifacts := stubs.TestArtifacts{TestNets: testNets, ReservedIps: ips}
      netClientStub := stubs.NewClientSetStub(testArtifacts)
      err := ipam.Free(netClientStub, testNets[tc.netIndex], tc.allocatedIp)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%v does not match with expectation", err)
        return
      }
    })
  }
}

func TestGarbageCollectIps(t *testing.T) {
  err := utils.SetupAllocationPools(testNets)
  if err != nil {
    t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
  }
  for _, tc := range gcTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ips := createExpectedAllocationsList(tc.allocatedIp4,false,tc.netIndex)
      testArtifacts := stubs.TestArtifacts{TestNets: testNets, ReservedIps: ips}
      netClientStub := stubs.NewClientSetStub(testArtifacts)
      ipam.GarbageCollectIps(netClientStub, &testNets[tc.netIndex], tc.allocatedIp4, tc.allocatedIp6)
    })
  }
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

