package admit

import (
  "bytes"
  "errors"
  "net"
  "reflect"
  "strconv"
  "strings"
  "time"
  "encoding/json"
  "math/rand"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/confman"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/pkg/metacni"
)

var (
  NetworkPatchPaths = map[string]string {
    "NetworkType": "/spec/NetworkType",
    "Alloc": "/spec/Options/alloc",
    "Pool": "/spec/Options/allocation_pool",
    "Device": "/spec/Options/host_device",
    "Vlan": "/spec/Options/vlan",
    "Vxlan": "/spec/Options/vxlan",
  }
)

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
  err = mutateNetManifest(newManifest)
  if err != nil {
    SendErroneousAdmissionResponse(responseWriter, admissionReview.Request.UID, err)
    return
  }
  responseAdmissionReview := v1beta1.AdmissionReview {
    Response: CreateReviewResponseFromPatches(createPatchListFromNetChanges(origNewManifest,newManifest)),
  }
  responseAdmissionReview.Response.UID = admissionReview.Request.UID
  SendAdmissionResponse(responseWriter, responseAdmissionReview)
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

func mutateNetManifest(dnet *danmtypes.DanmNet) error {
  if dnet.Spec.NetworkType == "" {
    dnet.Spec.NetworkType = "ipvlan"
  }
  var err error
  //L3, freshly added network
  if dnet.Spec.Options.Cidr != "" && dnet.Spec.Options.Alloc == "" {
    err = CreateAllocationArray(dnet)
    if err != nil {
      return err
    }
  }
  if dnet.TypeMeta.Kind == "TenantNetwork" {
    err = addTenantSpecificDetails(dnet)
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

//TODO: we could easily add CIDR + allocation pool overwrites as well for TenantNetworks, if needed
//Open an issue with your use-case if you see the need!
func addTenantSpecificDetails(tnet *danmtypes.DanmNet) error {
  tconf, err := getTenantConfig()
  if err != nil {
    return err
  }
  if IsTypeDynamic(tnet.Spec.NetworkType) {
    err = allocateDetailsForDynamicBackends(tnet,tconf)
  }
  nId, isPresent := tconf.NetworkIds[tnet.Spec.NetworkType]
  if isPresent {
    tnet.Spec.NetworkID = nId
  }
  return nil
}

func getTenantConfig() (*danmtypes.TenantConfig, error) {
  danmClient, err := metacni.CreateDanmClient()
  if err != nil {
    return nil, err
  }
  reply, err := danmClient.DanmV1().TenantConfigs().List(metav1.ListOptions{})
  if err != nil {
    return nil, err
  }
  configs := reply.Items
  if len(configs) == 0 {
    return nil, errors.New("TenantNetworks cannot be created without provisioning a TenantConfig first!")
  }
  //TODO: do a namespace based selection later if one generic config does not suffice
  return &configs[0], nil
}

func allocateDetailsForDynamicBackends(tnet *danmtypes.DanmNet,tconf *danmtypes.TenantConfig) error {
  for _, iface := range tconf.HostDevices {
    if tnet.Spec.Options.DevicePool != "" && strings.Contains(tnet.Spec.Options.DevicePool, iface.Name) {
      //This is the interface profile belonging to the network's DevicePool
      return attachNetworkToIfaceProfile(tnet,tconf,iface)
    } else if tnet.Spec.Options.Device == iface.Name {
      //This is the interface profile matching the requested host_device
      return attachNetworkToIfaceProfile(tnet,tconf,iface)
    }
  }
  //Device based networks need to have their related physical interfaces explicitly allowed configured by the administrator
  if tnet.Spec.Options.DevicePool != "" {
    return errors.New("The physical interface used by device_pool:" + tnet.Spec.Options.DevicePool + " is not forbidden for tenants!")
  }
  rand.Seed(time.Now().UnixNano())
  chosenProfile := tconf.HostDevices[rand.Intn(len(tconf.HostDevices))]
  //Otherwise we randomly choose an interface profile and attach the TenantNetwork to it
  return attachNetworkToIfaceProfile(tnet,tconf,chosenProfile)
}

func attachNetworkToIfaceProfile(tnet *danmtypes.DanmNet, tconf *danmtypes.TenantConfig, iface danmtypes.IfaceProfile) error {
  if tnet.Spec.Options.Device != "" && tnet.Spec.Options.DevicePool == "" {
    tnet.Spec.Options.Device = iface.Name
  }
  if (iface.VniType == "vlan" && tnet.Spec.Options.Vlan == 0) ||
     (iface.VniType == "vxlan" && tnet.Spec.Options.Vxlan == 0) {
    vni,err := confman.Reserve(tconf, iface)
    if err != nil {
      return errors.New("cannot reserve VNI for interface:" + iface.Name + " , because:" + err.Error())
    }
    if iface.VniType != "vlan" {
      tnet.Spec.Options.Vlan = vni
    } else {
      tnet.Spec.Options.Vxlan = vni
    }
  }
  return nil
}

func createPatchListFromNetChanges(origNetwork danmtypes.DanmNet, changedNetwork *danmtypes.DanmNet) []Patch {
  patchList := make([]Patch, 0)
  if origNetwork.Spec.Options.Alloc != changedNetwork.Spec.Options.Alloc {
    //TODO: Could (?) use some reflecting here to determine name of the struct field
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["Alloc"],
                json.RawMessage(`"` + changedNetwork.Spec.Options.Alloc + `"`)))
  }
  if origNetwork.Spec.NetworkType != changedNetwork.Spec.NetworkType {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["NetworkType"],
                json.RawMessage(`"` +  changedNetwork.Spec.NetworkType + `"`)))
  }
  if !reflect.DeepEqual(origNetwork.Spec.Options.Pool, changedNetwork.Spec.Options.Pool) {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["Pool"],
                json.RawMessage(`{"Start":"` + changedNetwork.Spec.Options.Pool.Start +
                                `","End":"` + changedNetwork.Spec.Options.Pool.End + `"}`)))
  }
  if origNetwork.Spec.Options.Device != changedNetwork.Spec.Options.Device {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["Device"],
                json.RawMessage(`"` + changedNetwork.Spec.Options.Device + `"`)))
  }
  if origNetwork.Spec.Options.Vlan != changedNetwork.Spec.Options.Vlan {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["Vlan"],
                json.RawMessage(`"` + strconv.Itoa(changedNetwork.Spec.Options.Vlan) + `"`)))
  }
  if origNetwork.Spec.Options.Vxlan != changedNetwork.Spec.Options.Vxlan {
    patchList = append(patchList, CreateGenericPatchFromChange(NetworkPatchPaths["Vxlan"],
                json.RawMessage(`"` + strconv.Itoa(changedNetwork.Spec.Options.Vxlan) + `"`)))
  }
  return patchList
}
