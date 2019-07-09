package confman_test

import (
  "testing"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/bitarray"
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

const (
  AllocFor5k = "gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
)

var (
  emptyTconfs []danmtypes.TenantConfig
  errorTconfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "error"}},
  }
  multipleTconfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "firstConf"}},
    danmtypes.TenantConfig{ObjectMeta: meta_v1.ObjectMeta {Name: "secondConf"}},
  }
  reserveConfs = []danmtypes.TenantConfig {
    danmtypes.TenantConfig{
      ObjectMeta: meta_v1.ObjectMeta {Name: "tconf"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc:AllocFor5k},
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vlan", VniRange: "200,500-510", Alloc:AllocFor5k},
      },
    },
    danmtypes.TenantConfig{
      ObjectMeta: meta_v1.ObjectMeta {Name: "error"},
      HostDevices: []danmtypes.IfaceProfile {
        danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "800-810", Alloc:AllocFor5k},
      },
    },
  }
  reserveIfaces = []danmtypes.IfaceProfile {
    danmtypes.IfaceProfile{Name:"invalidVni", VniRange: "invalid"},
    danmtypes.IfaceProfile{Name: "ens4", VniType: "vxlan", VniRange: "700-710", Alloc:AllocFor5k},
    danmtypes.IfaceProfile{Name: "ens4", VniType: "vlan", VniRange: "200,500-510", Alloc:AllocFor5k},
    danmtypes.IfaceProfile{Name: "hupak", VniType: "vlan", VniRange: "1000,1001", Alloc:AllocFor5k},
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

var reserveTcs = []struct {
  tcName string
  tconfName string
  ifaceName string
  vniType string
  reserveVnis []int
  isErrorExpected bool
  expectedVni int
}{
  {"invalidVni", "tconf", "invalidVni", "", nil, true, 0},
  {"reserveFirstFreeInEmptyIface", "tconf", "ens4", "vlan", nil, false, 200},
  {"reserveLastFreeInIface", "tconf", "ens4", "vlan", []int{200,509}, false, 510},
  {"noFreeVniInIface", "tconf", "ens4", "vlan", []int{200,510}, true, 0},
  {"errorUpdating", "error", "ens4", "vxlan", nil, true, 0},
  {"nonExistentProfile", "tconf", "hupak", "vlan", nil, true, 0},
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

func TestReserve(t *testing.T) {
  for _, tc := range reserveTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      tconf := getTconf(tc.tconfName, reserveConfs)
      iface := getIface(tc.ifaceName, tc.vniType, reserveIfaces)
      if tc.reserveVnis != nil {
        reserveVnis(&iface,tc.reserveVnis)
      }
      testArtifacts := stubs.TestArtifacts{TestTconfs: reserveConfs}
      tconfClientStub := stubs.NewClientSetStub(testArtifacts)
      vni, err := confman.Reserve(tconfClientStub, tconf, iface)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%v does not match with expectation", err)
        return
      }
      if tc.expectedVni != 0 {
        if tc.expectedVni != vni {
          t.Errorf("Received reserved VNI:%d does not match with expected:%d",vni,tc.expectedVni)
          return
        }
        _, updatedIface := getIfaceFromTconf(tc.ifaceName, tc.vniType, tconf)
        if updatedIface.Alloc == iface.Alloc {
          t.Errorf("Alloc field in the selected inteface profile did not change even though a VNI was reserved!")
          return
        }
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

func getTconf(tconfName string, tconfSet []danmtypes.TenantConfig) *danmtypes.TenantConfig {
  for _, tconf := range tconfSet {
    if tconf.ObjectMeta.Name == tconfName {
      return &tconf
    }
  }
  return nil
}

func getIface(ifaceName string, vniType string, ifaceSet []danmtypes.IfaceProfile) danmtypes.IfaceProfile {
  for _, iface := range ifaceSet {
    if iface.Name == ifaceName && iface.VniType == vniType{
      return iface
    }
  }
  return danmtypes.IfaceProfile{}
}

func getIfaceFromTconf(ifaceName string, vniType string, tconf *danmtypes.TenantConfig) (int,danmtypes.IfaceProfile) {
  for index, iface := range tconf.HostDevices {
    if iface.Name == ifaceName && iface.VniType == vniType {
      return index, iface
    }
  }
  return -1, danmtypes.IfaceProfile{}
}

func reserveVnis(iface *danmtypes.IfaceProfile, vniRange []int) {
  allocs := bitarray.NewBitArrayFromBase64(iface.Alloc)
  for i := vniRange[0]; i <= vniRange[1]; i++ {
    allocs.Set(uint32(i))
  }
  iface.Alloc = allocs.Encode()
}