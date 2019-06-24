package cnidel_test

import (
  "os"
  "strconv"
  "strings"
  "testing"
  "io/ioutil"
  "path/filepath"
  sriov_utils "github.com/intel/sriov-cni/pkg/utils"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  "github.com/nokia/danm/pkg/cnidel"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/test/stubs"
  "github.com/nokia/danm/test/utils"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
  cniTestConfigDir = "/etc/cni/net.d"
  cniTestConfigFile = "cnitest.conf"
)

var (
  cniTesterDir = cniTestConfigDir
  defaultDataDir = "/var/lib/cni/networks"
  flannelBridge = "cbr0"
  cniConf = datastructs.NetConf{CniConfigDir: "/etc/cni/net.d"}
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
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "ipamNeeded"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "macvlan", NetworkID: "cidr",},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "flannel-test"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "flannel", NetworkID: "flannel_conf",},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "no-conf"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "flannel", NetworkID: "hulululu",},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "no-binary"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "flanel", NetworkID: "flannel_conf",},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "macvlan-v4"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "macvlan", NetworkID: "macvlan", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Device: "ens1f0"}},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "macvlan-v6"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "macvlan", NetworkID: "macvlan", Options: danmtypes.DanmNetOption{Net6: "2a00:8a00:a000:1193::/64", Device: "ens1f1"}},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "macvlan-ds"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "macvlan", NetworkID: "macvlan", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Net6: "2a00:8a00:a000:1193::/64", Device: "ens1f1"}},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "sriov-test"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "sriov", NetworkID: "sriov-test", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Vlan: 500}},
  },
  danmtypes.DanmNet {
    ObjectMeta: meta_v1.ObjectMeta {Name: "full-macvlan"},
    Spec: danmtypes.DanmNetSpec{NetworkType: "macvlan", NetworkID: "full", Options: danmtypes.DanmNetOption{Cidr: "192.168.1.64/26", Device: "ens1f0"}},
  },
}

var expectedCniConfigs = []CniConf {
  {"flannel", []byte(`{"cniexp":{"cnitype":"flannel"},"cniconf":{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}}`)},
  {"flannel-ip", []byte(`{"cniexp":{"cnitype":"flannel","ip":"10.244.10.30/24","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"eth0"}},"cniconf":{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}}`)},
  {"macvlan-ip4", []byte(`{"cniexp":{"cnitype":"macvlan","ip":"192.168.1.65/26","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"ens1f0"}},"cniconf":{"cniVersion":"0.3.1","master":"ens1f0","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam","ips":[{"ipcidr":"192.168.1.65/26","version":4}]}}}`)},
  {"macvlan-ip6", []byte(`{"cniexp":{"cnitype":"macvlan","ip6":"2a00:8a00:a000:1193::/64","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"ens1f1"}},"cniconf":{"cniVersion":"0.3.1","master":"ens1f1","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam"}}}`)},
  {"macvlan-dual-stack", []byte(`{"cniexp":{"cnitype":"macvlan","ip":"192.168.1.65/26","ip6":"2a00:8a00:a000:1193::/64","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"ens1f1"}},"cniconf":{"cniVersion":"0.3.1","master":"ens1f1","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam","ips":[{"ipcidr":"192.168.1.65/26","version":4}]}}}`)},
  {"macvlan-ip4-type020", []byte(`{"cniexp":{"cnitype":"macvlan","ip":"192.168.1.65/26","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"ens1f0"},"return":"020"},"cniconf":{"cniVersion":"0.3.1","master":"ens1f0","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam","ips":[{"ipcidr":"192.168.1.65/26","version":4}]}}}`)},
  {"macvlan-ip6-type020", []byte(`{"cniexp":{"cnitype":"macvlan","ip6":"2a00:8a00:a000:1193::/64","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"ens1f1"},"return":"020"},"cniconf":{"cniVersion":"0.3.1","master":"ens1f1","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam"}}}`)},
  {"sriov-l3", []byte(`{"cniexp":{"cnitype":"sriov","ip":"192.168.1.65/26","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"eth0"}},"cniconf":{"cniVersion":"0.3.1","name":"sriov-test","type":"sriov","master":"enp175s0f1","l2enable":false,"vlan":500,"deviceID":"0000:af:06.0","ipam":{"type":"fakeipam","ips":[{"ipcidr":"192.168.1.65/26","version":4}]}}}`)},
  {"sriov-l2", []byte(`{"cniexp":{"cnitype":"sriov","env":{"CNI_COMMAND":"ADD","CNI_IFNAME":"eth0"}},"cniconf":{"cniVersion":"0.3.1","name":"sriov-test","type":"sriov","master":"enp175s0f1","l2enable":true,"vlan":500,"deviceID":"0000:af:06.0","ipam":{"type":"fakeipam"}}}`)},
  {"deleteflannel", []byte(`{"cniexp":{"cnitype":"flannel","env":{"CNI_COMMAND":"DEL","CNI_IFNAME":"eth0"}},"cniconf":{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}}`)},
  {"deletemacvlan", []byte(`{"cniexp":{"cnitype":"macvlan","env":{"CNI_COMMAND":"DEL","CNI_IFNAME":"ens1f0"}},"cniconf":{"cniVersion":"0.3.1","master":"ens1f0","mode":"bridge","mtu":1500,"ipam":{"type":"fakeipam"}}}`)},
}

var testCniConfFiles = []CniConf {
  {"flannel_conf.conf", []byte(`{"name":"cbr0","type":"flannel","delegate":{"hairpinMode":true,"isDefaultGateway":true}}`)},
}

var testEps = []danmtypes.DanmEp {
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "dynamicIpv4"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"ens1f0", Address: "dynamic",},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "dynamicIpv6"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"ens1f1", AddressIPv6: "dynamic",},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "dynamicDual"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"ens1f1", Address: "dynamic", AddressIPv6: "dynamic",},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "noIps"}, Spec: danmtypes.DanmEpSpec{Iface: danmtypes.DanmEpIface{Name: "eth0"}},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "dynamicIpv4WithDeviceId"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"eth0", Address: "dynamic", DeviceID: "0000:af:06.0"},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "noneWithDeviceId"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"eth0", Address: "none", DeviceID: "0000:af:06.0"},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "deleteFlannel"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"eth0", Address: "10.244.10.30"},},
  },
  danmtypes.DanmEp{
    ObjectMeta: meta_v1.ObjectMeta {Name: "withAddress"},
    Spec: danmtypes.DanmEpSpec {Iface: danmtypes.DanmEpIface{Name:"ens1f0", Address: "192.168.1.65",},},
  },
}

var delegationRequiredTcs = []struct {
  netName string
  isDelegationExpected bool
}{
  {"empty", false},
  {"ipvlan", false},
  {"IPVLAN-UPPER", false},
  {"sriov", true},
  {"flannel", true},
  {"hululululu", true},
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
  expectedIp6 string
  isErrorExpected bool
}{
  {"ipamNeededError", "ipamNeeded", "dynamicIpv4", "", "", "", true},
  {"emptyIpamconfigError", "ipamNeeded", "noIps", "", "", "", true},
  {"staticCniSuccess", "flannel-test", "noIps", "flannel", "", "", false},
  {"staticCniNoConfig", "no-conf", "noIps", "", "", "", true},
  {"staticCniNoBinary", "no-binary", "noIps", "flannel", "", "", true},
  {"staticCniWithIp", "flannel-test", "noIps", "flannel-ip", "10.244.10.30", "", false},
  {"dynamicMacvlanIpv4", "macvlan-v4", "dynamicIpv4", "macvlan-ip4", "192.168.1.65", "", false},
  {"dynamicMacvlanIpv6", "macvlan-v6", "dynamicIpv6", "macvlan-ip6", "", "2a00:8a00:a000:1193", false},
  {"dynamicMacvlanDualStack", "macvlan-ds", "dynamicDual", "macvlan-dual-stack", "192.168.1.65", "2a00:8a00:a000:1193", false},
  {"dynamicMacvlanIpv4Type020Result", "macvlan-v4", "dynamicIpv4", "macvlan-ip4-type020", "192.168.1.65", "", false},
  {"dynamicMacvlanIpv6Type020Result", "macvlan-v6", "dynamicIpv6", "macvlan-ip6-type020", "", "2a00:8a00:a000:1193", false},
  {"dynamicSriovNoDeviceId", "sriov-test", "dynamicIpv4", "", "", "", true},
  {"dynamicSriovL3", "sriov-test", "dynamicIpv4WithDeviceId", "sriov-l3", "", "", false},
  {"dynamicSriovL2", "sriov-test", "noneWithDeviceId", "sriov-l2", "", "", false},
}

var delDeleteTcs = []struct {
  tcName string
  netName string
  epName string
  cniConfName string
  isErrorExpected bool
}{
  {"flannel", "flannel-test", "deleteFlannel", "deleteflannel", false},
  {"macvlan", "full-macvlan", "withAddress", "deletemacvlan", false},
}

func TestIsDelegationRequired(t *testing.T) {
  for _, tc := range delegationRequiredTcs {
    t.Run(tc.netName, func(t *testing.T) {
      dnet := getTestNet(tc.netName)
      isDelRequired := cnidel.IsDelegationRequired(dnet)
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
  testSequenceId := 4
  expChosenName := testChosenName+strconv.Itoa(testSequenceId)
  ifaceName := cnidel.CalculateIfaceName("", testChosenName, testDefaultName, testSequenceId)
  if ifaceName != expChosenName {
    t.Errorf("Received value for explicitly set interface name: %s does not match with expected: %s", ifaceName, expChosenName)
  }
  expDefName := testDefaultName+strconv.Itoa(testSequenceId)
  defIfaceName := cnidel.CalculateIfaceName("", "", testDefaultName, testSequenceId)
  if defIfaceName != expDefName {
    t.Errorf("Received value for default interface name: %s does not match with expected: %s", defIfaceName, expDefName)
  }
  expFirstNicName := "eth0"
  firstIfaceName := cnidel.CalculateIfaceName("", testChosenName, testDefaultName, 0)
  if firstIfaceName != expFirstNicName {
    t.Errorf("The first interface shall always be named eth0, regardless what the user wants")
  }
  expChosenNameLegacy := testChosenName
  legacyIfaceName := cnidel.CalculateIfaceName(cnidel.LegacyNamingScheme, testChosenName, testDefaultName, testSequenceId)
  if legacyIfaceName != expChosenNameLegacy {
    t.Errorf("Received value for explicitly set interface name: %s does not match with expected: %s when using legacy interface naming scheme", ifaceName, expChosenNameLegacy)
  }
}

func TestDelegateInterfaceSetup(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil, nil)
  err := setupDelTest("ADD")
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
      cniRes, err := cnidel.DelegateInterfaceSetup(&cniConf,netClientStub,testNet,testEp)
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
      if tc.expectedIp6 != "" {
        if cniRes == nil {
          t.Errorf("CNI Result cannot be empty when we expect an IPv6!")
        }
        if strings.HasPrefix(tc.expectedIp6, testEp.Spec.Iface.AddressIPv6) {
          t.Errorf("Expected IP:%s is not saved in DanmEp.Spec.Iface's respective address field:%s", tc.expectedIp6, testEp.Spec.Iface.AddressIPv6)
        }
      }
    })
  }
  err = teardownDelTest()
  if err != nil {
    t.Errorf("Test suite setup could not be reversed because:%s", err.Error())
  }
}

func TestDelegateInterfaceDelete(t *testing.T) {
  netClientStub := stubs.NewClientSetStub(testNets, nil, nil)
  err := setupDelTest("DEL")
  if err != nil {
    t.Errorf("Test suite could not be set-up because:%s", err.Error())
  }
  for _, tc := range delDeleteTcs {
    t.Run(tc.tcName, func(t *testing.T) {
      err = setupDelTestTc(tc.cniConfName)
      if err != nil {
        t.Errorf("TC could not be set-up because:%s", err.Error())
      }
      testNet := getTestNet(tc.netName)
      testEp := getTestEp(tc.epName)
      if testNet.Spec.NetworkType == "flannel" && testEp.Spec.Iface.Address != "" {
        var dataDir = filepath.Join(defaultDataDir, flannelBridge)
        err = os.MkdirAll(dataDir, os.ModePerm)
        if err != nil {
          t.Errorf("Delete TC Flannel prereq could not be set-up because:%s", err.Error())
        }
        _,err = os.Create(filepath.Join(dataDir, testEp.Spec.Iface.Address))
        if err != nil {
          t.Errorf("Delete TC Flannel prereq could not be set-up because:%s", err.Error())
        }
      }
      err := cnidel.DelegateInterfaceDelete(&cniConf,netClientStub,testNet,testEp)
      if (err != nil && !tc.isErrorExpected) || (err == nil && tc.isErrorExpected) {
        var detailedErrorMessage string
        if err != nil {
          detailedErrorMessage = err.Error()
        }
        t.Errorf("Received error does not match with expectation: %t for TC: %s, detailed error message: %s", tc.isErrorExpected, tc.tcName, detailedErrorMessage)
      }
      if testNet.Spec.NetworkType == "flannel" && testEp.Spec.Iface.Address != "" {
        var ipFile = filepath.Join(defaultDataDir, flannelBridge, testEp.Spec.Iface.Address)
        _,err = os.Lstat(ipFile)
        if err == nil {
          t.Errorf("IP file:" + ipFile + " was not cleaned-up by Flannel IP exhaustion protection code!")
        }
      }
    })
  }
}

func setupDelTest(opType string) error {
  os.RemoveAll(cniTestConfigDir)
  err := os.MkdirAll(cniTestConfigDir, os.ModePerm)
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_PATH", cniTesterDir)
  if err != nil {
    return err
  }
  err = os.Setenv("CNI_COMMAND", opType)
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
  testPlugins := [3]string{"flannel","macvlan","sriov"}
  for _, plugin := range testPlugins {
    os.RemoveAll(filepath.Join(cniTesterDir, plugin))
    input, err := ioutil.ReadFile(filepath.Join(os.Getenv("GOPATH"),"bin","cnitest"))
    if err != nil {
      return err
    }
    err = ioutil.WriteFile(filepath.Join(cniTesterDir, plugin), input, 777)
    if err != nil {
      return err
    }
  }
  for _, confFile := range testCniConfFiles {
    err = ioutil.WriteFile(filepath.Join(cniTestConfigDir, confFile.ConfName), confFile.Conftent, 0666)
    if err != nil {
      return err
    }
  }
  err = utils.SetupAllocationPools(testNets)
  if err != nil {
    return err
  }
  if opType == "ADD" {
    err = sriov_utils.CreateTmpSysFs()
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

func teardownDelTest() error {
  return sriov_utils.RemoveTmpSysFs()
}