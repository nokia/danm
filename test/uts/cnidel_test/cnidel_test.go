package cnidel_test

import (
  "os"
  "strings"
  "testing"
  "io/ioutil"
  "path/filepath"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/cnidel"
  "github.com/nokia/danm/test/stubs"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
  cniTestConfigDir = "/etc/cni/net.d"
  cniTestConfigFile = "cnitest.conf"
)

var (
  cniTesterDir = cniTestConfigDir
)

type CniConf struct {
  ConfName string
  Conftent []byte
}

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
    ObjectMeta: meta_v1.ObjectMeta {Name: "IPVLAN-UPPER"},
    Spec: danmtypes.DanmNetSpec{NetworkID: "IPVLAN-UPPER", NetworkType: "IPVLAN"},
  },
  danmtypes.DanmNet{
    ObjectMeta: meta_v1.ObjectMeta {Name: "sriov"},  
    Spec: danmtypes.DanmNetSpec{NetworkID: "sriov", NetworkType: "sriov"},
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
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "ipamNeeded"}, 
    Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "cidr", Validation: true,},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "flannel-test"}, 
    Spec: danmtypes.DanmNetSpec{NetworkType: "flannel", NetworkID: "flannel_conf", Validation: true,},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "no-conf"}, 
    Spec: danmtypes.DanmNetSpec{NetworkType: "flannel", NetworkID: "hulululu", Validation: true,},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "no-binary"}, 
    Spec: danmtypes.DanmNetSpec{NetworkType: "flanel", NetworkID: "flannel_conf", Validation: true,},
  },
}

var expectedCniConfigs = []CniConf {
  {"flannel", []byte(`{"cniexp":{"cnitype":"flannel"},"cniconf":{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}}`)},
  {"flannel-ip", []byte(`{"cniexp":{"cnitype":"flannel","ip":"10.244.10.30/24"},"cniconf":{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}}`)},
}

var testCniConfFiles = []CniConf {
  {"flannel_conf.conf", []byte(`{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}`)},
}

var testEps = []danmtypes.DanmEp {
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "dynamicIpv4"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Address: "dynamic",},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "noIps"}, Spec: danmtypes.DanmEpSpec{Iface: danmtypes.DanmEpIface{Name: "eth0"}},
  },
}

var delegationRequiredTcs = []struct {
  netName string
  isErrorExpected bool
  isDelegationExpected bool
}{
  {"empty", false, false},
  {"ipvlan", false, false},
  {"IPVLAN-UPPER", false, false},
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

var delSetupTcs = []struct {
  tcName string
  netName string
  epName string
  cniConfName string
  expectedIp string
  isErrorExpected bool
}{
  {"ipamNeededError", "ipamNeeded", "dynamicIpv4", "", "", true},
  {"emptyIpamconfigError", "ipamNeeded", "noIps", "", "", true},
  {"staticCniSuccess", "flannel-test", "noIps", "flannel", "", false},
  {"staticCniNoConfig", "no-conf", "noIps", "", "", true},
  {"staticCniNoBinary", "no-binary", "noIps", "flannel", "", true},
  {"staticCniWithIp", "flannel-test", "noIps", "flannel-ip", "10.244.10.30", false},
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

func TestGetEnv(t *testing.T) {
  testEnvKey := "HOTEL"
  testEnvVal := "trivago"
  os.Setenv(testEnvKey, testEnvVal)
  defer os.Unsetenv(testEnvKey)
  existingValue := cnidel.GetEnv(testEnvKey, "booking")
  if existingValue != testEnvVal {
    t.Errorf("Received value for set environment variable : %s does not match with expected: %s", existingValue, testEnvVal)
  }
  defaultValue := cnidel.GetEnv("TROLOLOLO", testEnvVal)
  if defaultValue != testEnvVal {
    t.Errorf("Received value for unset environment variable: %s does not match with expected: %s", defaultValue, testEnvVal)
  }
}

func TestCalculateIfaceName(t *testing.T) {
  testChosenName := "thechosenone"
  testDefaultName := "notthechosenone"
  ifaceName := cnidel.CalculateIfaceName(testChosenName, testDefaultName)
  if ifaceName != testChosenName {
    t.Errorf("Received value for explicitly set interface name: %s does not match with expected: %s", ifaceName, testChosenName)
  }
  defIfaceName := cnidel.CalculateIfaceName("", testDefaultName)
  if defIfaceName != testDefaultName {
    t.Errorf("Received value for default interface name: %s does not match with expected: %s", defIfaceName, testChosenName)
  }
}

func TestDelegateInterfaceSetup(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil, nil)
  err := setupDelTest()
  if err != nil {
    t.Errorf("Test suite could not be set-up because:%s", err.Error())
  }
  for _, tc := range delSetupTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      err = setupDelTestTc(tc.cniConfName)
      if err != nil {
        t.Errorf("TC could not be set-up because:%s", err.Error())
      }
      testNet := getTestNet(tc.netName)
      testEp := getTestEp(tc.epName)
      cniRes, err := cnidel.DelegateInterfaceSetup(netClientStub,testNet,testEp)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        var detailedErrorMessage string
        if err != nil {
          detailedErrorMessage = err.Error()
        }
        t.Errorf("Received error does not match with expectation: %t for TC: %s, detailed error message: %s", tc.isErrorExpected, tc.tcName, detailedErrorMessage)
      }
      if tc.expectedIp != "" {
        if cniRes == nil {
          t.Errorf("CNI Result cannot be empty when we expect an IP!")  
        }
        if strings.HasPrefix(tc.expectedIp, testEp.Spec.Iface.Address) {
          t.Errorf("Expected IP:%s is not saved in DanmEp.Spec.Iface's respective address field:%s", tc.expectedIp, testEp.Spec.Iface.Address)  
        }
      }
    })
  }
}

func setupDelTest() error {
  os.RemoveAll(cniTestConfigDir)
  err := os.MkdirAll(cniTestConfigDir, os.ModePerm)
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_PATH", cniTesterDir)
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_COMMAND", "ADD")
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_CONTAINERID", "12346")
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_NETNS", "argsdfhtz")
  if err != nil {
    return err
  }
  os.RemoveAll(filepath.Join(cniTesterDir, "flannel"))
  input, err := ioutil.ReadFile(filepath.Join(os.Getenv("GOPATH"),"bin","cnitest"))
  if err != nil {
    return err
  }
  err = ioutil.WriteFile(filepath.Join(cniTesterDir, "flannel"), input, 777)
  if err != nil {
    return err
  }
  for _, confFile := range testCniConfFiles {
    err = ioutil.WriteFile(filepath.Join(cniTestConfigDir, confFile.ConfName), confFile.Conftent, 0666)
    if err != nil {
      return err
    }
  }
  return nil
}

func setupDelTestTc(expectedCniConfig string) error {
  var expectedConf CniConf
  for _, conf := range expectedCniConfigs {
    if conf.ConfName == expectedCniConfig {
      expectedConf = conf
      break
    }
  }
  os.Remove(filepath.Join(cniTestConfigDir, cniTestConfigFile))
  err := ioutil.WriteFile(filepath.Join(cniTestConfigDir, cniTestConfigFile), expectedConf.Conftent, 0666)
  if err != nil {
    return err
  }
  return nil
}

func getTestNet(netId string) *danmtypes.DanmNet {
  for _, net := range testNets {
    if net.ObjectMeta.Name == netId {
      return &net
    }
  }
  return nil
}

func getTestEp(epId string) *danmtypes.DanmEp {
  for _, ep := range testEps {
    if ep.ObjectMeta.Name == epId {
      return &ep
    }
  }
  return nil
}