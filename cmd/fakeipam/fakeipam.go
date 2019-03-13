package main

import (
  "errors"
  "net"
  "strings"
  "encoding/json"
  "github.com/containernetworking/cni/pkg/skel"
  types "github.com/containernetworking/cni/pkg/types/020"
  gentypes "github.com/containernetworking/cni/pkg/types"
  "github.com/containernetworking/cni/pkg/version"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
)

//Fakeipam plugin is a very simple CNI-style IPAM plugin, which prints the received IP allocation information to its output.
//It is used by DANM to integrate to 3rd-party CNI plugins until its own IPAM plugin is separated into a srandalone binary.
//First, DANM internally handles IPAM duties, then invokes the 3rd-party CNI (e.g. SRIOV) with the full CNI config.
//3rd-party CNIs would invoke the configured fakeipam plugin according to the CNI interface specification.
//At the end, fakeipam will simply regurgitate the IP allocation information originally coming from DANM.

type cniConfig struct {
  Ipam   danmtypes.IpamConfig `json:"ipam"`
}

func reserveIp(args *skel.CmdArgs) error {
  ipamConf, err := loadIpamConfig(args.StdinData)
  if err != nil {
    return err
  }
  cniRes,err := createCniResult(ipamConf)
  if err != nil {
    return err
  }  
  return cniRes.Print()
}

func loadIpamConfig(rawConfig []byte) (danmtypes.IpamConfig,error) {
  cniConf := cniConfig{}
  err := json.Unmarshal(rawConfig, &cniConf)
  if  err != nil {
    return danmtypes.IpamConfig{}, err
  }
  if cniConf.Ipam.Ip == "" {
    return danmtypes.IpamConfig{}, errors.New("No IP was passed to fake IPAM")
  }
  return cniConf.Ipam, nil
}

//TODO: CNI 0.2.0 style of result is used because SRIOV plugin can't handle newer format
//This should be generalized though, and return result should be current.Result in most cases
func createCniResult(ipamConf danmtypes.IpamConfig) (*types.Result,error) {
  _, ip, err := net.ParseCIDR(ipamConf.Ip + "/" + strings.Split(ipamConf.Subnet, "/")[1])
  if err != nil {
    return nil, errors.New("Can't parse IP from IPAM config because:"+err.Error())
  }
  ip.IP = net.ParseIP(ipamConf.Ip)
  var routes []gentypes.Route
  for _, route := range ipamConf.Routes {
    _, destNet, err := net.ParseCIDR(route.Dst)
    if err == nil {
      routes = append(routes, gentypes.Route {
        Dst: *destNet,
        GW: net.ParseIP(route.Gw),
      })
    }
  }
  ipConf := &types.IPConfig{
    IP: *ip,
    Gateway: net.ParseIP(ipamConf.DefaultGw),
    Routes: routes,
  }
  cniRes := &types.Result{IP4: ipConf}
  return cniRes, nil
}
  
func freeIp(args *skel.CmdArgs) error {
  return nil
}

func main() {
  skel.PluginMain(reserveIp, freeIp, version.All)
}