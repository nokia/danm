package stubs

import (
  client "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  rest "k8s.io/client-go/rest"
)

type ClientStub struct {
  testNets []danmtypes.DanmNet
  testEps []danmtypes.DanmEp
  reservedIps []ReservedIpsList
}

func (client *ClientStub) DanmNets(namespace string) client.DanmNetInterface {
  return newNetClientStub(client.testNets, client.reservedIps)
}

func (client *ClientStub) DanmEps(namespace string) client.DanmEpInterface {
  return newEpClientStub(client.testEps)
}

func (client *ClientStub) TenantConfigs() client.TenantConfigInterface {
  return nil
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

func newClientStub(nets []danmtypes.DanmNet, eps []danmtypes.DanmEp, ips []ReservedIpsList) *ClientStub {
  return &ClientStub {
    testNets: nets,
    testEps: eps,
    reservedIps: ips,
  }
}
