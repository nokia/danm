package confman_test

import (
  "testing"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/confman"
  "github.com/nokia/danm/test/stubs"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TconfSets struct {
  sets []TconfSet
}

type TconfSet struct {
  name string
  tconfs []danmtypes.TenantConfig
}

var (
  emptyTconfs []danmtypes.TenantConfig
  errorTconfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "error"}},
  }
  multipleTconfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "firstConf"}},
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "secondConf"}},
  }
  tconfSets = []TconfSet {
    TconfSet{name: "emptyTcs", tconfs: emptyTconfs},
    TconfSet{name: "errorTconfs", tconfs: errorTconfs},
    TconfSet{name: "multipleConfigs", tconfs: multipleTconfs},
  }
  testConfigs = TconfSets {sets: tconfSets}
)

var getTconfTcs = []struct {
  tcName string
  tconfSets TconfSets
  isErrorExpected bool
}{
  {"emptyTcs", testConfigs, true},
  {"errorTconfs", testConfigs, true},
  {"multipleConfigs", testConfigs, false},
}

func TestGetTenantConfig(t *testing.T) {
  for _, tc := range getTconfTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      tconfSet := getTconfSet(tc.tcName, tc.tconfSets.sets)
      testArtifacts := stubs.TestArtifacts{TestTconfs: tconfSet}
      tconfClientStub := stubs.NewClientSetStub(testArtifacts)
      tconf, err := confman.GetTenantConfig(tconfClientStub)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%v does not match with expectation", err)
        return
      }
      if tconf != nil && tconf.ObjectMeta.Name != tconfSet[0].ObjectMeta.Name {
        t.Errorf("The name of the returned TenantConfig:%s does not match with the expected:%s", tconf.ObjectMeta.Name, tconfSet[0].ObjectMeta.Name)
      }
    })
  }
}

func getTconfSet(tconfName string, tconfSets []TconfSet) []danmtypes.TenantConfig {
  for _, tconfSet := range tconfSets {
    if tconfSet.name == tconfName {
      return tconfSet.tconfs
    }
  }
  return nil
}