package danm

import (
  discovery "k8s.io/client-go/discovery"
  danmv1 "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
)

type ClientSetStub struct {
  DanmClient *ClientStub
}

func (c *ClientSetStub) DanmV1() danmv1.DanmV1Interface {
  return c.DanmClient
}

func (c *ClientSetStub) Danm() danmv1.DanmV1Interface {
  return c.DanmClient
}

func (c *ClientSetStub) Discovery() discovery.DiscoveryInterface {
  return nil
}

func NewClientSetStub(objects TestArtifacts) *ClientSetStub {
  var clientSet ClientSetStub
  clientSet.DanmClient = newClientStub(objects)
  return &clientSet
}