package datastructs

import (
  "github.com/containernetworking/cni/pkg/types"
)

type NetConf struct {
  types.NetConf
  Kubeconfig   string `json:"kubeconfig"`
  CniConfigDir string `json:"cniDir"`
  NamingScheme string `json:"namingScheme"`
}