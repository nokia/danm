package admit_tests

import (
  "strconv"
  "strings"
  "testing"
  "encoding/json"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/admit"
  stubs "github.com/nokia/danm/test/stubs/danm"
  httpstub "github.com/nokia/danm/test/stubs/http"
  "github.com/nokia/danm/test/utils"
  "k8s.io/api/admission/v1beta1"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
  DnetType = "DanmNet"
  TnetType = "TenantNetwork"
  CnetType = "ClusterNetwork"
)

var validateNetworkTcs = []struct {
  tcName string
  oldNetName string
  newNetName string
  neType string
  opType v1beta1.Operation
  tconf []danmtypes.TenantConfig
  isErrorExpected bool
  expectedPatches []admit.Patch
  timesUpdateShouldBeCalled int
}{
  {"EmptyRequest", "", "", "", "", nil, true, nil, 0},
  {"MalformedOldObject", "malformed", "", "", "", nil, true, nil, 0},
  {"MalformedNewObject", "", "malformed", "", "", nil, true, nil, 0},
  {"ObjectWithInvalidType", "", "invalid-type", "", "", nil, true, nil, 0},
  {"Ipv4RouteWithoutCidrDNet", "", "no-cidr", DnetType, "", nil, true, nil, 0},
  {"Ipv4RouteWithoutCidrTNet", "", "no-cidr", TnetType, "", nil, true, nil, 0},
  {"Ipv4RouteWithoutCidrCNet", "", "no-cidr", CnetType, "", nil, true, nil, 0},
  {"Ipv4InvalidCidrDNet", "", "invalid-cidr", DnetType, "", nil, true, nil, 0},
  {"Ipv4InvalidCidrTNet", "", "invalid-cidr", TnetType, "", nil, true, nil, 0},
  {"Ipv4InvalidCidrCNet", "", "invalid-cidr", CnetType, "", nil, true, nil, 0},
  {"Ipv4GwOutsideCidrDNet", "", "gw-outside-cidr", DnetType, "", nil, true, nil, 0},
  {"Ipv4GwOutsideCidrTNet", "", "gw-outside-cidr", TnetType, "", nil, true, nil, 0},
  {"Ipv4GwOutsideCidrCNet", "", "gw-outside-cidr", CnetType, "", nil, true, nil, 0},
  {"Ipv6RouteWithoutCidrDNet", "", "no-net6", DnetType, "", nil, true, nil, 0},
  {"Ipv6RouteWithoutCidrTNet", "", "no-net6", TnetType, "", nil, true, nil, 0},
  {"Ipv6RouteWithoutCidrCNet", "", "no-net6", CnetType, "", nil, true, nil, 0},
  {"Ipv6InvalidCidrDNet", "", "invalid-net6", DnetType, "", nil, true, nil, 0},
  {"Ipv6InvalidCidrTNet", "", "invalid-net6", TnetType, "", nil, true, nil, 0},
  {"Ipv6InvalidCidrCNet", "", "invalid-net6", CnetType, "", nil, true, nil, 0},
  {"Ipv6GwOutsideCidrDNet", "", "gw-outside-net6", DnetType, "", nil, true, nil, 0},
  {"Ipv6GwOutsideCidrTNet", "", "gw-outside-net6", TnetType, "", nil, true, nil, 0},
  {"Ipv6GwOutsideCidrCNet", "", "gw-outside-net6", CnetType, "", nil, true, nil, 0},
  {"InvalidVidsDNet", "", "invalid-vids", DnetType, "", nil, true, nil, 0},
  {"InvalidVidsCNet", "", "invalid-vids", CnetType, "", nil, true, nil, 0},
  {"MissingNidDNet", "", "missing-nid", DnetType, "", nil, true, nil, 0},
  {"MissingNidCNet", "", "missing-nid", CnetType, "", nil, true, nil, 0},
  {"TooLongNidWithDynamicNeTypeDNet", "", "long-nid", DnetType, "", nil, true, nil, 0},
  {"TooLongNidWithDynamicNeTypeCNet", "", "long-nid", CnetType, "", nil, true, nil, 0},
  {"WithAllowedTenantsDefinedDNet", "", "with-allowed-tenants", DnetType, "", nil, true, nil, 0},
  {"WithAllowedTenantsDefinedTNet", "", "with-allowed-tenants", TnetType, "", nil, true, nil, 0},
  {"SriovWithoutDevicePoolDNet", "", "sriov-without-dp", DnetType, "", nil, true, nil, 0},
  {"SriovWithoutDevicePoolTNet", "", "sriov-without-dp", TnetType, "", nil, true, nil, 0},
  {"SriovWithoutDevicePoolCNet", "", "sriov-without-dp", CnetType, "", nil, true, nil, 0},
  {"SriovWithDeviceDNet", "", "sriov-with-device", DnetType, "", nil, true, nil, 0},
  {"SriovWithDeviceTNet", "", "sriov-with-device", TnetType, "", nil, true, nil, 0},
  {"SriovWithDeviceCNet", "", "sriov-with-device", CnetType, "", nil, true, nil, 0},
  {"SriovWithDevicePlusDpDNet", "", "sriov-with-dp-and-device", DnetType, "", nil, true, nil, 0},
  {"SriovWithDevicePlusDpTNet", "", "sriov-with-dp-and-device", TnetType, "", nil, true, nil, 0},
  {"SriovWithDevicePlusDpCNet", "", "sriov-with-dp-and-device", CnetType, "", nil, true, nil, 0},
  {"IpvlanWithDevicePlusDpDNet", "", "ipvlan-with-dp-and-device", DnetType, "", nil, true, nil, 0},
  {"IpvlanWithDevicePlusDpTNet", "", "ipvlan-with-dp-and-device", TnetType, "", nil, true, nil, 0},
  {"IpvlanWithDevicePlusDpCNet", "", "ipvlan-with-dp-and-device", CnetType, "", nil, true, nil, 0},
  {"AllocDuringCreateDNet", "", "alloc-without-cidr", DnetType, v1beta1.Create, nil, true, nil, 0},
  {"AllocDuringCreateTNet", "", "alloc-without-cidr", TnetType, v1beta1.Create, nil, true, nil, 0},
  {"AllocDuringCreateCNet", "", "alloc-without-cidr", CnetType, v1beta1.Create, nil, true, nil, 0},
  {"AllocationPoolWithoutCidrDNet", "", "alloc-without-cidr", DnetType, v1beta1.Update, nil, true, nil, 0},
  {"AllocationPoolWithoutCidrTNet", "", "alloc-without-cidr", TnetType, v1beta1.Update, nil, true, nil, 0},
  {"AllocationPoolWithoutCidrCNet", "", "alloc-without-cidr", CnetType, v1beta1.Update, nil, true, nil, 0},
  {"AllocationPoolStartOutsideCidrDNet", "", "allocstart-outside-cidr", DnetType, "", nil, true, nil, 0},
  {"AllocationPoolStartOutsideCidrTNet", "", "allocstart-outside-cidr", TnetType, "", nil, true, nil, 0},
  {"AllocationPoolStartOutsideCidrCNet", "", "allocstart-outside-cidr", CnetType, "", nil, true, nil, 0},
  {"AllocationPoolEndOutsideCidrDNet", "", "allocend-outside-cidr", DnetType, "", nil, true, nil, 0},
  {"AllocationPoolEndOutsideCidrTNet", "", "allocend-outside-cidr", TnetType, "", nil, true, nil, 0},
  {"AllocationPoolEndOutsideCidrCNet", "", "allocend-outside-cidr", CnetType, "", nil, true, nil, 0},
  {"AllocationPoolWithoutAnyIpDNet", "", "no-free-ip", DnetType, "", nil, true, nil, 0},
  {"AllocationPoolWithoutAnyIpTNet", "", "no-free-ip", TnetType, "", nil, true, nil, 0},
  {"AllocationPoolWithoutAnyIpCNet", "", "no-free-ip", CnetType, "", nil, true, nil, 0},
  {"CreateWithVlanTNet", "", "tnet-vlan", TnetType, v1beta1.Create, nil, true, nil, 0},
  {"CreateWithVxlanTNet", "", "tnet-vxlan", TnetType, v1beta1.Create, nil, true, nil, 0},
  {"UpdateWithVlanTNet", "", "tnet-vlan", TnetType, v1beta1.Update, nil, true, nil, 0},
  {"UpdateWithVxlanTNet", "", "tnet-vxlan", TnetType, v1beta1.Update, nil, true, nil, 0},
  {"UpdateWithDeviceTNet", "", "tnet-device", TnetType, v1beta1.Update, nil, true, nil, 0},
  {"UpdateWithDevicePoolTNet", "", "tnet-dp", TnetType, v1beta1.Update, nil, true, nil, 0},
  {"NoNeTypeCreateSuccess", "", "no-netype", DnetType, v1beta1.Create, nil, false, neTypeAndAlloc, 0},
  {"NoNeTypeUpdateSuccess", "", "no-netype-update", CnetType, v1beta1.Update, nil, false, onlyNeType, 0},
  {"L2NoPatchSuccess", "", "l2", CnetType, v1beta1.Create, nil, false, nil, 0},
}

var (
  valNets = []danmtypes.DanmNet {
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "malformed"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-type"},
      TypeMeta: meta_v1.TypeMeta {Kind: "DanmEp"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "no-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Routes: map[string]string{"10.20.0.0/24": "10.0.0.1"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.0/a4"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "gw-outside-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Cidr: "10.20.1.0/24", Routes: map[string]string{"10.20.20.0/24": "10.20.1.1", "10.20.30.0/24": "10.20.0.1"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "no-net6"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Routes6: map[string]string{"2a00:8a00:a000:1193::/64": "2a00:8a00:a000:1192::"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-net6"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Net6: "2g00:8a00:a000:1193::/64"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "gw-outside-net6"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64", Routes6: map[string]string{"3a00:8a00:a000:1193::/64": "4a00:8a00:a000:1192::"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-vids"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Vlan: 50, Vxlan:60}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "missing-nid"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "flannel"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "long-nid"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "abcdeftgasdf", Options: danmtypes.DanmNetOption{Vlan: 50}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "with-allowed-tenants"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", AllowedTenants: []string{"tenant1","tenant2"}, Options: danmtypes.DanmNetOption{Vlan: 50}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "sriov-without-dp"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "e2"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "sriov-with-device"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "e2", Options: danmtypes.DanmNetOption{Device: "ens1f1"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "sriov-with-dp-and-device"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "e2", Options: danmtypes.DanmNetOption{DevicePool: "nokia.k8s.io/sriov_ens1f1", Device: "ens1f1"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "ipvlan-with-dp-and-device"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{DevicePool: "nokia.k8s.io/sriov_ens1f1", Device: "ens1f1"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "alloc-without-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Alloc: "gAAAAAAAAAAAAAAE", Pool: danmtypes.IP4Pool{Start: "192.168.1.1"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "allocstart-outside-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Pool: danmtypes.IP4Pool{Start: "192.168.1.63"}, Cidr: "192.168.1.64/26"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "allocend-outside-cidr"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Pool: danmtypes.IP4Pool{End: "192.168.1.128"}, Cidr: "192.168.1.64/26"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "no-free-ip"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Pool: danmtypes.IP4Pool{Start: "192.168.1.127", End: "192.168.1.127"}, Cidr: "192.168.1.64/26"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "tnet-vlan"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Vlan: 50}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "tnet-vxlan"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Vxlan: 50}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "tnet-device"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Device: "ens3"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "tnet-dp"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{DevicePool: "nokia.k8s.io/sriov_ens1f0"}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "no-netype"},
      Spec: danmtypes.DanmNetSpec{NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Routes: map[string]string{"10.20.0.0/24": "192.168.1.64"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "no-netype-update"},
      Spec: danmtypes.DanmNetSpec{NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Alloc: "gAAAAAE=", Pool: danmtypes.IP4Pool{Start: "192.168.1.65",End: "192.168.1.126"}, Cidr: "192.168.1.64/26", Routes: map[string]string{"10.20.0.0/24": "192.168.1.64"}}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "l2"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Device: "ens4"}},
    },
  }
)

var (
  neTypeAndAlloc = []admit.Patch {
    admit.Patch {Path: "/spec/NetworkType"},
    admit.Patch {Path: "/spec/Options/alloc"},
    admit.Patch {Path: "/spec/Options/allocation_pool"},
  }
  onlyNeType = []admit.Patch {
    admit.Patch {Path: "/spec/NetworkType"},
  }
)

func TestValidateNetwork(t *testing.T) {
  validator := admit.Validator{}
  for _, tc := range validateNetworkTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      writerStub := httpstub.NewWriterStub()
      oldNet, _, shouldOldMalform := getNetForValidate(tc.oldNetName, valNets, tc.neType)
      newNet, _, shouldNewMalform := getNetForValidate(tc.newNetName, valNets, tc.neType)
      request,err := utils.CreateHttpRequest(oldNet, newNet, shouldOldMalform, shouldNewMalform, tc.opType)
      if err != nil {
        t.Errorf("Could not create test HTTP Request object, because:%v", err)
        return
      }
      testArtifacts := utils.TestArtifacts{TestNets: valNets}
      if tc.tconf != nil {
        testArtifacts.TestTconfs = tc.tconf
      }
      testClient := stubs.NewClientSetStub(testArtifacts)
      validator.Client = testClient
      validator.ValidateNetwork(writerStub, request)
      err = utils.ValidateHttpResponse(writerStub, tc.isErrorExpected, tc.expectedPatches)
      if err != nil {
        t.Errorf("Received HTTP Response did not match expectation, because:%v", err)
        return
      }
      var timesUpdateWasCalled int
      if testClient.DanmClient.TconfClient != nil {
        timesUpdateWasCalled = testClient.DanmClient.TconfClient.TimesUpdateWasCalled
      }
      if tc.timesUpdateShouldBeCalled != timesUpdateWasCalled {
        t.Errorf("TenantConfig should have been updated:" + strconv.Itoa(tc.timesUpdateShouldBeCalled) + " times, but it happened:" + strconv.Itoa(timesUpdateWasCalled) + " times instead")
      }
    })
  }
}

func getNetForValidate(name string, nets []danmtypes.DanmNet, neType string) ([]byte, *danmtypes.DanmNet, bool) {
  dnet := utils.GetTestNet(name, nets)
  if dnet == nil {
    return nil, nil, false
  }
  var shouldItMalform bool
  if strings.HasPrefix(dnet.ObjectMeta.Name, "malform") {
    shouldItMalform = true
  }
  dnet.TypeMeta.Kind = neType
  dnetBinary,_ := json.Marshal(dnet)
  return dnetBinary, dnet, shouldItMalform
}