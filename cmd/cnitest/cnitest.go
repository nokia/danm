package main

import (
  "errors"
  "log"
  "os"
  "reflect"
  "encoding/json"
  "io/ioutil"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/cnidel"
)

const (
  cniTestConfigFile = "/etc/cni/net.d/cnitest.conf"
)

type PartialCniConfig struct {
  CniType  string `json:"cnitype"`
}

type SriovCniTestConfig struct {
  CniType  string          `json:"cnitype"`
  CniConf  cnidel.SriovNet `json:"cniconf"`
}

type MacvlanCniTestConfig struct {
  CniType  string            `json:"cnitype"`
  CniConf  cnidel.MacvlanNet `json:"cniconf"`
}

type FlannelCniTestConfig struct {
  CniType  string      `json:"cnitype"`
  CniConf  FlannelConf `json:"cniconf"`
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
  var preConfig PartialCniConfig
  expectedCniConf, err := ioutil.ReadFile(cniTestConfigFile)
  if err != nil {
    return errors.New("could not read expected CNI config from disk, because:" + err.Error())
  }
  err = json.Unmarshal(expectedCniConf, &preConfig)
  if err != nil {
    return errors.New("could not unmarshal partial CNI config, because:" + err.Error())
  }
  if preConfig.CniType == "sriov" {
    err = validateSriovConfig(args.StdinData, expectedCniConf)
  } else if preConfig.CniType == "macvlan" {
    err = validateMacvlanConfig(args.StdinData, expectedCniConf)
  } else if preConfig.CniType == "flannel" {
    err = validateFlannelConfig(args.StdinData, expectedCniConf)
  }
  if err != nil {
    return err
  }
  var cniRes current.Result
  return cniRes.Print()
}

func validateSriovConfig(receivedCniConfig, expectedCniConfig []byte) error {
  return nil
}

func validateMacvlanConfig(receivedCniConfig, expectedCniConfig []byte) error {
  return nil
}

func validateFlannelConfig(receivedCniConfig, expectedCniConfig []byte) error {
  var recFlannelConf FlannelConf
  err := json.Unmarshal(receivedCniConfig, &recFlannelConf)
  if err != nil {
    return errors.New("Received Flannel config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Received Flannel config:%v",recFlannelConf)
  var expFlannelConfg FlannelCniTestConfig
  err = json.Unmarshal(expectedCniConfig, &expFlannelConfg)
  if err != nil {
    return errors.New("Expected Flannel config could not be unmarshalled, because:" + err.Error())
  }
  log.Printf("Expected Flannel config:%v",expFlannelConfg.CniConf)
  if !reflect.DeepEqual(recFlannelConf, expFlannelConfg.CniConf) {
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