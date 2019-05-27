package admit

import (
  "bytes"
  "errors"
  "reflect"
  "encoding/json"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
)

const (
  MaxAllowedVni = 5000
  HostDevicePath = "/hostDevices"
)

func ValidateTenantConfig(responseWriter http.ResponseWriter, request *http.Request) {
  admissionReview, err := DecodeAdmissionReview(request)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  oldManifest, err := decodeTenantConfig(admissionReview.Request.OldObject.Raw)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  newManifest, err := decodeTenantConfig(admissionReview.Request.Object.Raw)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  origNewManifest := *newManifest
  isManifestValid, err := validateConfig(oldManifest, newManifest, admissionReview.Request.Operation)
  if !isManifestValid {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  err = mutateConfigManifest(newManifest)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  responseAdmissionReview := v1beta1.AdmissionReview {
    Response: CreateReviewResponseFromPatches(createPatchListFromConfigChanges(origNewManifest,newManifest)),
  }
  responseAdmissionReview.Response.UID = admissionReview.Request.UID
  SendAdmissionResponse(responseWriter, responseAdmissionReview)
}

//TODO: can the return type be interface{}, and somehow encoding be input based?
//Until that, this is unfortunetaly duplicated code
func decodeTenantConfig(objectToReview []byte) (*danmtypes.TenantConfig,error) {
  configManifest := danmtypes.TenantConfig{}
  if objectToReview == nil {
    return &configManifest, nil
  }
  decoder := json.NewDecoder(bytes.NewReader(objectToReview))
  //We are using Decoder interface, because it can notify us if any unknown fields were put into the object
  decoder.DisallowUnknownFields()
  err := decoder.Decode(&configManifest)
  if err != nil {
    return nil, errors.New("ERROR: unknown fields are not allowed:" + err.Error())
  }
  return &configManifest, nil
}

//TODO: as above. Until reflection is figured out, this is somewhat of a duplication
//Maybe a struct wrapping the exact object type could also work (that would push reflection responsibility on the validators though)
func validateConfig(oldManifest, newManifest *danmtypes.TenantConfig, opType v1beta1.Operation) (bool,error) {
  if newManifest.TypeMeta.Kind != "TenantConfig" {
    return false, errors.New("K8s API type:" + newManifest.TypeMeta.Kind + " is not handled by DANM webhook")
  }
  err := validateTenantconfig(oldManifest,newManifest,opType)
  if err != nil {
      return false, err
  }
  return true, nil
}

func mutateConfigManifest(tconf *danmtypes.TenantConfig) error {
  for ifaceIndex, ifaceConf := range tconf.HostDevices {
    //We don't want to either re-init existing allocations, or unnecessarily create arrays for non-virtual networks
    if ifaceConf.Alloc != "" || ifaceConf.VniType == "" {
      continue
    }
    bitArray, err := bitarray.NewBitArray(MaxAllowedVni)
    if err != nil {
      return err
    }
    tconf.HostDevices[ifaceIndex].Alloc = bitArray.Encode()
  }
  return nil
}

func createPatchListFromConfigChanges(origConfig danmtypes.TenantConfig, changedConfig *danmtypes.TenantConfig) []Patch {
  patchList := make([]Patch, 0)
  for ifaceIndex, ifaceConf := range changedConfig.HostDevices {
    if !reflect.DeepEqual(origConfig.HostDevices[ifaceIndex], ifaceConf) {
      patchList = append(patchList, CreateGenericPatchFromChange(HostDevicePath + "/" + ifaceConf.Name,
                json.RawMessage(`{"name":"` + ifaceConf.Name +
                                `","vniType":"` + ifaceConf.VniType +
                                `","vniRange":"` + ifaceConf.VniRange +
                                `","alloc":"` + ifaceConf.Alloc + `"}`)))
    }
  }
  return patchList
}