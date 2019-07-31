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
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
  delNets = []danmtypes.DanmNet {
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "malformed"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-type"},
      TypeMeta: meta_v1.TypeMeta {Kind: "DanmNet"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "flannel"},
      TypeMeta: meta_v1.TypeMeta {Kind: "TenantNetwork"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "flannel", NetworkID: "flannel"},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "ipvlan"},
      TypeMeta: meta_v1.TypeMeta {Kind: "TenantNetwork"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "ipvlan", NetworkID: "nanomsg", Options: danmtypes.DanmNetOption{Device: "ens4", Vlan: 500}},
    },
    danmtypes.DanmNet {
      ObjectMeta: meta_v1.ObjectMeta {Name: "sriov"},
      TypeMeta: meta_v1.TypeMeta {Kind: "TenantNetwork"},
      Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "e2", Options: danmtypes.DanmNetOption{DevicePool: "nokia.k8s.io/sriov_ens1f1", Vlan: 500}},
    },
  }
  delConf = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{
      ObjectMeta: meta_v1.ObjectMeta {Name: "tconf"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens6", VniType: "vxlan", VniRange: "1200-1300", Alloc: utils.ExhaustedAllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vlan", VniRange: "1500-1550", Alloc: utils.ExhaustedAllocFor5k},},
    },
  }
  errConf = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{
      ObjectMeta: meta_v1.ObjectMeta {Name: "errorupdate"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vlan", VniRange: "1200-1300", Alloc: utils.ExhaustedAllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vlan", VniRange: "1500-1550", Alloc: utils.ExhaustedAllocFor5k},},
    },
  }
  validConf = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{
      ObjectMeta: meta_v1.ObjectMeta {Name: "tconf"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "400-500", Alloc: utils.ExhaustedAllocFor5k},
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vlan", VniRange: "400-500", Alloc: utils.ExhaustedAllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vlan", VniRange: "500-600", Alloc: utils.ExhaustedAllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f1", VniType: "vlan", VniRange: "500-600", Alloc: utils.ExhaustedAllocFor5k},},
    },
  }
)

var deleteNetworkTcs = []struct {
  tcName string
  oldNetName string
  tconf []danmtypes.TenantConfig
  isErrorExpected bool
  shouldVniBeFreed bool
  expectedPatches []admit.Patch
  timesUpdateShouldBeCalled int
}{
  {"emptyRequest", "", nil, true, false, nil, 0},
  {"malformedOldObject", "malformed", nil, true, false, nil, 0},
  {"objectWithInvalidType", "invalid-type", nil, false, false, nil, 0},
  {"staticNetwork", "flannel", nil, false, false, nil, 0},
  {"noTenantConfig", "ipvlan", nil, true, false, nil, 0},
  {"missingInterFaceProfile", "ipvlan", delConf, false, false, nil, 0},
  {"missingDeviceProfile", "sriov", delConf, false, false, nil, 0},
  {"errorUpdating", "ipvlan", errConf, true, true, nil, 1},
  {"freeDevice", "ipvlan", validConf, false, true, nil, 1},
  {"freeDevicePool", "sriov", validConf, false, true, nil, 1},
}

func TestDeleteNetwork(t *testing.T) {
  validator := admit.Validator{}
  for _, tc := range deleteNetworkTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      defer resetTconf(tc.tconf)
      writerStub := httpstub.NewWriterStub()
      oldNet, dnet, shouldOldMalform := getTestNet(tc.oldNetName, delNets)
      request,err := utils.CreateHttpRequest(oldNet, nil, shouldOldMalform, false, "")
      if err != nil {
        t.Errorf("Could not create test HTTP Request object, because:%v", err)
        return
      }
      testArtifacts := utils.TestArtifacts{TestNets: delNets}
      if tc.shouldVniBeFreed {
        testArtifacts.ReservedVnis = createVniReservation(dnet, false)
      } else {
        testArtifacts.ReservedVnis = createVniReservation(dnet, true)
      }
      if tc.tconf != nil {
        testArtifacts.TestTconfs = tc.tconf
      }
      testClient := stubs.NewClientSetStub(testArtifacts)
      validator.Client = testClient
      validator.DeleteNetwork(writerStub, request)
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

func getTestNet(name string, nets []danmtypes.DanmNet) ([]byte, *danmtypes.DanmNet, bool) {
  dnet := utils.GetTestNet(name, nets)
  if dnet == nil {
    return nil, nil, false
  }
  var shouldItMalform bool
  if strings.HasPrefix(dnet.ObjectMeta.Name, "malform") {
    shouldItMalform = true
  }
  dnetBinary,_ := json.Marshal(dnet)
  return dnetBinary, dnet, shouldItMalform
}

func createVniReservation(dnet *danmtypes.DanmNet, shouldBeReserved bool) []utils.ReservedVnisList {
  if dnet == nil {
    return nil
  }
  vni := dnet.Spec.Options.Vlan
  vniType := "vlan"
  if vni == 0 {
    vni = dnet.Spec.Options.Vxlan
    vniType = "vxlan"
  }
  ifaceName := dnet.Spec.Options.Device
  if ifaceName == "" {
    ifaceName = dnet.Spec.Options.DevicePool
  }
  return utils.CreateExpectedVniAllocationsList(vni, vniType, ifaceName, shouldBeReserved)
}

func resetTconf(tconf []danmtypes.TenantConfig) {
  if tconf == nil {
    return
  }
  for index,_ := range tconf[0].HostDevices {
    tconf[0].HostDevices[index].Alloc = utils.ExhaustedAllocFor5k
  }
}