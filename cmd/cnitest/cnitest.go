package main

import (
  "errors"
  "log"
  "os"
  "reflect"
  "encoding/json"
  "io/ioutil"
  "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/cnidel"
)

const (
  cniTestConfigFile = "/etc/cni/net.d/cnitest.conf"
)

type TestConfig struct {
  CniExpectations `json:"cniexp"`
}

type CniExpectations struct {
  CniType    string `json:"cnitype"`
  Ip         string `json:"ip"`
  ReturnType string `json:"return"`
}

type SriovCniTestConfig struct {
  CniConf  cnidel.SriovNet `json:"cniconf"`
}

type MacvlanCniTestConfig struct {
  CniConf  cnidel.MacvlanNet `json:"cniconf"`
}

type FlannelCniTestConfig struct {
  CniConf  FlannelConf     `json:"cniconf"`
}

type FlannelConf struct {
  Name  string             `json:"name"`
  Type  string             `json:"type"`
  Delegate FlannelDelegate `json:"delegate"`
}

type FlannelDelegate struct {
  IsHairPinMode  bool `json:"hairpinMode"`
  IsDefGw        bool `json:"isDefaultGateway"`
}

func testSetup(args *skel.CmdArgs) error {
  var tcConf TestConfig
  expectedCniConf, err := ioutil.ReadFile(cniTestConfigFile)
  if err != nil {
    return errors.New("could not read expected CNI config from disk, because:" + err.Error())
  }
  err = json.Unmarshal(expectedCniConf, &tcConf)
  if err != nil {
    return errors.New("could not unmarshal test CNI config, because:" + err.Error())
  }
  if tcConf.CniExpectations.CniType == "sriov" {
    err = validateSriovConfig(args.StdinData, expectedCniConf)
  } else if tcConf.CniExpectations.CniType == "macvlan" {
    err = validateMacvlanConfig(args.StdinData, expectedCniConf)
  } else if tcConf.CniExpectations.CniType == "flannel" {
    err = validateFlannelConfig(args.StdinData, expectedCniConf)
  }
  if err != nil {
    return err
  }
  cniRes := current.Result {CNIVersion: "0.3.1"}
  if tcConf.CniExpectations.Ip != "" {
    iface := &current.Interface{
      Name: "eth0",
      Mac: "AA:BB:CC:DD:EE:FF",
      Sandbox: "hululululu",
    }
    cniRes.Interfaces = append(cniRes.Interfaces, iface)
    ip, _ := types.ParseCIDR(tcConf.CniExpectations.Ip)
    ipConf := &current.IPConfig {
      Version: "4",
      Address: *ip,
    }
    cniRes.IPs = append(cniRes.IPs, ipConf)
  }
  return cniRes.Print()
}

func validateSriovConfig(receivedCniConfig, expectedCniConfig []byte) error {
  return nil
}

func validateMacvlanConfig(receivedCniConfig, expectedCniConfig []byte) error {
  var recMacvlanConf cnidel.MacvlanNet
  err := json.Unmarshal(receivedCniConfig, &recMacvlanConf)
  if err != nil {
    return errors.New("Received MACVLAN config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Received MACVLAN config:%v",recMacvlanConf)
  var expMacvlanConf MacvlanCniTestConfig
  err = json.Unmarshal(expectedCniConfig, &expMacvlanConf)
  if err != nil {
    return errors.New("Expected MACVLAN config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Expected MACVLAN config:%v",expMacvlanConf.CniConf)
  if !reflect.DeepEqual(recMacvlanConf, expMacvlanConf.CniConf) {
    return errors.New("Received MACVLAN delegate configuration does not match with expected!")
  }
  return nil
}

func validateFlannelConfig(receivedCniConfig, expectedCniConfig []byte) error {
  var recFlannelConf FlannelConf
  err := json.Unmarshal(receivedCniConfig, &recFlannelConf)
  if err != nil {
    return errors.New("Received Flannel config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Received Flannel config:%v",recFlannelConf)
  var expFlannelConf FlannelCniTestConfig
  err = json.Unmarshal(expectedCniConfig, &expFlannelConf)
  if err != nil {
    return errors.New("Expected Flannel config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Expected Flannel config:%v",expFlannelConf.CniConf)
  if !reflect.DeepEqual(recFlannelConf, expFlannelConf.CniConf) {
    return errors.New("Received Flannel delegate configuration does not match with expected!")
  }
  return nil
}

func testDelete(args *skel.CmdArgs) error {
  return nil
}

func main() {
  var err error
  f, err := os.OpenFile("/var/log/cnitest.log", os.O_RDWR | os.O_CREATE , 0666)
  if err == nil {
    log.SetOutput(f)
    defer f.Close()
  }
  skel.PluginMain(testSetup, testDelete, version.All)
}