package main

import (
  "flag"
  "os"
  "log"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/clientcmd"
  "k8s.io/client-go/tools/cache"
  "github.com/nokia/danm/pkg/netcontrol"
)

func getClientConfig(kubeConfig *string) (*rest.Config, error) {
  if kubeConfig != nil {
    return clientcmd.BuildConfigFromFlags("", *kubeConfig)
  }
  return rest.InClusterConfig()
}

func watchRes(controller cache.Controller) {
  stop := make(chan struct{})
  go controller.Run(stop)
}

func main() {
  log.SetOutput(os.Stdout)
  log.Println("Starting DANM Watcher...")
  kubeConfig := flag.String("kubeconf", "", "Path to a kube config. Only required if out-of-cluster.")
  flag.Parse()
  config, err := getClientConfig(kubeConfig)
  if err != nil {
    log.Println("ERROR: Parsing kubeconfig failed with error:" + err.Error() + " , exiting")
    os.Exit(-1)
  }
  netHandler, err := netcontrol.NewHandler(config)
  if err != nil {
    log.Println("ERROR: Creation of K8s DanmNet Controller failed with error:" + err.Error() + " , exiting")
    os.Exit(-1)
  }
  dnController := netHandler.CreateController()
  watchRes(dnController)

  // Wait forever
  select {}
}
