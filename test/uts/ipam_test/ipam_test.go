package ipam_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	danmtypes "github.com/danm-cni/danm/crd/apis/danm/v1"
	"github.com/danm-cni/danm/pkg/ipam"
	"github.com/danm-cni/danm/pkg/netcontrol"
	stubs "github.com/danm-cni/danm/test/stubs/danm"
	"github.com/danm-cni/danm/test/utils"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testNets = []danmtypes.DanmNet{
	{ObjectMeta: meta_v1.ObjectMeta{Name: "l2"}, Spec: danmtypes.DanmNetSpec{NetworkID: "l2"}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "cidr"},
		TypeMeta: meta_v1.TypeMeta{Kind: "ClusterNetwork"},
		Spec:     danmtypes.DanmNetSpec{NetworkID: "cidr", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}},
		Status: danmtypes.DanmNetStatus{
			ReservedIPs: []danmtypes.NetRipStatus{
				{Namespace: "rip-test", Objects: []string{"rip1", "rip2"}},
			}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullIpv4"}, Spec: danmtypes.DanmNetSpec{NetworkID: "fullIpv4", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.0/30"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "net6"}, Spec: danmtypes.DanmNetSpec{NetworkID: "net6", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64", Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "smallNet6"}, Spec: danmtypes.DanmNetSpec{NetworkID: "smallNet6", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/120"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "conflict"}, Spec: danmtypes.DanmNetSpec{NetworkID: "conflict", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "conflicterror"}, Spec: danmtypes.DanmNetSpec{NetworkID: "conflicterror", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullconflictFree"}, Spec: danmtypes.DanmNetSpec{NetworkID: "fullconflictFree", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullconflicterrorFree"}, Spec: danmtypes.DanmNetSpec{NetworkID: "fullconflicterrorFree", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullerror"}, Spec: danmtypes.DanmNetSpec{NetworkID: "error", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "error"}, Spec: danmtypes.DanmNetSpec{NetworkID: "error", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "staticFirst"}, Spec: danmtypes.DanmNetSpec{NetworkID: "cidr", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullinitv6"}, Spec: danmtypes.DanmNetSpec{NetworkID: "net6", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Net6: "2a00:8a00:a000:1193::/64"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "v4RestrictedPool"}, Spec: danmtypes.DanmNetSpec{NetworkID: "v4RestrictedPool", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80"}}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "v4RestrictedPoolWithLastIp"}, Spec: danmtypes.DanmNetSpec{NetworkID: "v4RestrictedPoolWithLastIp", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80", LastIp: "192.168.1.80"}}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "nogws"}, Spec: danmtypes.DanmNetSpec{NetworkID: "nogws", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Net6: "2a00:8a00:a000:1193::/64"}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "gws"}, Spec: danmtypes.DanmNetSpec{NetworkID: "gws", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Net6: "2a00:8a00:a000:1193::/64", Routes: map[string]string{"10.20.20.0/24": "192.168.1.65"}, Routes6: map[string]string{"2a00:8a00:a000:1194:/64": "2a00:8a00:a000:1193::1"}}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "fullinitv6Gws"}, Spec: danmtypes.DanmNetSpec{NetworkID: "gwsReserved", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Net6: "2a00:8a00:a000:1193::/64", Routes: map[string]string{"10.20.20.0/24": "192.168.1.65"}, Routes6: map[string]string{"2a00:8a00:a000:1194:/64": "2a00:8a00:a000:1193::1"}}}},
	{ObjectMeta: meta_v1.ObjectMeta{Name: "lastIpCidr"}, Spec: danmtypes.DanmNetSpec{NetworkID: "lastIpCidr", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80", LastIp: "192.168.1.79/26"}}}},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "superDualStack"},
		TypeMeta:   meta_v1.TypeMeta{Kind: "ClusterNetwork"},
		Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "superDualStack",
			Options: danmtypes.DanmNetOption{
				Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80", LastIp: "192.168.1.72/26"},
				Net6: "2001:db8:85a3::8a2e:370:7334/108", Pool6: danmtypes.IpPoolV6{Cidr: "2001:db8:85a3::8a2e:370:7334/109", IpPool: danmtypes.IpPool{Start: "2001:db8:85a3::8a2e:370:7340", End: "2001:db8:85a3::8a2e:370:7350"}}}},
		Status: danmtypes.DanmNetStatus{
			ReservedIPs: []danmtypes.NetRipStatus{
				{Namespace: "rip-test", Objects: []string{"rip1", "rip2"}},
			}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "ripTest"},
		TypeMeta:   meta_v1.TypeMeta{Kind: "ClusterNetwork"},
		Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "ripTest",
			Options: danmtypes.DanmNetOption{
				Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80", LastIp: "192.168.1.72/26"},
				Net6: "2001:db8:85a3::8a2e:370:7334/109", Pool6: danmtypes.IpPoolV6{IpPool: danmtypes.IpPool{Start: "2001:db8:85a3::8a2e:370:7340", End: "2001:db8:85a3::8a2e:370:7350"}}}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "fullRipTest"},
		TypeMeta:   meta_v1.TypeMeta{Kind: "ClusterNetwork"},
		Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "fullRipTest",
			Options: danmtypes.DanmNetOption{
				Cidr: "192.168.1.64/26", Pool: danmtypes.IpPool{Start: "192.168.1.70", End: "192.168.1.80", LastIp: "192.168.1.72/26"},
				Net6: "2001:db8:85a3::8a2e:370:7334/109", Pool6: danmtypes.IpPoolV6{IpPool: danmtypes.IpPool{Start: "2001:db8:85a3::8a2e:370:7340", End: "2001:db8:85a3::8a2e:370:7350"}}}},
	},
}

var testNetStatuses = []danmtypes.DanmNetStatus{
	{
		ReservedIPs: []danmtypes.NetRipStatus{
			{Namespace: "rip-test", Objects: []string{"dualStackOutsideCidr", "dualStackOutsideAlloc", "dualStackInAlloc", "dualStackInAllocShareable"}},
		},
	},
}

var testRips = []danmtypes.ReservedIP{
	{
		TypeMeta:   meta_v1.TypeMeta{Kind: "ReservedIP"},
		ObjectMeta: meta_v1.ObjectMeta{Name: "dualStackOutsideCidr", Namespace: "rip-test"},
		Spec: danmtypes.ReservedIPSpec{
			Network: danmtypes.RipNetworkSelector{Name: "ripTest", Type: "ClusterNetwork"},
			Ips:     []danmtypes.RipIP{{Address: "192.168.1.63", Shareable: false}, {Address: "2001:db8:85a3::8a2e:369:1", Shareable: false}},
		},
	},
	{
		TypeMeta:   meta_v1.TypeMeta{Kind: "ReservedIP"},
		ObjectMeta: meta_v1.ObjectMeta{Name: "dualStackOutsideAlloc", Namespace: "rip-test"},
		Spec: danmtypes.ReservedIPSpec{
			Network: danmtypes.RipNetworkSelector{Name: "ripTest", Type: "ClusterNetwork"},
			Ips:     []danmtypes.RipIP{{Address: "192.168.1.69", Shareable: false}, {Address: "2001:db8:85a3::8a2e:370:7339", Shareable: false}},
		},
	},
	{
		TypeMeta:   meta_v1.TypeMeta{Kind: "ReservedIP"},
		ObjectMeta: meta_v1.ObjectMeta{Name: "dualStackInAlloc", Namespace: "rip-test"},
		Spec: danmtypes.ReservedIPSpec{
			Network: danmtypes.RipNetworkSelector{Name: "ripTest", Type: "ClusterNetwork"},
			Ips:     []danmtypes.RipIP{{Address: "192.168.1.70", Shareable: false}, {Address: "2001:db8:85a3::8a2e:370:7340", Shareable: false}},
		},
	},
	{
		TypeMeta:   meta_v1.TypeMeta{Kind: "ReservedIP"},
		ObjectMeta: meta_v1.ObjectMeta{Name: "dualStackInAllocShareable", Namespace: "rip-test"},
		Spec: danmtypes.ReservedIPSpec{
			Network: danmtypes.RipNetworkSelector{Name: "ripTest", Type: "ClusterNetwork"},
			Ips:     []danmtypes.RipIP{{Address: "192.168.1.71", Shareable: true}, {Address: "2001:db8:85a3::8a2e:370:7341", Shareable: true}},
		},
	},
}

var reserveTcs = []struct {
	netName                   string
	netIndex                  int
	statusIndex               int
	requestedIp4              string
	requestedIp6              string
	expectedIp4               string
	expectedIp6               string
	isErrorExpected           bool
	timesUpdateShouldBeCalled int
	requestNamespace          string
}{
	{"noIpsRequested", 0, -1, "", "", "", "", false, 0, ""},
	{"noneIPv4", 0, -1, "none", "", "none", "", false, 0, ""},
	{"noneIPv6", 0, -1, "", "none", "", "none", false, 0, ""},
	{"noneDualStack", 0, -1, "none", "none", "none", "none", false, 0, ""},
	{"dynamicErrorIPv4", 0, -1, "dynamic", "", "", "", true, 0, ""},
	{"dynamicErrorIPv6", 0, -1, "", "dynamic", "", "", true, 0, ""},
	{"dynamicErrorDualStack", 0, -1, "dynamic", "dynamic", "", "", true, 0, ""},
	{"dynamicIPv4Success", 1, -1, "dynamic", "", "192.168.1.65/26", "", false, 1, ""},
	{"dynamicIPv4Exhausted", 2, -1, "dynamic", "", "", "", true, 0, ""},
	{"staticInvalidIPv4", 2, -1, "hululululu", "", "", "", true, 0, ""},
	{"staticInvalidNoCidrIPv4", 2, -1, "192.168.1.1", "", "", "", true, 0, ""},
	{"staticL2IPv4", 0, -1, "192.168.1.1/26", "", "", "", true, 0, ""},
	{"staticNetmaskMismatchIPv4", 2, -1, "192.168.1.1/32", "", "", "", true, 0, ""},
	{"staticAlreadyUsedIPv4", 2, -1, "192.168.1.2/30", "", "", "", true, 0, ""},
	{"staticSuccessLastIPv4", 1, -1, "192.168.1.126/26", "", "192.168.1.126/26", "", false, 1, ""},
	{"staticSuccessFirstIPv4", 11, -1, "192.168.1.65/26", "", "192.168.1.65/26", "", false, 1, ""},
	{"staticFailAfterLastIPv4", 1, -1, "192.168.1.127/26", "", "", "", true, 0, ""},
	{"staticFailBeforeFirstIPv4", 1, -1, "192.168.1.64/26", "", "", "", true, 0, ""},
	{"dynamicIPv6Success", 3, -1, "", "dynamic", "", "2a00:8a00:a000:1193::1/64", false, 1, ""},
	{"dynamicIPv6SmallCidrSizeSuccess", 4, -1, "", "dynamic", "", "2a00:8a00:a000:1193::1/120", false, 1, ""},
	{"staticL2IPv6", 2, -1, "", "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", "", "", true, 0, ""},
	{"staticInvalidIPv6", 3, -1, "", "2a00:8a00:a000:1193:hulu:lulu:lulu:lulu/64", "", "", true, 0, ""},
	{"staticIPv/OutsideOfAllocCidr", 3, -1, "", "2a00:8a00:a000:2193:f816:3eff:fe24:e348/64", "", "", true, 0, ""},
	{"staticIPv6Success", 3, -1, "", "2a00:8a00:a000:1193::3e:1010", "", "2a00:8a00:a000:1193::3e:1010/64", false, 1, ""},
	{"dynamicDualStackSuccess", 3, -1, "dynamic", "dynamic", "192.168.1.65/26", "2a00:8a00:a000:1193::2/64", false, 1, ""},
	{"staticDualStackSuccess", 3, -1, "192.168.1.115", "2a00:8a00:a000:1193::3e:2002", "192.168.1.115/26", "2a00:8a00:a000:1193::3e:2002/64", false, 1, ""},
	{"resolvedConflictDuringUpdate", 5, -1, "dynamic", "", "192.168.1.65/26", "", false, 2, ""},
	{"unresolvedConflictAfterUpdate", 6, -1, "dynamic", "", "", "", true, 1, ""},
	{"errorUpdate", 10, -1, "dynamic", "", "", "", true, 1, ""},
	{"dyanmicV4FromAllocationPool", 13, -1, "dynamic", "", "192.168.1.70/26", "", false, 1, ""},
	{"dyanmicV4FromAllocationPoolWithLastIpSet", 14, -1, "dynamic", "", "192.168.1.70/26", "", false, 1, ""},
	{"dyanmicV4FirstIpIsNotAGw", 15, -1, "dynamic", "", "192.168.1.65/26", "", false, 1, ""},
	{"dyanmicV6FirstIpIsNotAGw", 15, -1, "", "dynamic", "", "2a00:8a00:a000:1193::1/64", false, 1, ""},
	{"dyanmicV4FirstIpIsGw", 16, -1, "dynamic", "", "192.168.1.66/26", "", false, 1, ""},
	{"dyanmicV6FirstIpIsGw", 16, -1, "", "dynamic", "", "2a00:8a00:a000:1193::2/64", false, 1, ""},
	{"staticV4GwSuccess", 16, -1, "192.168.1.65/26", "", "192.168.1.65/26", "", false, 1, ""},
	{"staticV6GwSuccess", 16, -1, "", "2a00:8a00:a000:1193::1", "", "2a00:8a00:a000:1193::1/64", false, 1, ""},
	{"staticV4GwFailIpReserved", 17, -1, "192.168.1.65/26", "", "", "", true, 0, ""},
	{"staticV6GwFailIpReserved", 17, -1, "", "2a00:8a00:a000:1193::1", "", "", true, 0, ""},
	{"dyanmicV4FromAllocationPoolWithLastIpCidr", 18, -1, "dynamic", "", "192.168.1.80/26", "", false, 1, ""},
	{"ripError", 19, -1, "dynamic", "", "", "", true, 0, "rip-test"},
	{"ripV4NotExistingRipAsked", 20, 0, "doesntExist", "", "", "", true, 0, "rip-test"},
	{"ripV6NotExistingRipAsked", 20, 0, "", "doesntExist", "", "", true, 0, "rip-test"},
	{"ripV4OutsideNetCidr", 20, 0, "dualStackOutsideCidr", "", "", "", true, 0, "rip-test"},
	{"ripV6OutsideNetCidr", 20, 0, "", "dualStackOutsideCidr", "", "", true, 0, "rip-test"},
	{"ripV4OutsideAlloc", 20, 0, "dualStackOutsideAlloc", "", "192.168.1.69/26", "", false, 1, "rip-test"},
	{"ripV6OutsideAlloc", 20, 0, "", "dualStackOutsideAlloc", "", "2001:db8:85a3::8a2e:370:7339/109", false, 1, "rip-test"},
	{"ripDsInsideAlloc", 20, 0, "dualStackInAlloc", "dualStackInAlloc", "192.168.1.70/26", "2001:db8:85a3::8a2e:370:7340/109", false, 1, "rip-test"},
	{"ripV4InsideAllocExhausted", 21, 0, "dualStackInAlloc", "", "", "", true, 0, "rip-test"},
	{"ripV6InsideAllocExhausted", 21, 0, "", "dualStackInAlloc", "", "", true, 0, "rip-test"},
	{"shareableRipV4InsideAllocExhausted", 21, 0, "dualStackInAllocShareable", "", "192.168.1.71/26", "", false, 0, "rip-test"},
	{"shareableRipV6InsideAllocExhausted", 21, 0, "", "dualStackInAllocShareable", "", "2001:db8:85a3::8a2e:370:7341/109", false, 0, "rip-test"},
}

var freeTcs = []struct {
	netName                   string
	netIndex                  int
	allocatedIp               string
	isErrorExpected           bool
	timesUpdateShouldBeCalled int
}{
	{"l2Network", 0, "192.168.1.126/26", false, 0},
	{"noAssignedIp", 1, "", false, 0},
	{"successfulFree", 2, "192.168.1.2/30", false, 1},
	{"noNetmask", 2, "192.168.1.2", false, 0},
	{"outOfRange", 2, "192.168.1.10/30", false, 0},
	{"invalidIp", 2, "192.168.hululu/30", false, 0},
	{"ipv6OutsideOfCidr", 3, "2a00:8a00:a000:1193:f816:3eff:fe24:e348/64", false, 0},
	{"resolvedConflictDuringUpdate", 7, "192.168.1.69/26", false, 2},
	{"unresolvedConflictAfterUpdate", 8, "192.168.1.69/26", true, 1},
	{"errorUpdate", 9, "192.168.1.69/26", true, 1},
	{"ipv6SuccesfulFree", 12, "2a00:8a00:a000:1193::1/106", false, 1},
}

var gcTcs = []struct {
	netName      string
	netIndex     int
	allocatedIp4 string
	allocatedIp6 string
}{
	{"ip4OnlyGc", 12, "192.168.1.110/26", ""},
	{"ip6OnlyGc", 12, "", "2a00:8a00:a000:1193::1/106"},
	{"dualStackGc", 12, "192.168.1.115", "2a00:8a00:a000:1193::5"},
	{"emptyV4FilledV6", 12, "", "2a00:8a00:a000:1193::5"},
	{"noneV4FilledV6", 12, "none", "2a00:8a00:a000:1193::5"},
	{"filledV4EmptyV6", 12, "192.168.1.115", ""},
	{"filledV4NoneV6", 12, "192.168.1.115", "none"},
}

var wasAllocatedTcs = []struct {
	tcName            string
	netName           string
	allocatedIp       string
	expectedAllocated bool
}{
	{"empty", "superDualStack", "", true},
	{"none", "superDualStack", "none", true},
	{"v4IpForm", "superDualStack", "192.168.1.71", true},
	{"v4CidrForm", "superDualStack", "192.168.1.71/26", true},
	{"v6IpForm", "superDualStack", "2001:db8:85a3::8a2e:370:7341", true},
	{"v6CidrForm", "superDualStack", "2001:db8:85a3::8a2e:370:7341/109", true},
	{"v4OutsideDynamicRange", "superDualStack", "192.168.1.81", true},
	{"v6OutsideDynamicRange", "superDualStack", "2001:db8:85a3::8a2e:370:8000", true},
	{"v4OutsideNetworkCidr", "superDualStack", "192.168.1.63", false},
	{"v6OutsideNetworkCidr", "superDualStack", "2001:db8:85a3::8a2e:369:0", false},
	{"v4L2", "l2", "192.168.1.81", false},
	{"v6l2", "l2", "2001:db8:85a3::8a2e:370:8000", false},
}

func TestReserve(t *testing.T) {
	err := utils.SetupAllocationPools(testNets)
	if err != nil {
		t.Errorf("Allocation pool for testnets could not be set-up because:%v", err)
	}
	for _, tc := range reserveTcs {
		t.Run(tc.netName, func(t *testing.T) {
			var ips []utils.ReservedIpsList
			ips = utils.AppendIpToExpectedAllocsList(ips, tc.expectedIp4, true, testNets[tc.netIndex].Spec.NetworkID)
			ips = utils.AppendIpToExpectedAllocsList(ips, tc.expectedIp6, true, testNets[tc.netIndex].Spec.NetworkID)
			if tc.statusIndex >= 0 {
				testNets[tc.netIndex].Status = testNetStatuses[tc.statusIndex]
			}
			testArtifacts := utils.TestArtifacts{TestNets: testNets, TestRips: testRips, ReservedIps: ips}
			if strings.Contains(tc.netName, "ripError") {
				testArtifacts.IsRipListErrorExpected = true
			}
			netClientStub := stubs.NewClientSetStub(testArtifacts)
			//The request always comes from the Pod's namespace, which is not necessarily the same as the network's own
			requestNamespace := tc.requestNamespace
			if requestNamespace == "" {
				requestNamespace = testNets[tc.netIndex].Namespace
			}
			ip4, ip6, err := ipam.Reserve(netClientStub, testNets[tc.netIndex], tc.requestedIp4, tc.requestedIp6, requestNamespace)
			if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
				t.Errorf("Received error:%v does not match with expectation", err)
				return
			}
			if ip4 != tc.expectedIp4 {
				t.Errorf("Allocated IP4 address:%s does not match with expected:%s", ip4, tc.expectedIp4)
			}
			if ip6 != tc.expectedIp6 {
				t.Errorf("Allocated IP6 address:%s does not match with the expected:%s", ip6, tc.expectedIp6)
			}
			var timesUpdateWasCalled int
			if netClientStub.DanmClient.NetClient != nil && (testNets[tc.netIndex].Kind == netcontrol.DanmNetKind || testNets[tc.netIndex].Kind == "") {
				timesUpdateWasCalled = netClientStub.DanmClient.NetClient.TimesUpdateWasCalled
			}
			if netClientStub.DanmClient.CnetClient != nil && testNets[tc.netIndex].Kind == netcontrol.ClusterNetworkKind {
				timesUpdateWasCalled = netClientStub.DanmClient.CnetClient.TimesUpdateWasCalled
			}
			if tc.timesUpdateShouldBeCalled != timesUpdateWasCalled {
				t.Errorf("%s", "Network manifest should have been updated:"+strconv.Itoa(tc.timesUpdateShouldBeCalled)+" times, but it happened:"+strconv.Itoa(timesUpdateWasCalled)+" times instead")
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
			var ips []utils.ReservedIpsList
			ips = utils.AppendIpToExpectedAllocsList(ips, tc.allocatedIp, false, testNets[tc.netIndex].Spec.NetworkID)
			testArtifacts := utils.TestArtifacts{TestNets: testNets, ReservedIps: ips}
			netClientStub := stubs.NewClientSetStub(testArtifacts)
			err := ipam.Free(netClientStub, testNets[tc.netIndex], tc.allocatedIp)
			if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
				t.Errorf("Received error:%v does not match with expectation", err)
				return
			}
			var timesUpdateWasCalled int
			if netClientStub.DanmClient.NetClient != nil {
				timesUpdateWasCalled = netClientStub.DanmClient.NetClient.TimesUpdateWasCalled
			}
			if tc.timesUpdateShouldBeCalled != timesUpdateWasCalled {
				t.Errorf("%s", "Network manifest should have been updated:"+strconv.Itoa(tc.timesUpdateShouldBeCalled)+" times, but it happened:"+strconv.Itoa(timesUpdateWasCalled)+" times instead")
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
			var ips []utils.ReservedIpsList
			ips = utils.AppendIpToExpectedAllocsList(ips, tc.allocatedIp4, false, testNets[tc.netIndex].Spec.NetworkID)
			ips = utils.AppendIpToExpectedAllocsList(ips, tc.allocatedIp6, false, testNets[tc.netIndex].Spec.NetworkID)
			testArtifacts := utils.TestArtifacts{TestNets: testNets, ReservedIps: ips}
			netClientStub := stubs.NewClientSetStub(testArtifacts)
			err := ipam.GarbageCollectIps(netClientStub, &testNets[tc.netIndex], tc.allocatedIp4, tc.allocatedIp6)
			if err != nil {
				t.Errorf("Received error:%v does not match with expectation", err)
			}
		})
	}
}

func TestWasIpAllocatedByDanm(t *testing.T) {
	for _, tc := range wasAllocatedTcs {
		t.Run(tc.tcName, func(t *testing.T) {
			var testNet danmtypes.DanmNet
			for _, net := range testNets {
				if net.Name == tc.netName {
					testNet = net
					break
				}
			}
			wasAllocated := ipam.WasIpAllocatedByDanm(tc.allocatedIp, &testNet)
			if wasAllocated != tc.expectedAllocated {
				t.Errorf("Received allocation verdict:%v does not match with expectation:%v", wasAllocated, tc.expectedAllocated)
			}
		})
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
