package stubs

import (
  "errors"
  "strings"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  types "k8s.io/apimachinery/pkg/types"
  watch "k8s.io/apimachinery/pkg/watch"
)
  
type TconfClientStub struct{
  testTconfs []danmtypes.TenantConfig
}

func newTconfClientStub(tconfs []danmtypes.TenantConfig) TconfClientStub {
  return TconfClientStub{testTconfs: tconfs}
}
  
func (tconfClient TconfClientStub) Create(obj *danmtypes.TenantConfig) (*danmtypes.TenantConfig, error) {
  return nil, nil
}

func (tconfClient TconfClientStub) Update(obj *danmtypes.TenantConfig) (*danmtypes.TenantConfig, error) {
  if strings.HasPrefix(obj.ObjectMeta.Name,"error") {
    return nil, errors.New("here you go")
  }
  return &danmtypes.TenantConfig{}, nil
}

func (tconfClient TconfClientStub) Delete(name string, options *meta_v1.DeleteOptions) error {
  return nil
}

func (tconfClient TconfClientStub) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
  return nil
}

func (tconfClient TconfClientStub) Get(tconfNameName string, options meta_v1.GetOptions) (*danmtypes.TenantConfig, error) {
  for _, tconf := range tconfClient.testTconfs {
    if tconf.ObjectMeta.Name == tconfNameName {
      return &tconf, nil
    }
  }
  return nil, nil
}

func (tconfClient TconfClientStub) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
  watch := watch.NewEmptyWatch()
  return watch, nil
}

func (tconfClient TconfClientStub) List(opts meta_v1.ListOptions) (*danmtypes.TenantConfigList, error) {
  if tconfClient.testTconfs == nil {
    return nil, nil
  }
  if strings.HasPrefix(tconfClient.testTconfs[0].ObjectMeta.Name,"error") {
    return nil, errors.New("error happened")
  }
  tconfList := danmtypes.TenantConfigList{Items: tconfClient.testTconfs }
  return &tconfList, nil
}

func (tconfClient TconfClientStub) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *danmtypes.TenantConfig, err error) {
  return nil, nil
}

