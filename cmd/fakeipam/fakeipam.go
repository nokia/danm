package main

import (
  "errors"
  "net"
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
  if len(cniConf.Ipam.Ips) == 0 {
    return danmtypes.IpamConfig{}, errors.New("No IP was passed to fake IPAM")
  }
  return cniConf.Ipam, nil
}

//TODO: CNI 0.2.0 style of result is used because SRIOV plugin can't handle newer format
//This should be generalized though, and return result should be current.Result in most cases
func createCniResult(ipamConf danmtypes.IpamConfig) (*types.Result,error) {
  var ip net.IP
  var ipNet = new(net.IPNet)
  var ip4 = new(types.IPConfig)
  var ip6 = new(types.IPConfig)
  var cniRoutes = []gentypes.Route{}
  for _, ipamIp := range ipamConf.Ips {
    ip, ipNet, _ = net.ParseCIDR(ipamIp.IpCidr)
    cniRoutes = nil
    for _, ipamRoute := range ipamIp.Routes {
      _, routeNet, _ := net.ParseCIDR(ipamRoute.Dst)
      cniRoutes = append(cniRoutes, gentypes.Route {
        Dst: *routeNet,
        GW: net.ParseIP(ipamRoute.Gw),
      })
    }
    ipNet.IP = ip
    // CNI Result can have only one IP for each Version.
    // In case multiple IPs are set with the same Version, the last one is used.
    switch ipamIp.Version {
    case 4:
      ip4 = &types.IPConfig{
        IP: *ipNet,
        Gateway: net.ParseIP(ipamIp.DefaultGw),
        Routes: cniRoutes,
      }
    case 6:
      ip6 = &types.IPConfig{
        IP: *ipNet,
        Gateway: net.ParseIP(ipamIp.DefaultGw),
        Routes: cniRoutes,
      }
    }
  }
  cniRes := &types.Result{IP4: ip4, IP6: ip6}
  return cniRes, nil
}

func freeIp(args *skel.CmdArgs) error {
  return nil
}

func main() {
  skel.PluginMain(reserveIp, freeIp, version.All)
}