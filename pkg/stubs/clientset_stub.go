package stubs

import (
  discovery "k8s.io/client-go/discovery"
  danmv1 "github.com/nokia/danm/pkg/crd/client/clientset/versioned/typed/danm/v1"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
)

type ClientSetStub struct {
  danmClient *ClientStub
}

func (c *ClientSetStub) DanmV1() danmv1.DanmV1Interface {
  return c.danmClient
}

func (c *ClientSetStub) Danm() danmv1.DanmV1Interface {
  return c.danmClient
}

func (c *ClientSetStub) Discovery() discovery.DiscoveryInterface {
  return nil
}

func NewClientSetStub(nets []danmtypes.DanmNet, eps []danmtypes.DanmEp) *ClientSetStub {
  var clientSet ClientSetStub
  clientSet.danmClient = newClientStub(nets, eps)
  return &clientSet
}