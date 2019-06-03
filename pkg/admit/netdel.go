package admit

import (
  "errors"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  "github.com/nokia/danm/pkg/confman"
)

func DeleteNetwork(responseWriter http.ResponseWriter, request *http.Request) {
  admissionReview, err := DecodeAdmissionReview(request)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  oldManifest, err := getNetworkManifest(admissionReview.Request.OldObject.Raw)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  if oldManifest.TypeMeta.Kind == "TenantNetwork" && IsTypeDynamic(oldManifest.Spec.NetworkType) {
    tconf, err := confman.GetTenantConfig()
    if err != nil {
      SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID,
      errors.New("The network's VNI could not be freed, because:" + err.Error()))
      return
    }
    err = confman.Free(tconf,oldManifest)
    if err != nil {
      SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID,
      errors.New("The network's VNI could not be freed, because:" + err.Error()))
      return
    }
  }
  responseAdmissionReview := v1beta1.AdmissionReview {
    Response: CreateReviewResponseFromPatches(nil),
  }
  responseAdmissionReview.Response.UID = admissionReview.Request.UID
  SendAdmissionResponse(responseWriter, responseAdmissionReview)
}