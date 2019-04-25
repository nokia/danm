package netadmit

import (
  "errors"
  "net"
  "encoding/json"
  "io/ioutil"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  "k8s.io/apimachinery/pkg/runtime"
  "k8s.io/apimachinery/pkg/runtime/serializer"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/ipam"
)

func ValidateNetwork(responseWriter http.ResponseWriter, request *http.Request) {
  admissionReview, err := DecodeAdmissionReview(request)
  if err != nil {
    return
  }
  manifest, err := getNetworkManifest(admissionReview.Request.Object.Raw)
  if err != nil {
    return
  }
  isManifestValid, err := validateNetworkByType(manifest)
  if !isManifestValid {
    return
  }
/*  bitArray, err := CreateAllocationArray(dnet)
  if err != nil {
    return err
  }
  dnet.Spec.Options.Alloc = bitArray.Encode()*/
  return
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

func getNetworkManifest(objectToReview []byte) (*danmtypes.DanmNet,error) {
  networkManifest := danmtypes.DanmNet{}
  err := json.Unmarshal(objectToReview, &networkManifest)
  return &networkManifest, err
}

func validateNetworkByType(manifest *danmtypes.DanmNet) (bool,error) {
  for _, validatorMapping := range danmValidationConfig.ValidatorMappings {
    if validatorMapping.ApiType == manifest.TypeMeta.Kind {
      for _, validator := range validatorMapping.Validators {
        err := validator(manifest)
        if err != nil {
          return false, err
        }
      }
    }
  }
  return true, nil
}

func CreateAllocationArray(dnet *danmtypes.DanmNet) (*bitarray.BitArray,error) {
  _,ipnet,_ := net.ParseCIDR(dnet.Spec.Options.Cidr)
  bitArray, err := bitarray.CreateBitArrayFromIpnet(ipnet)
  if err != nil {
    return nil, err
  }
  err = reserveGatewayIps(dnet.Spec.Options.Routes, bitArray, ipnet)
  if err != nil {
    return nil, err
  }
  return bitArray, nil
}

func reserveGatewayIps(routes map[string]string, bitArray *bitarray.BitArray, ipnet *net.IPNet) error {
  for _, gw := range routes {
    gatewayPosition := ipam.Ip2int(net.ParseIP(gw)) - ipam.Ip2int(ipnet.IP)
    bitArray.Set(gatewayPosition)
  }
  return nil
}