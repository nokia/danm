package utils

import (
  "bytes"
  "errors"
  "net"
  "strconv"
  "strings"
  "encoding/json"
  "io/ioutil"
  "net/http"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/admit"
  httpstub "github.com/nokia/danm/test/stubs/http"
  "k8s.io/api/admission/v1beta1"
)

var (
  AllocFor5k = createAlloc(5000)
  ExhaustedAllocFor5k = exhaustAlloc(AllocFor5k)
)

type TestArtifacts struct {
  TestNets []danmtypes.DanmNet
  TestEps []danmtypes.DanmEp
  ReservedIps []ReservedIpsList
  TestTconfs []danmtypes.TenantConfig
  ReservedVnis []ReservedVnisList
  ExhaustAllocs []int
}

type ReservedIpsList struct {
  NetworkId string
  Reservations []Reservation
}

type Reservation struct {
  Ip string
  Set bool
}

type ReservedVnisList struct {
  ProfileName string
  VniType string
  Reservations []VniReservation
}

type VniReservation struct {
  Vni int
  Set bool
}

type MalformedObject struct {
  ExtraField string `json:"extraField,omitempty"`
}

func SetupAllocationPools(nets []danmtypes.DanmNet) error {
  for index, dnet := range nets {
    if dnet.Spec.Options.Cidr != "" {
      nets[index].Spec = InitAllocPool(&dnet).Spec
    }
  }
  return nil
}

func InitAllocPool(dnet *danmtypes.DanmNet) *danmtypes.DanmNet {
  if dnet.Spec.Options.Cidr == "" {
    return dnet
  }
  admit.CreateAllocationArray(dnet)
  _, ipnet, _ := net.ParseCIDR(dnet.Spec.Options.Cidr)
  if dnet.Spec.Options.Pool.Start == "" {
    dnet.Spec.Options.Pool.Start = (ipam.Int2ip(ipam.Ip2int(ipnet.IP) + 1)).String()
  }
  if dnet.Spec.Options.Pool.End == "" {
    dnet.Spec.Options.Pool.End = (ipam.Int2ip(ipam.Ip2int(admit.GetBroadcastAddress(ipnet)) - 1)).String()
  }
  if strings.HasPrefix(dnet.ObjectMeta.Name, "full") {
    exhaustNetwork(dnet)
  }
  return dnet
}

func GetTestNet(netId string, testNets []danmtypes.DanmNet) *danmtypes.DanmNet {
  for _, net := range testNets {
    if net.ObjectMeta.Name == netId {
      return &net
    }
  }
  return nil
}

func CreateExpectedAllocationsList(ip string, isExpectedToBeSet bool, networkId string) []ReservedIpsList {
  var ips []ReservedIpsList
  if ip != "" {
    reservation := Reservation {Ip: ip, Set: isExpectedToBeSet,}
    expectedAllocation := ReservedIpsList{NetworkId: networkId, Reservations: []Reservation {reservation,},}
    ips = append(ips, expectedAllocation)
  }
  return ips
}

func CreateExpectedVniAllocationsList(vni int, vniType, ifaceName string, isExpectedToBeSet bool) []ReservedVnisList {
  var vnis []ReservedVnisList
  if vni != 0 {
    reservation := VniReservation {Vni: vni, Set: isExpectedToBeSet,}
    expectedAllocation := ReservedVnisList{ProfileName: ifaceName, VniType: vniType, Reservations: []VniReservation {reservation,},}
    vnis = append(vnis, expectedAllocation)
  }
  return vnis
}

func exhaustNetwork(netInfo *danmtypes.DanmNet) {
    ba := bitarray.NewBitArrayFromBase64(netInfo.Spec.Options.Alloc)
    _, ipnet, _ := net.ParseCIDR(netInfo.Spec.Options.Cidr)
    ipnetNum := ipam.Ip2int(ipnet.IP)
    begin := ipam.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.Start)) - ipnetNum
    end := ipam.Ip2int(net.ParseIP(netInfo.Spec.Options.Pool.End)) - ipnetNum
    for i:=begin;i<=end;i++ {
        ba.Set(uint32(i))
    }
    netInfo.Spec.Options.Alloc = ba.Encode()
}

func GetTconf(tconfName string, tconfSet []danmtypes.TenantConfig) *danmtypes.TenantConfig {
  for _, tconf := range tconfSet {
    if tconf.ObjectMeta.Name == tconfName {
      return &tconf
    }
  }
  return nil
}

func CreateHttpRequest(oldObj, newObj []byte, isOldMalformed, isNewMalformed bool, opType v1beta1.Operation) (*http.Request, error) {
  request := v1beta1.AdmissionRequest{}
  review := v1beta1.AdmissionReview{Request: &request}
  if opType != "" {
    review.Request.Operation = opType
  }
  var err error
  if oldObj != nil  {
    review.Request.OldObject.Raw = canItMalform(oldObj, isOldMalformed)
  }
  if newObj != nil {
    review.Request.Object.Raw = canItMalform(newObj, isNewMalformed)
  }
  httpRequest := http.Request{}
  if oldObj != nil || newObj != nil {
    rawReview, err := json.Marshal(review)
    if err != nil {
      errors.New("AdmissionReview couldn't be marshalled because:" + err.Error())
    }
    reader := bytes.NewReader(rawReview)
    httpRequest.Body = ioutil.NopCloser(reader)
  }
  return &httpRequest, err
}

func canItMalform(obj []byte, shouldBeMalformed bool) []byte {
  if shouldBeMalformed {
    malformedObj := MalformedObject{ExtraField: "blupp"}
    obj, _ = json.Marshal(malformedObj)
  }
  return obj
}

func ValidateHttpResponse(writer *httpstub.ResponseWriterStub, isErrorExpected bool, expectedPatches []admit.Patch) error {
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
  return validatePatches(response, expectedPatches)
}

func validatePatches(response *v1beta1.AdmissionResponse, expectedPatches []admit.Patch) error {
  if len(expectedPatches) == 0 {
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
  if len(patches) != len(expectedPatches) {
    return errors.New("received number of patches:" + strconv.Itoa(len(patches)) + " was not what we expected:" + strconv.Itoa(len(expectedPatches)))
  }
  for _, expPatch := range expectedPatches {
    var foundMatchingPatch bool
    for _, recPatch := range patches {
      if expPatch.Path == recPatch.Path {
        foundMatchingPatch = true
      }
    }
    if !foundMatchingPatch {
      return errors.New("Patch expected to modify path:" + expPatch.Path + " was not included in the response")
    }
  }
  return nil
}

func createAlloc(len int) string {
  ba,_ := bitarray.NewBitArray(len+1)
  return ba.Encode()
}

func exhaustAlloc(alloc string) string {
  ba := bitarray.NewBitArrayFromBase64(alloc)
  for i:=0; i<ba.Len(); i++ {
        ba.Set(uint32(i))
  }
  return ba.Encode()
}

func ReserveVnis(iface *danmtypes.IfaceProfile, vniRange []int) {
  allocs := bitarray.NewBitArrayFromBase64(iface.Alloc)
  for i := vniRange[0]; i <= vniRange[1]; i++ {
    allocs.Set(uint32(i))
  }
  iface.Alloc = allocs.Encode()
}