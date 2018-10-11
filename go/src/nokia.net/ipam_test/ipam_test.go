package ipam_test

import (
  "testing"
  "os"
  "nokia.net/ipam"
  danmtypes "nokia.net/crd/apis/danm/v1"
  "nokia.net/stubs"
)

var testNets = []danmtypes.DanmNet {
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "emptyVal", Validation: ""} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "falseValLower", Validation: "false"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "falseValUpper", Validation: "FALSE"} },
  danmtypes.DanmNet {Spec: danmtypes.DanmNetSpec{NetworkID: "emptyNet", Validation: "TRUE"} },
}

var reserveTcs = []struct {
  netName string
  netInfo danmtypes.DanmNet
  requestedIp4 string
  requestedIp6 string
  expectedIp4 string
  expectedIp6 string
  isErrorExpected bool
  isMacExpected bool
}{
  {"emptyVal", testNets[0], "", "", "", "", true, false},
  {"falseValLower", testNets[1], "", "", "", "", true, false},
  {"falseValUpper", testNets[2], "", "", "", "", true, false},
  {"noIpsRequested", testNets[3], "", "", "", "", false, true},
}

func TestReserve(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil)
  for _, tc := range reserveTcs {
    t.Run(tc.netName, func(t *testing.T) {
      ip4, ip6, mac, err := ipam.Reserve(netClientStub, tc.netInfo, tc.requestedIp4, tc.requestedIp6)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        t.Errorf("Received error:%s does not match with expectation", err.Error())
        return
      }
      if tc.isMacExpected {
        if mac == "" {
          t.Errorf("MAC address was expected to be returned, however it was not")
        }
      }
      if ip4 != tc.expectedIp4 {
        t.Errorf("Allocated IP4 address:%s does not match with expected:%s", ip4, tc.expectedIp4)
      }
      if ip6 != tc.expectedIp6 {
        t.Errorf("Allocated IP6 address:%s does not match with expected:%s", ip6, tc.expectedIp6)
      }
    })
  }
}

func TestMain(m *testing.M) {
  code := m.Run() 
  os.Exit(code)
}

