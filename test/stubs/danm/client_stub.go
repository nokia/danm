package danm

import (
  client "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  rest "k8s.io/client-go/rest"
)

type TestArtifacts struct {
  TestNets []danmtypes.DanmNet
  TestEps []danmtypes.DanmEp
  ReservedIps []ReservedIpsList
  TestTconfs []danmtypes.TenantConfig
}

type ReservedIpsList struct {
  NetworkId string
  Reservations []Reservation
}

type Reservation struct {
  Ip string
  Set bool
}

type ClientStub struct {
  Objects TestArtifacts
}

func (client *ClientStub) DanmNets(namespace string) client.DanmNetInterface {
  return newNetClientStub(client.Objects.TestNets, client.Objects.ReservedIps)
}

func (client *ClientStub) DanmEps(namespace string) client.DanmEpInterface {
  return newEpClientStub(client.Objects.TestEps)
}

func (client *ClientStub) TenantConfigs() client.TenantConfigInterface {
  return newTconfClientStub(client.Objects.TestTconfs)
}

func (client *ClientStub) TenantNetworks(namespace string) client.TenantNetworkInterface {
  return nil
}

func (client *ClientStub) ClusterNetworks() client.ClusterNetworkInterface {
  return nil
}

func (c *ClientStub) RESTClient() rest.Interface {
  return nil
}

func newClientStub(ta TestArtifacts) *ClientStub {
  return &ClientStub {
    Objects: ta,
  }
}
