package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	danmtypes "github.com/danm-cni/danm/crd/apis/danm/v1"
	"github.com/danm-cni/danm/pkg/bitarray"
	"github.com/danm-cni/danm/pkg/ipam"
	"k8s.io/api/admission/v1beta1"
)

var (
	AllocFor5k          = createAlloc(5000)
	ExhaustedAllocFor5k = exhaustAlloc(AllocFor5k)
)

type TestArtifacts struct {
	TestNets               []danmtypes.DanmNet
	TestTnets              []danmtypes.TenantNetwork
	TestCnets              []danmtypes.ClusterNetwork
	TestEps                []danmtypes.DanmEp
	TestRips               []danmtypes.ReservedIP
	ReservedIps            []ReservedIpsList
	TestTconfs             []danmtypes.TenantConfig
	ReservedVnis           []ReservedVnisList
	ExhaustAllocs          []int
	IsRipListErrorExpected bool
}

type ReservedIpsList struct {
	NetworkId    string
	Reservations []Reservation
}

type Reservation struct {
	Ip  string
	Set bool
}

type ReservedVnisList struct {
	ProfileName  string
	VniType      string
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
		InitAllocPool(&dnet)
		nets[index].Spec = dnet.Spec
	}
	return nil
}

func InitAllocPool(dnet *danmtypes.DanmNet) {
	dnet.Spec.Options.Alloc = ""
	dnet.Spec.Options.Pool.Start, dnet.Spec.Options.Pool.End, dnet.Spec.Options.Alloc =
		ipam.InitAllocPool(dnet.Spec.Options.Cidr, dnet.Spec.Options.Pool.Start, dnet.Spec.Options.Pool.End, dnet.Spec.Options.Alloc, dnet.Spec.Options.Routes)
	ipam.InitV6AllocFields(dnet)
	if strings.HasPrefix(dnet.ObjectMeta.Name, "full") {
		exhaustNetwork(dnet)
	}
}

func GetTestNet(netId string, testNets []danmtypes.DanmNet) *danmtypes.DanmNet {
	for _, net := range testNets {
		if net.ObjectMeta.Name == netId {
			return &net
		}
	}
	return nil
}

func AppendIpToExpectedAllocsList(allocs []ReservedIpsList, ip string, isExpectedToBeSet bool, networkId string) []ReservedIpsList {
	if ip != "" {
		reservation := Reservation{Ip: ip, Set: isExpectedToBeSet}
		expectedAllocation := ReservedIpsList{NetworkId: networkId, Reservations: []Reservation{reservation}}
		allocs = append(allocs, expectedAllocation)
	}
	return allocs
}

func CreateExpectedVniAllocationsList(vni int, vniType, ifaceName string, isExpectedToBeSet bool) []ReservedVnisList {
	var vnis []ReservedVnisList
	if vni != 0 {
		reservation := VniReservation{Vni: vni, Set: isExpectedToBeSet}
		expectedAllocation := ReservedVnisList{ProfileName: ifaceName, VniType: vniType, Reservations: []VniReservation{reservation}}
		vnis = append(vnis, expectedAllocation)
	}
	return vnis
}

func exhaustNetwork(dnet *danmtypes.DanmNet) {
	var i uint32
	if dnet.Spec.Options.Alloc != "" {
		v4Ba := bitarray.NewBitArrayFromBase64(dnet.Spec.Options.Alloc)
		for i = 0; i < v4Ba.Len(); i++ {
			v4Ba.Set(i)
		}
		dnet.Spec.Options.Alloc = v4Ba.Encode()
	}
	if dnet.Spec.Options.Alloc6 != "" {
		v6Ba := bitarray.NewBitArrayFromBase64(dnet.Spec.Options.Alloc6)
		for i = 0; i < v6Ba.Len(); i++ {
			v6Ba.Set(i)
		}
		dnet.Spec.Options.Alloc6 = v6Ba.Encode()
	}
}

func GetTconf(tconfName string, tconfSet []danmtypes.TenantConfig) *danmtypes.TenantConfig {
	for _, tconf := range tconfSet {
		if tconf.ObjectMeta.Name == tconfName {
			return &tconf
		}
	}
	return nil
}

func CreateHttpRequest(oldObj, newObj []byte, isOldMalformed, isNewMalformed bool, opType v1beta1.Operation, isDry bool) (*http.Request, error) {
	request := v1beta1.AdmissionRequest{}
	review := v1beta1.AdmissionReview{Request: &request}
	if opType != "" {
		review.Request.Operation = opType
	}
	var err error
	if oldObj != nil {
		review.Request.OldObject.Raw = canItMalform(oldObj, isOldMalformed)
	}
	if newObj != nil {
		review.Request.Object.Raw = canItMalform(newObj, isNewMalformed)
	}
	if isDry {
		trueBool := true
		review.Request.DryRun = &trueBool
	}
	httpRequest := http.Request{}
	if oldObj != nil || newObj != nil {
		rawReview, err := json.Marshal(review)
		if err != nil {
			return nil, errors.New("AdmissionReview couldn't be marshalled because:" + err.Error())
		}
		reader := bytes.NewReader(rawReview)
		httpRequest.Body = io.NopCloser(reader)
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

func createAlloc(len uint32) string {
	ba, _ := bitarray.NewBitArray(len + 1)
	return ba.Encode()
}

func exhaustAlloc(alloc string) string {
	ba := bitarray.NewBitArrayFromBase64(alloc)
	var i uint32
	for i = 0; i < ba.Len(); i++ {
		ba.Set(i)
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
