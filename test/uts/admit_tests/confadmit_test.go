package admit_tests

import (
  "bytes"
  "errors"
  "strings"
  "testing"
  "encoding/json"
  "io/ioutil"
  "net/http"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/admit"
  httpstub "github.com/nokia/danm/test/stubs/http"
  "github.com/nokia/danm/test/utils"
  "k8s.io/api/admission/v1beta1"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MalformedTconf struct {
  HostDevices []danmtypes.IfaceProfile    `json:"hostDevices,omitempty"`
  NetworkIds  map[string]string `json:"networkIds,omitempty"`
  ExtraField string             `json:"extraField,omitempty"`
}

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
      oldTconf := utils.GetTconf(tc.oldTconfName, validateConfs)
      newTconf := utils.GetTconf(tc.newTconfName, validateConfs)
      request,err := createHttpRequest(oldTconf, newTconf, tc.opType)
      if err != nil {
        t.Errorf("Could not create test HTTP Request object, because:%v", err)
        return
      }
      validator.ValidateTenantConfig(writerStub, request)
      err = validateHttpResponse(writerStub, tc.isErrorExpected, tc.expectedPatches)
      if err != nil {
        t.Errorf("Received HTTP Response did not match expectation, because:%v", err)
        return
      }
    })
  }
}

func createHttpRequest(oldConf, newConf *danmtypes.TenantConfig, opType v1beta1.Operation) (*http.Request, error) {
  request := v1beta1.AdmissionRequest{}
  review := v1beta1.AdmissionReview{Request: &request}
  if opType != "" {
    review.Request.Operation = opType
  }
  var err error
  if oldConf != nil  {
    review.Request.OldObject.Raw = canItMalform(oldConf)
  }
  if newConf != nil {
    review.Request.Object.Raw = canItMalform(newConf)
  }
  httpRequest := http.Request{}
  if oldConf != nil || newConf != nil {
    rawReview, err := json.Marshal(review)
    if err != nil {
      errors.New("AdmissionReview couldn't be Marshalled because:" + err.Error())
    }
    reader := bytes.NewReader(rawReview)
    httpRequest.Body = ioutil.NopCloser(reader)
  }
  return &httpRequest, err
}

func canItMalform(config *danmtypes.TenantConfig) []byte {
  var bytes []byte
  if strings.HasPrefix(config.ObjectMeta.Name, "malformed") {
    malformedConf := MalformedTconf{HostDevices: config.HostDevices, NetworkIds: config.NetworkIds, ExtraField: "blupp"}
    bytes, _ = json.Marshal(malformedConf)
  } else {
    bytes, _ = json.Marshal(config)
  }
  return bytes
}

func validateHttpResponse(writer *httpstub.ResponseWriterStub, isErrorExpected bool, expectedPatches string) error {
  if writer.RespHeader.Get("Content-Type") != "application/json" {
    return errors.New("Content-Type is not set to application/json in the HTTP Header")
  }
  response, err := writer.GetAdmissionResponse()
  if err != nil {
    return err
  }
  if isErrorExpected {
    if response.Allowed {
      return errors.New("request would have been admitted but we expected an error")
    }
    if response.Result.Message == "" {
      return errors.New("a faulty response was sent without explanation")
    }
  } else {
    if !response.Allowed {
      return errors.New("request would have been denied but we expected it to pass through validation")
    }
    if response.Result != nil {
      return errors.New("an unnecessary Result message is put into a successful response")
    }
  }
  if expectedPatches != "" {
     return validatePatches(response, expectedPatches)
  }
  return nil
}

func validatePatches(response *v1beta1.AdmissionResponse, expectedPatches string) error {
  if expectedPatches == "empty" {
    if response.Patch != nil {
      return errors.New("did not expect any patches but some were included in the admission response")
    }
    return nil
  }
  var patches []admit.Patch
  err := json.Unmarshal(response.Patch, &patches)
  if err != nil {
    return err
  }
  if len(patches) != 1 {
    return errors.New("received number of patches was not the expected 1")
  }
  return nil
}
