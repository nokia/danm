package stubs

import (
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  types "k8s.io/apimachinery/pkg/types"
  watch "k8s.io/apimachinery/pkg/watch"
)
  
type EpClientStub struct{
  testEps []danmtypes.DanmEp
}

func newEpClientStub(eps []danmtypes.DanmEp) EpClientStub {
  return EpClientStub{testEps: eps}
}
  
func (epClient EpClientStub) Create(obj *danmtypes.DanmEp) (*danmtypes.DanmEp, error) {
  return nil, nil
}

func (epClient EpClientStub) Update(obj *danmtypes.DanmEp) (*danmtypes.DanmEp, error) {
  return nil, nil
}

func (epClient EpClientStub) Delete(name string, options *meta_v1.DeleteOptions) error {
  return nil
}

func (epClient EpClientStub) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
  return nil
}

func (epClient EpClientStub) Get(epName string, options meta_v1.GetOptions) (*danmtypes.DanmEp, error) {
  for _, testNet := range epClient.testEps {
    if testNet.Spec.NetworkName == epName {
      return &testNet, nil
    }
  }
  return nil, nil
}

func (epClient EpClientStub) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
  watch := watch.NewEmptyWatch()
  return watch, nil
}

func (epClient EpClientStub) List(opts meta_v1.ListOptions) (*danmtypes.DanmEpList, error) {
  return nil, nil
}

func (epClient EpClientStub) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *danmtypes.DanmEp, err error) {
  return nil, nil
}

