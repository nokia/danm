package cnidel_test

import (
  "testing"
  "github.com/nokia/danm/pkg/cnidel"
  danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/stubs"
)

var testNets = []danmtypes.DanmNet {
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "empty", NetworkType: ""} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "ipvlan", NetworkType: "ipvlan"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "IPVLAN", NetworkType: "IPVLAN"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "sriov",NetworkType: "sriov"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "flannel", NetworkType: "flannel"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "hululululu", NetworkType: "hululululu"} },
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
}

func TestIsDelegationRequired(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil)
  for _, tc := range delegationRequiredTcs {
    t.Run(tc.netName, func(t *testing.T) {
      isDelRequired,_,err := cnidel.IsDelegationRequired(netClientStub,tc.netName,"hululululu")
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error does not match with expectation: %b", tc.isErrorExpected)
      }
      if isDelRequired != tc.isDelegationExpected {
        t.Errorf("Received delegation result does not match with expected")
      }
    })
  }
}
