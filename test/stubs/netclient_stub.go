package stubs

import (
  "errors"
  "net"
  "strings"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  types "k8s.io/apimachinery/pkg/types"
  watch "k8s.io/apimachinery/pkg/watch"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
  "github.com/nokia/danm/pkg/netcontrol"
)
const (
  magicVersion = "42"
)

type NetClientStub struct{
  testNets []danmtypes.DanmNet
  reservedIpsList []ReservedIpsList
}

func newNetClientStub(nets []danmtypes.DanmNet, ips []ReservedIpsList) *NetClientStub {
  return &NetClientStub{testNets: nets, reservedIpsList: ips}
}

func (netClient *NetClientStub) Create(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  return nil, nil
}

func (netClient *NetClientStub) Update(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  for _, netReservation := range netClient.reservedIpsList {
    if obj.Spec.NetworkID == netReservation.NetworkId {
      ba := bitarray.NewBitArrayFromBase64(obj.Spec.Options.Alloc)
      _, ipnet, _ := net.ParseCIDR(obj.Spec.Options.Cidr)
      ipnetNum := netcontrol.Ip2int(ipnet.IP)
      for _, reservation := range netReservation.Reservations {
        ip,_,err := net.ParseCIDR(reservation.Ip)
        if err != nil {
          continue
        }
        ipInInt := netcontrol.Ip2int(ip) - ipnetNum
        if !ipnet.Contains(ip) {
          continue
        }
        if !ba.Get(uint32(ipInInt)) && reservation.Set {
          return nil, errors.New("Reservation failure, IP:" + reservation.Ip + " must be reserved in DanmNet:" + obj.Spec.NetworkID)
        }
        if ba.Get(uint32(ipInInt)) && !reservation.Set {
          return nil, errors.New("Reservation failure, IP:" + reservation.Ip + " must be free in DanmNet:" + obj.Spec.NetworkID)
        }
      }
    }
  }
  if strings.Contains(obj.Spec.NetworkID, "conflict") && obj.ObjectMeta.ResourceVersion != magicVersion {
    for index, net := range netClient.testNets {
      if net.Spec.NetworkID == obj.Spec.NetworkID {
        netClient.testNets[index].ObjectMeta.ResourceVersion = magicVersion
      }
    }
    return nil, errors.New(danmtypes.OptimisticLockErrorMsg)
  }
  if strings.Contains(obj.Spec.NetworkID, "error") {
    return nil, errors.New("fatal error, don't retry")
  }
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
  for _, testNet := range netClient.testNets {
    if testNet.Spec.NetworkID == netName {
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

func (netClient *NetClientStub) AddReservedIpsList(reservedIps []ReservedIpsList) {
  netClient.reservedIpsList = reservedIps
}
