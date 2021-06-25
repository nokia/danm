// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package danm

import (
  discovery "k8s.io/client-go/discovery"
  danmv1 "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  "github.com/nokia/danm/test/utils"
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

func NewClientSetStub(objects utils.TestArtifacts) *ClientSetStub {
  var clientSet ClientSetStub
  clientSet.DanmClient = newClientStub(objects)
  return &clientSet
}