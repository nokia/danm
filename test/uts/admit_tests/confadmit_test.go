package admit_tests

import (
  "strings"
  "testing"
  "encoding/json"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/admit"
  httpstub "github.com/nokia/danm/test/stubs/http"
  "github.com/nokia/danm/test/utils"
  "k8s.io/api/admission/v1beta1"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
  validateConfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "malformed"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc: utils.AllocFor5k},
      },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-type"},
      TypeMeta: meta_v1.TypeMeta {Kind: "invalid"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc: utils.AllocFor5k},
      },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "empty-config"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "noname"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710"},
        danmtypes.IfaceProfile{VniType: "vlan", VniRange: "200,500-510"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "norange"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan"},
        danmtypes.IfaceProfile{Name: "ens5", VniType: "vlan", VniRange: "700-710"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "notype"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710"},
        danmtypes.IfaceProfile{Name: "ens5", VniRange: "700-710"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-vni-type"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan2", VniRange: "700-710"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-vni-value"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-71a0"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "invalid-vni-range"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "900-4999,5001"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "valid-vni-range"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "900-4999,5000"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "manual-alloc-old"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc: utils.AllocFor5k},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "manual-alloc"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc: utils.AllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vlan", VniRange: "700-710"},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "nonetype"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      NetworkIds: map[string]string {
        "": "asd",
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "nonid"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      NetworkIds: map[string]string {
        "flannel": "",
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "longnid"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      NetworkIds: map[string]string {
        "flannel": "abcdefghijkl",
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "longnid-sriov"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      NetworkIds: map[string]string {
        "flannel": "abcdefghijkl",
        "sriov": "abcdefghijkl",
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "shortnid"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      NetworkIds: map[string]string {
        "flannel": "abcdefghijkl",
        "sriov": "abcdefghijk",
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "old-iface"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "900-4999,5000", Alloc: utils.AllocFor5k},
       },
    },
    danmtypes.TenantConfig {
      ObjectMeta: meta_v1.ObjectMeta {Name: "new-iface"},TypeMeta: meta_v1.TypeMeta {Kind: "TenantConfig"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "900-4999,5000", Alloc: utils.AllocFor5k},
       },
      NetworkIds: map[string]string {
        "flannel": "flannel",
       },
    },
  }
)

var validateTconfTcs = []struct {
  tcName string
  oldTconfName string
  newTconfName string
  opType v1beta1.Operation
  isErrorExpected bool
  expectedPatches string
}{
  {"emptyRequest", "", "", "", true, "empty"},
  {"malformedOldObject", "malformed", "", "", true, "empty"},
  {"malformedNewObject", "", "malformed", "", true, "empty"},
  {"objectWithInvalidType", "", "invalid-type", "", true, "empty"},
  {"emptyCofig", "", "empty-config", "", true, "empty"},
  {"interfaceProfileWithoutName", "", "noname", "", true, "empty"},
  {"interfaceProfileWithoutVniRange", "", "norange", "", true, "empty"},
  {"interfaceProfileWithoutVniType", "", "notype", "", true, "empty"},
  {"interfaceProfileWithInvalidVniType", "", "invalid-vni-type", "", true, "empty"},
  {"interfaceProfileWithInvalidVniValue", "", "invalid-vni-value", "", true, "empty"},
  {"interfaceProfileWithInvalidVniRange", "", "invalid-vni-range", "", true, "empty"},
  {"interfaceProfileWithValidVniRange", "", "valid-vni-range", "", false, "1"},
  {"interfaceProfileWithSetAlloc", "", "manual-alloc", v1beta1.Create, true, "empty"},
  {"interfaceProfileChangeWithAlloc", "manual-alloc-old", "manual-alloc", v1beta1.Update, false, "1"},
  {"networkIdWithoutKey", "", "nonid", "", true, "empty"},
  {"networkIdWithoutValue", "", "nonetype", "", true, "empty"},
  {"longNidWithStaticNeType", "", "longnid", "", false, "empty"},
  {"longNidWithDynamicNeType", "", "longnid-sriov", "", true, "empty"},
  {"okayNids", "", "shortnid", "", false, "empty"},
  {"noChangeInIfaces", "old-iface", "new-iface", v1beta1.Update, false, "empty"},
}

func TestValidateTenantConfig(t *testing.T) {
  validator := admit.Validator{}
  for _, tc := range validateTconfTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      writerStub := httpstub.NewWriterStub()
      oldTconf, shouldOldMalform := getTestConf(tc.oldTconfName, validateConfs)
      newTconf, shouldNewMalform := getTestConf(tc.newTconfName, validateConfs)
      request,err := utils.CreateHttpRequest(oldTconf, newTconf, shouldOldMalform, shouldNewMalform, tc.opType)
      if err != nil {
        t.Errorf("Could not create test HTTP Request object, because:%v", err)
        return
      }
      validator.ValidateTenantConfig(writerStub, request)
      err = utils.ValidateHttpResponse(writerStub, tc.isErrorExpected, tc.expectedPatches)
      if err != nil {
        t.Errorf("Received HTTP Response did not match expectation, because:%v", err)
        return
      }
    })
  }
}

func getTestConf(name string, confs []danmtypes.TenantConfig) ([]byte, bool) {
  tconf := utils.GetTconf(name, confs)
  if tconf == nil {
    return nil, false
  }
  var shouldItMalform bool
  if strings.HasPrefix(tconf.ObjectMeta.Name, "malform") {
    shouldItMalform = true
  }
  tconfBinary,_ := json.Marshal(tconf)
  return tconfBinary, shouldItMalform
}
