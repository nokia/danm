package main

import (
  "log"
  "os"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/containernetworking/cni/pkg/version"
  "github.com/nokia/danm/pkg/metacni"
)

func main() {
  var err error
  f, err := os.OpenFile("/var/log/plugin.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0640)
  if err == nil {
    log.SetOutput(f)
    defer f.Close()
  }
  skel.PluginMain(metacni.CreateInterfaces, metacni.DeleteInterfaces, version.All)
}
