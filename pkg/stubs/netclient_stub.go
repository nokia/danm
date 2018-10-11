package stubs

import (
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
  types "k8s.io/apimachinery/pkg/types"
  watch "k8s.io/apimachinery/pkg/watch"
)
  
type NetClientStub struct{
  testNets []danmtypes.DanmNet
}

func newNetClientStub(nets []danmtypes.DanmNet) NetClientStub {
  return NetClientStub{testNets: nets}
}
  
func (netClient NetClientStub) Create(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  return nil, nil
}

func (netClient NetClientStub) Update(obj *danmtypes.DanmNet) (*danmtypes.DanmNet, error) {
  return nil, nil
}

func (netClient NetClientStub) Delete(name string, options *meta_v1.DeleteOptions) error {
  return nil
}

func (netClient NetClientStub) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
  return nil
}

func (netClient NetClientStub) Get(netName string, options meta_v1.GetOptions) (*danmtypes.DanmNet, error) {
  for _, testNet := range netClient.testNets {
    if testNet.Spec.NetworkID == netName {
      return &testNet, nil
    }
  }
  return nil, nil
}

func (netClient NetClientStub) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
  watch := watch.NewEmptyWatch()
  return watch, nil
}

func (netClient NetClientStub) List(opts meta_v1.ListOptions) (*danmtypes.DanmNetList, error) {
  return nil, nil
}

func (netClient NetClientStub) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *danmtypes.DanmNet, err error) {
  return nil, nil
}

