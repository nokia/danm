package main

import (
  "log"
  "os"
  "github.com/containernetworking/cni/pkg/skel"
  "github.com/nokia/danm/pkg/datastructs"
  "github.com/nokia/danm/pkg/metacni"
)

func main() {
  var err error
  f, err := os.OpenFile("/var/log/danm.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0640)
  if err != nil {
    log.Println("ERROR: cannot create log file, because:" + err.Error())
  }
  defer f.Close()
  log.SetOutput(f)
  log.SetFlags(log.LstdFlags | log.Lmicroseconds)
  skel.PluginMain(metacni.CreateInterfaces, metacni.GetInterfaces, metacni.DeleteInterfaces, datastructs.SupportedCniVersions, "")
}
