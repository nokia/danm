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
      ObjectMeta: meta_v1.ObjectMeta {Name: "tconf"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc: utils.AllocFor5k},
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vlan", VniRange: "200,500-510", Alloc: utils.AllocFor5k},
        danmtypes.IfaceProfile{Name: "ens6", VniType: "vxlan", VniRange: "1200-1300", Alloc: utils.AllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vlan", VniRange: "1500-1550", Alloc: utils.AllocFor5k},
        danmtypes.IfaceProfile{Name: "nokia.k8s.io/sriov_ens1f0", VniType: "vxlan", VniRange: "1600-1650", Alloc: utils.AllocFor5k},
      },
    },
  }
)

var validateTconfTcs = []struct {
  tcName string
  oldTconfName string
  newTconfName string
  isErrorExpected bool
}{
  {"emptyRequest", "", "", true},
  {"malformedOldObject", "malformed", "", true},
  {"malformedNewObject", "", "malformed", true},
}

func TestValidateTenantConfig(t *testing.T) {
  validator := admit.Validator{}
  for _, tc := range validateTconfTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      writerStub := httpstub.NewWriterStub()
      oldTconf := utils.GetTconf(tc.oldTconfName, validateConfs)
      newTconf := utils.GetTconf(tc.newTconfName, validateConfs)
      request,err := createHttpRequest(oldTconf, newTconf)
      if err != nil {
        t.Errorf("Could not create test HTTP Request object, because:%v", err)
        return
      }
      validator.ValidateTenantConfig(writerStub, request)
      err = validateHttpResponse(writerStub, tc.isErrorExpected)
      if err != nil {
        t.Errorf("Received HTTP Response did not match expectation, because:%v", err)
        return
      }
    })
  }
}

func createHttpRequest(oldConf, newConf *danmtypes.TenantConfig) (*http.Request, error) {
  request := v1beta1.AdmissionRequest{}
  review := v1beta1.AdmissionReview{Request: &request}
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

func validateHttpResponse(writer *httpstub.ResponseWriterStub, isErrorExpected bool) error {
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
//    fmt.Printf("lofasz: %+v\n", response)
    if response.Result.Message == "" {
      return errors.New("a faulty response was sent without explanation")
    }
  } else {
    if !response.Allowed {
      return errors.New("request would have been denied but we expected it to pass through validation")
    }
    if response.Result.Message != "" {
      return errors.New("a Message is put into a successful response")
    }
  }
  return nil
}