package danm

import (
  "errors"
  "net"
  "strings"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  types "k8s.io/apimachinery/pkg/types"
  watch "k8s.io/apimachinery/pkg/watch"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/ipam"
  "github.com/nokia/danm/test/utils"
)

const (
  magicVersion = "42"
)

type NetClientStub struct{
  TestNets []danmtypes.DanmNet
  ReservedIpsList []utils.ReservedIpsList
  TimesUpdateWasCalled int
}

func newNetClientStub(nets []danmtypes.DanmNet, ips []utils.ReservedIpsList) *NetClientStub {
  return &NetClientStub{TestNets: nets, ReservedIpsList: ips}
}

func (netClient *NetClientStub) Create(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  return nil, nil
}

func (netClient *NetClientStub) Update(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  netClient.TimesUpdateWasCalled++
  for _, netReservation := range netClient.ReservedIpsList {
    if obj.Spec.NetworkID == netReservation.NetworkId {
      ba := bitarray.NewBitArrayFromBase64(obj.Spec.Options.Alloc)
      _, ipnet, _ := net.ParseCIDR(obj.Spec.Options.Cidr)
      ipnetNum := ipam.Ip2int(ipnet.IP)
      for _, reservation := range netReservation.Reservations {
        ip,_,err := net.ParseCIDR(reservation.Ip)
        if err != nil {
          continue
        }
        ipInInt := ipam.Ip2int(ip) - ipnetNum
        if !ipnet.Contains(ip) {
          continue
        }
        if !ba.Get(uint32(ipInInt)) && reservation.Set {
          return nil, errors.New("Reservation failure, IP:" + reservation.Ip + " should have been reserved in DanmNet:" + obj.Spec.NetworkID)
        }
        if ba.Get(uint32(ipInInt)) && !reservation.Set {
          return nil, errors.New("Reservation failure, IP:" + reservation.Ip + " should have been free in DanmNet:" + obj.Spec.NetworkID)
        }
      }
    }
  }
  var netIndex int
  for index, net := range netClient.TestNets {
    if net.ObjectMeta.Name == obj.ObjectMeta.Name {
      netIndex = index
    }
  }
  if strings.Contains(obj.Spec.NetworkID, "conflict") && obj.ObjectMeta.ResourceVersion != magicVersion {
    netClient.TestNets[netIndex].ObjectMeta.ResourceVersion = magicVersion
    return nil, errors.New(datastructs.OptimisticLockErrorMsg)
  }
  if strings.Contains(obj.Spec.NetworkID, "error") {
    return nil, errors.New("fatal error, don't retry")
  }
  netClient.TestNets[netIndex] = *obj
  return obj, nil
}

func (netClient *NetClientStub) Delete(name string, options *meta_v1.DeleteOptions) error {
  return nil
}

func (netClient *NetClientStub) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
  return nil
}

func (netClient *NetClientStub) Get(netName string, options meta_v1.GetOptions) (*danmtypes.DanmNet, error) {
  if strings.Contains(netName, "error") {
    return nil, errors.New("fatal error, don't retry")
  }
  for _, testNet := range netClient.TestNets {
    if testNet.ObjectMeta.Name == netName {
      return &testNet, nil
    }
  }
  return nil, errors.New("let's test error case as well")
}

func (netClient *NetClientStub) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
  watch := watch.NewEmptyWatch()
  return watch, nil
}

func (netClient *NetClientStub) List(opts meta_v1.ListOptions) (*danmtypes.DanmNetList, error) {
  return nil, nil
}

func (netClient *NetClientStub) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *danmtypes.DanmNet, err error) {
  return nil, nil
}

func (netClient *NetClientStub) AddReservedIpsList(reservedIps []utils.ReservedIpsList) {
  netClient.ReservedIpsList = reservedIps
}
