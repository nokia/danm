package cnidel_test

import (
  "testing"
  "github.com/nokia/danm/pkg/cnidel"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/test/stubs"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testNets = []danmtypes.DanmNet {
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "empty"},
    Spec:       danmtypes.DanmNetSpec{NetworkID: "empty", NetworkType: ""},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "ipvlan"},
    Spec: danmtypes.DanmNetSpec{NetworkID: "ipvlan", NetworkType: "ipvlan"},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "IPVLAN"},
    Spec: danmtypes.DanmNetSpec{NetworkID: "IPVLAN", NetworkType: "IPVLAN"},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "sriov"},  
    Spec: danmtypes.DanmNetSpec{NetworkID: "sriov",NetworkType: "sriov"},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "flannel"},  
    Spec: danmtypes.DanmNetSpec{NetworkID: "flannel", NetworkType: "flannel"},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "hululululu"},  
    Spec: danmtypes.DanmNetSpec{NetworkID: "hululululu", NetworkType: "hululululu"},
  },
  danmtypes.DanmNet{ 
    Spec: danmtypes.DanmNetSpec{NetworkID: "nometa", NetworkType: "macvlan"},
  },
}

var delegationRequiredTcs = []struct {
  netName string
  isErrorExpected bool
  isDelegationExpected bool
}{
  {"empty", false, false},
  {"ipvlan", false, false},
  {"sriov", false, true},
  {"flannel", false, true},
  {"hululululu", false, true},
  {"error", true, false},
  {"nometa", true, false},
}

var isDeviceNeededTcs = []struct {
  BackendName string
  deviceNeeded bool
}{
  {"sriov", true},
  {"macvlan", false},
  {"neverhas", false},
}

func TestIsDelegationRequired(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil, nil)
  for _, tc := range delegationRequiredTcs {
    t.Run(tc.netName, func(t *testing.T) {
      isDelRequired,_,err := cnidel.IsDelegationRequired(netClientStub,tc.netName,"hululululu")
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        var detailedErrorMessage string
        if err != nil {
          detailedErrorMessage = err.Error()
        }
        t.Errorf("Received error does not match with expectation: %t for TC: %s, detailed error message: %s", tc.isErrorExpected, tc.netName, detailedErrorMessage)
      }
      if isDelRequired != tc.isDelegationExpected {
        t.Errorf("Received delegation result does not match with expected for TC: %s", tc.netName)
      }
    })
  }
}

func TestIsDeviceNeeded(t *testing.T) {
  for _, tc := range isDeviceNeededTcs {
    isDevNeeded := cnidel.IsDeviceNeeded(tc.BackendName)
    if isDevNeeded != tc.deviceNeeded {
      t.Errorf("Received device needed result does not match with expected")
    }
  }
}
