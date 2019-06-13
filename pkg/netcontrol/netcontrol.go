package netcontrol

import (
  "errors"
  "log"
  "strconv"
  "strings"
  "time"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  client "github.com/nokia/danm/crd/client/clientset/versioned/typed/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  danminformers "github.com/nokia/danm/crd/client/informers/externalversions"
  "github.com/nokia/danm/pkg/datastructs"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/cache"
)

// Handler represents an object watching the K8s API for changes in the DanmNet API path
// Upon the reception of a notification it validates the body, and handles the related VxLAN/VLAN/RT creation/deletions on the host
type NetWatcher struct {
  Factories map[string]danminformers.SharedInformerFactory
  Clients map[string]danmclientset.Interface
  Controllers map[string]cache.Controller
}

// NewHandler initializes and returns a new Handler object
// Upon the reception of a notification it performs DanmNet validation, and host network management operations
// Handler contains additional members: one performing HTTPS operations, the other to interact with DamnEp objects
func NewWatcher(cfg *rest.Config) (*NetWatcher,error) {
  netWatcher := &NetWatcher{
    Factories: make(map[string]danminformers.SharedInformerFactory),
    Clients: make(map[string]danmclientset.Interface),
    Controllers: make(map[string]cache.Controller),
  }
  //this is how we test if the specific API is used within the cluster, or not
  //we can only create an Informer for an existing API, otherwise we get errors
  dnetClient, err := danmclientset.NewForConfig(cfg)
  if err != nil {
    return nil, err
  } 
  _, err = dnetClient.DanmV1().DanmNets("").List(meta_v1.ListOptions{})
  if err == nil {
    netWatcher.createDnetInformer(dnetClient)
  }
  tnetClient, err := danmclientset.NewForConfig(cfg)
  if err != nil {
    return nil, err
  }
  _, err = tnetClient.DanmV1().TenantNetworks("").List(meta_v1.ListOptions{})
  if err == nil {
    netWatcher.createTnetInformer(tnetClient)
  }
  cnetClient, err := danmclientset.NewForConfig(cfg)
  if err != nil {
    return nil, err
  }
  _, err = cnetClient.DanmV1().ClusterNetworks().List(meta_v1.ListOptions{})
  if err == nil {
    netWatcher.createCnetInformer(cnetClient)
  }
  log.Println("Number of watcher's started for recognized APIs:" + strconv.Itoa(len(netWatcher.Controllers)))
  if len(netWatcher.Controllers) == 0 {
    return nil, errors.New("no network management APIs are installed in the cluster, netwatcher cannot start!")
  }
  return netWatcher, nil
}

func (netWatcher *NetWatcher) Run(stopCh *chan struct{}) {
  for _, controller := range netWatcher.Controllers {
    go controller.Run(*stopCh)
  }
}


func (netWatcher *NetWatcher) createDnetInformer(dnetClient danmclientset.Interface) { 
  netWatcher.Clients["DanmNet"] = dnetClient
  dnetInformerFactory := danminformers.NewSharedInformerFactory(dnetClient, time.Minute*10)
  netWatcher.Factories["DanmNet"] = dnetInformerFactory
  dnetController := dnetInformerFactory.Danm().V1().DanmNets().Informer()
  dnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddDanmNet,
      DeleteFunc: DeleteDanmNet,
  })
  netWatcher.Controllers["DanmNet"] = dnetController
}

func (netWatcher *NetWatcher) createTnetInformer(tnetClient danmclientset.Interface) {
  netWatcher.Clients["TenantNetwork"] = tnetClient
  tnetInformerFactory := danminformers.NewSharedInformerFactory(tnetClient, time.Minute*10)
  netWatcher.Factories["TenantNetwork"] = tnetInformerFactory
  tnetController := tnetInformerFactory.Danm().V1().TenantNetworks().Informer()
  tnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddTenantNetwork,
      DeleteFunc: DeleteTenantNetwork,
  })
  netWatcher.Controllers["TenantNetwork"] = tnetController
}

func (netWatcher *NetWatcher) createCnetInformer(cnetClient danmclientset.Interface) {
  netWatcher.Clients["ClusterNetwork"] = cnetClient
  cnetInformerFactory := danminformers.NewSharedInformerFactory(cnetClient, time.Minute*10)
  netWatcher.Factories["ClusterNetwork"] = cnetInformerFactory
  cnetController := cnetInformerFactory.Danm().V1().ClusterNetworks().Informer()
  cnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddClusterNetwork,
      DeleteFunc: DeleteClusterNetwork,
  })
  netWatcher.Controllers["ClusterNetwork"] = cnetController
}

func AddDanmNet(obj interface{}) {
  dn, isNetwork := obj.(*danmtypes.DanmNet)
  if !isNetwork {
    log.Println("ERROR: Can't create interfaces for DanmNet, 'cause we have received an invalid object from the K8s API server")
    return
  }
  err := setupHost(dn)
  if err != nil {
    log.Println("ERROR: Creating host interfaces for DanmNet:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func DeleteDanmNet(obj interface{}) {
  dn, isNetwork := obj.(*danmtypes.DanmNet)
  if !isNetwork {
    tombStone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
      if !objIsTombstone {
        log.Println("ERROR: Can't delete interfaces for DanmNet, 'cause we have received an invalid object from the K8s API server")
        return
    }
    var isObjectInTombStoneNetwork bool
    dn, isObjectInTombStoneNetwork = tombStone.Obj.(*danmtypes.DanmNet)
    if !isObjectInTombStoneNetwork {
      log.Println("ERROR: Can't delete interfaces for DanmNet, 'cause we have received an invalid object from the K8s API server in the Event tombstone")
      return
    }
  }
  err := deleteNetworks(dn)
  if err != nil {
    log.Println("INFO: Deletion of host interfaces for DanmNet:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func AddTenantNetwork(obj interface{}) {
  tn, isNetwork := obj.(*danmtypes.TenantNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't create interfaces for TenantNetwork, 'cause we have received an invalid object from the K8s API server")
    return
  }
  dnet := ConvertTnetToDnet(tn)
  err := setupHost(dnet)
  if err != nil {
    log.Println("ERROR: Creating host interfaces for TenantNetwork:" + dnet.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func DeleteTenantNetwork(obj interface{}) {
  tn, isNetwork := obj.(*danmtypes.TenantNetwork)
  if !isNetwork {
    tombStone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
      if !objIsTombstone {
        log.Println("ERROR: Can't delete interfaces for TenantNetwork, 'cause we have received an invalid object from the K8s API server")
        return
    }
    var isObjectInTombStoneNetwork bool
    tn, isObjectInTombStoneNetwork = tombStone.Obj.(*danmtypes.TenantNetwork)
    if !isObjectInTombStoneNetwork {
      log.Println("ERROR: Can't delete interfaces for TenantNetwork, 'cause we have received an invalid object from the K8s API server in the Event tombstone")
      return
    }
  }
  dn := ConvertTnetToDnet(tn)
  err := deleteNetworks(dn)
  if err != nil {
    log.Println("INFO: Deletion of host interfaces for TenantNetwork:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func AddClusterNetwork(obj interface{}) {
  cn, isNetwork := obj.(*danmtypes.ClusterNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't create interfaces for ClusterNetwork, 'cause we have received an invalid object from the K8s API server")
    return
  }
  dnet := ConvertCnetToDnet(cn)
  err := setupHost(dnet)
  if err != nil {
    log.Println("ERROR: Creating host interfaces for ClusterNetwork:" + dnet.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func DeleteClusterNetwork(obj interface{}) {
  cn, isNetwork := obj.(*danmtypes.ClusterNetwork)
  if !isNetwork {
    tombStone, objIsTombstone := obj.(cache.DeletedFinalStateUnknown)
      if !objIsTombstone {
        log.Println("ERROR: Can't delete interfaces for ClusterNetwork, 'cause we have received an invalid object from the K8s API server")
        return
    }
    var isObjectInTombStoneNetwork bool
    cn, isObjectInTombStoneNetwork = tombStone.Obj.(*danmtypes.ClusterNetwork)
    if !isObjectInTombStoneNetwork {
      log.Println("ERROR: Can't delete interfaces for ClusterNetwork, 'cause we have received an invalid object from the K8s API server in the Event tombstone")
      return
    }
  }
  dn := ConvertCnetToDnet(cn)
  err := deleteNetworks(dn)
  if err != nil {
    log.Println("INFO: Deletion of host interfaces for ClusterNetwork:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func ConvertTnetToDnet(tnet *danmtypes.TenantNetwork) *danmtypes.DanmNet {
  return &danmtypes.DanmNet {
    TypeMeta: tnet.TypeMeta,
    ObjectMeta: tnet.ObjectMeta,
    Spec: tnet.Spec,
  }
}

func ConvertCnetToDnet(cnet *danmtypes.ClusterNetwork) *danmtypes.DanmNet {
  return &danmtypes.DanmNet {
    TypeMeta: cnet.TypeMeta,
    ObjectMeta: cnet.ObjectMeta,
    Spec: cnet.Spec,
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