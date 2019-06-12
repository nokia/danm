package netcontrol

import (
  "log"
  "reflect"
  "strings"
  "time"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/cache"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  client "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  danminformers "github.com/nokia/danm/crd/client/informers/externalversions"
  "github.com/nokia/danm/pkg/datastructs"
)

// Handler represents an object watching the K8s API for changes in the DanmNet API path
// Upon the reception of a notification it validates the body, and handles the related VxLAN/VLAN/RT creation/deletions on the host
type NetWatcher struct {
  Client danmclientset.Interface
  Controllers []cache.Controller
}

// NewHandler initializes and returns a new Handler object
// Upon the reception of a notification it performs DanmNet validation, and host network management operations
// Handler contains additional members: one performing HTTPS operations, the other to interact with DamnEp objects
func NewWatcher(cfg *rest.Config) (*NetWatcher,error) {
  netWatcher := NetWatcher{}
  client, err := danmclientset.NewForConfig(cfg)
  if err != nil {
    return &netWatcher, err
  }
  netWatcher.Client = client
  danmInformerFactory := danminformers.NewSharedInformerFactory(client, time.Minute*10)
  dnetController := danmInformerFactory.Danm().V1().DanmNets().Informer()
  dnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: func(obj interface{}) {
        AddDanmNet(netWatcher.Client, *(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      DeleteFunc: func(obj interface{}) {
        DeleteDanmNet(*(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      UpdateFunc: func(oldObj, newObj interface{}) {
     },
  })
  netWatcher.Controllers = append(netWatcher.Controllers, dnetController)
  tnetController := danmInformerFactory.Danm().V1().TenantNetworks().Informer()
  tnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: func(obj interface{}) {
        AddDanmNet(netWatcher.Client, *(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      DeleteFunc: func(obj interface{}) {
        DeleteDanmNet(*(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      UpdateFunc: func(oldObj, newObj interface{}) {
     },
  })
  netWatcher.Controllers = append(netWatcher.Controllers, tnetController)
  cnetController := danmInformerFactory.Danm().V1().ClusterNetworks().Informer()
  cnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: func(obj interface{}) {
        AddDanmNet(netWatcher.Client, *(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      DeleteFunc: func(obj interface{}) {
        DeleteDanmNet(*(reflect.ValueOf(obj).Interface().(*danmtypes.DanmNet)))
      },
      UpdateFunc: func(oldObj, newObj interface{}) {
     },
  })
  netWatcher.Controllers = append(netWatcher.Controllers, cnetController)
  return &netWatcher, nil
}

func (netWatcher *NetWatcher) Run(stopCh *chan struct{}) {
  for _, controller := range netWatcher.Controllers {
    go controller.Run(*stopCh)
  }
}

func PutDanmNet(client client.DanmNetInterface, dnet *danmtypes.DanmNet) (bool,error) {
  var wasResourceAlreadyUpdated bool = false
  _, err := client.Update(dnet)
  if err != nil {
    if strings.Contains(err.Error(),datastructs.OptimisticLockErrorMsg) {
      wasResourceAlreadyUpdated = true
      return wasResourceAlreadyUpdated, nil
    }
    return wasResourceAlreadyUpdated, err
  }
  return wasResourceAlreadyUpdated, nil
}

// validate DanmNet body
// update validity in apiserver, don't care for 409 (PATCH or PUT)
// create host specific network stuff: rt_tables, vlan, and vxlan interfaces
func AddDanmNet(client danmclientset.Interface, dn danmtypes.DanmNet) {
  err := setupHost(&dn)
  if err != nil {
    log.Println("ERROR: Creating host interfaces for DanmNet:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
  return
}

// delete host_specific network stuff: rt_tables, vlan, and vxlan interfaces
func DeleteDanmNet(dn danmtypes.DanmNet) {
  err := deleteNetworks(&dn)
  if err != nil {
    log.Println("INFO: Deletion of host interfaces for DanmNet:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
  return
}
