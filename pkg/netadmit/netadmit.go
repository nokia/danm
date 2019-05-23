package netadmit

import (
  "bytes"
  "errors"
  "log"
  "net"
  "reflect"
  "encoding/json"
  "io/ioutil"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/apimachinery/pkg/runtime"
  "k8s.io/apimachinery/pkg/runtime/serializer"
  "k8s.io/apimachinery/pkg/types"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/ipam"
)

var (
  NetworkPatchPaths = map[string]string {
    "NetworkType": "/spec/NetworkType",
    "Alloc": "/spec/Options/alloc",
    "Pool": "/spec/Options/allocation_pool",
  }
)

type Patch struct {
  Op    string          `json:"op"`
  Path  string          `json:"path"`
  Value json.RawMessage `json:"value"`
}

func ValidateNetwork(responseWriter http.ResponseWriter, request *http.Request) {
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
  newManifest, err := getNetworkManifest(admissionReview.Request.Object.Raw)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  origNewManifest := *newManifest
  isManifestValid, err := validateNetworkByType(oldManifest, newManifest, admissionReview.Request.Operation)
  if !isManifestValid {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  err = mutateManifest(newManifest)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  responseAdmissionReview := v1beta1.AdmissionReview {
    Response: CreateReviewResponseFromPatches(createPatchListFromChanges(origNewManifest,newManifest)),
  }
  responseAdmissionReview.Response.UID = admissionReview.Request.UID
  SendAdmissionResponse(responseWriter, responseAdmissionReview)
}

func DecodeAdmissionReview(httpRequest *http.Request) (*v1beta1.AdmissionReview,error) {
  var payload []byte
  if httpRequest.Body == nil {
    return nil, errors.New("Received review request is empty!")
  }
  payload, err := ioutil.ReadAll(httpRequest.Body);
  if err != nil {
    return nil, err
  }
  codecs := serializer.NewCodecFactory(runtime.NewScheme())
  deserializer := codecs.UniversalDeserializer()
  reviewRequest := v1beta1.AdmissionReview{}
  _, _, err = deserializer.Decode(payload, nil, &reviewRequest)
  return &reviewRequest, err
}

func SendErroneousAdmissionResponse(responseWriter http.ResponseWriter, uid types.UID, err error) {
  log.Println("ERROR: Admitting resource failed with error:" + err.Error())
  failedResponse := &v1beta1.AdmissionResponse {
    Result: &metav1.Status{
      Message: err.Error(),
    },
    Allowed: false,
  }
  failedResponse.UID = uid
  responseAdmissionReview := v1beta1.AdmissionReview {
    Response: failedResponse,
  }
  SendAdmissionResponse(responseWriter, responseAdmissionReview)
}

func SendAdmissionResponse(responseWriter http.ResponseWriter, reviewResponse v1beta1.AdmissionReview) {
  respBytes, err := json.Marshal(reviewResponse)
  if err != nil {
    log.Println("ERROR: Failed to send AdmissionResponse for request:" + string(reviewResponse.Response.UID) + " because JSON marshalling failed with error:" + err.Error())
  }
  responseWriter.Header().Set("Content-Type", "application/json")
  _, err = responseWriter.Write(respBytes)
  if err != nil {
    log.Println("ERROR: Failed to send AdmissionRespons for request:" + string(reviewResponse.Response.UID) + " because putting the HTTP response on the wire failed with error:" + err.Error())
  }
}

func getNetworkManifest(objectToReview []byte) (*danmtypes.DanmNet,error) {
  networkManifest := danmtypes.DanmNet{}
  if objectToReview == nil {
    return &networkManifest, nil
  }
  decoder := json.NewDecoder(bytes.NewReader(objectToReview))
  //We are using Decoder interface, because it can notify us if any unknown fields were put into the object
  decoder.DisallowUnknownFields()
  err := decoder.Decode(&networkManifest)
  if err != nil {
    return nil, errors.New("ERROR: unknown fields are not allowed:" + err.Error())
  }
  return &networkManifest, nil
}

func validateNetworkByType(oldManifest, newManifest *danmtypes.DanmNet, opType v1beta1.Operation) (bool,error) {
  validatorMapping, isTypeHandled := danmValidationConfig[newManifest.TypeMeta.Kind]
  if !isTypeHandled {
    return false, errors.New("K8s API type:" + newManifest.TypeMeta.Kind + " is not handled by DANM webhook")
  }
  for _, validator := range validatorMapping {
    err := validator(oldManifest,newManifest,opType)
    if err != nil {
      return false, err
    }
  }
  return true, nil
}

func mutateManifest(dnet *danmtypes.DanmNet) error {
  if dnet.Spec.NetworkType == "" {
    dnet.Spec.NetworkType = "ipvlan"
  }
  var err error
  //L3, freshly added network
  if dnet.Spec.Options.Cidr != "" && dnet.Spec.Options.Alloc == "" {
    err = CreateAllocationArray(dnet)
  }
  return err
}

func CreateAllocationArray(dnet *danmtypes.DanmNet) error {
  _,ipnet,_ := net.ParseCIDR(dnet.Spec.Options.Cidr)
  bitArray, err := bitarray.CreateBitArrayFromIpnet(ipnet)
  if err != nil {
    return err
  }
  reserveGatewayIps(dnet.Spec.Options.Routes, bitArray, ipnet)
  dnet.Spec.Options.Alloc = bitArray.Encode()
  return nil
}

func reserveGatewayIps(routes map[string]string, bitArray *bitarray.BitArray, ipnet *net.IPNet) {
  for _, gw := range routes {
    gatewayPosition := ipam.Ip2int(net.ParseIP(gw)) - ipam.Ip2int(ipnet.IP)
    bitArray.Set(gatewayPosition)
  }
}

func CreateReviewResponseFromPatches(patchList []Patch) *v1beta1.AdmissionResponse {
  reviewResponse := v1beta1.AdmissionResponse{Allowed: true}
  var patches []byte
  var err error
  if len(patchList) > 0 {
    patches, err = json.Marshal(patchList)
    if err != nil {
      reviewResponse.Allowed = false
      reviewResponse.Result  = &metav1.Status{ Message: "List of patches could not be encoded, because:" + err.Error(),}
      return &reviewResponse
    }
  }
  if len(patches) > 0 {
    reviewResponse.Patch = []byte(patches)
    pt := v1beta1.PatchTypeJSONPatch
    reviewResponse.PatchType = &pt
  }
  return &reviewResponse
}

func createPatchListFromChanges(origNetwork danmtypes.DanmNet, changedNetwork *danmtypes.DanmNet) []Patch {
  patchList := make([]Patch, 0)
  if origNetwork.Spec.Options.Alloc != changedNetwork.Spec.Options.Alloc {
    //TODO: Use some reflecting here to determine name of the struct field
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths, "Alloc",
                json.RawMessage(`"` + changedNetwork.Spec.Options.Alloc + `"`)))
  }
  if origNetwork.Spec.NetworkType != changedNetwork.Spec.NetworkType {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths, "NetworkType",
                json.RawMessage(`"` +  changedNetwork.Spec.NetworkType + `"`)))
  }
  if !reflect.DeepEqual(origNetwork.Spec.Options.Pool, changedNetwork.Spec.Options.Pool) {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths, "Pool",
                json.RawMessage(`{"Start":"` + changedNetwork.Spec.Options.Pool.Start + `","End":"` + changedNetwork.Spec.Options.Pool.End + `"}`)))
  }
  return patchList
}

func CreateGenericPatchFromChange(attributePaths map[string]string, attribute string, value []byte ) Patch {
  patch := Patch {
    Op:    "replace",
    Path:  attributePaths[attribute],
    Value: value,
  }
  return patch
}
