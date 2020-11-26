package ipam

import (
    "fmt"

    danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
    danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
)

type ConditionFunc func(danmClient danmclientset.Interface, ep *danmtypes.DanmEp, dnet *danmtypes.DanmNet) bool
type ReleaseFunc func(danmClient danmclientset.Interface, ep *danmtypes.DanmEp, dnet *danmtypes.DanmNet) error

type ipamHandler struct {
    ConditionFunc ConditionFunc
    ReleaseFunc   ReleaseFunc
}

func NewIpamHandler(conditionFunc ConditionFunc, releaseFunc ReleaseFunc) *ipamHandler {
    return &ipamHandler{ConditionFunc: conditionFunc, ReleaseFunc: releaseFunc}
}

var CniHandlers = map[string]*ipamHandler {
    "danm": NewIpamHandler(DanmIpamHandlerConditionFunc, DanmIpamHandlerReleaseFunc),
}

// DanmIpamHandlerConditionFunc tells if IP address was allocated by DANM IPAM
// IP was allocated by DANM only if it falls into any of the defined subnets
func DanmIpamHandlerConditionFunc(danmClient danmclientset.Interface, ep *danmtypes.DanmEp, dnet *danmtypes.DanmNet) bool {
    return WasIpAllocatedByDanm(ep.Spec.Iface.Address, dnet.Spec.Options.Cidr) || WasIpAllocatedByDanm(ep.Spec.Iface.AddressIPv6, dnet.Spec.Options.Pool6.Cidr)
}

//DanmIpamHandlerReleaseFunc
func DanmIpamHandlerReleaseFunc(danmClient danmclientset.Interface, ep *danmtypes.DanmEp, dnet *danmtypes.DanmNet) error {
    err := GarbageCollectIps(danmClient, dnet, ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
    if err != nil {
        return fmt.Errorf("DanmEp: %s cannot be safely deleted because freeing its reserved IP addresses failed with error: %s", ep.ObjectMeta.Name, err)
    }
    return nil
}

func FindAndCallFirstIpamReleaseHandlerIfAny(danmClient danmclientset.Interface, ep *danmtypes.DanmEp, dnet *danmtypes.DanmNet) error {
    for _, handler := range CniHandlers {
        if handler.ConditionFunc(danmClient, ep, dnet) {
            return handler.ReleaseFunc(danmClient, ep, dnet)
        }
    }
    return nil
}
