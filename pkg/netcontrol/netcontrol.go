package netcontrol

import (
  "errors"
  "log"
  "strconv"
  "strings"
  "time"
  danmtypes "github.com/nokia/danm/crd/apis/danm/v1"
  danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
  danminformers "github.com/nokia/danm/crd/client/informers/externalversions"
  "github.com/nokia/danm/pkg/datastructs"
  meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/cache"
)

const(
  DanmNetKind = "DanmNet"
  TenantNetworkKind = "TenantNetwork"
  ClusterNetworkKind = "ClusterNetwork"
)

// NetWatcher represents an object watching the K8s API for changes in all three network management API paths
// Upon the reception of a notification it handles the related VxLAN/VLAN/RT creation/deletions on the host
type NetWatcher struct {
  Factories map[string]danminformers.SharedInformerFactory
  Clients map[string]danmclientset.Interface
  Controllers map[string]cache.Controller
}

// NewWatcher initializes and returns a new NetWatcher object
// Upon the reception of a notification it performs host network management operations
// Watcher stores all K8s Clients, Factories, and Informeres of the DANM network management APIs
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
  netWatcher.Clients[DanmNetKind] = dnetClient
  dnetInformerFactory := danminformers.NewSharedInformerFactory(dnetClient, time.Minute*10)
  netWatcher.Factories[DanmNetKind] = dnetInformerFactory
  dnetController := dnetInformerFactory.Danm().V1().DanmNets().Informer()
  dnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddDanmNet,
      UpdateFunc: UpdateDanmNet,
      DeleteFunc: DeleteDanmNet,
  })
  netWatcher.Controllers[DanmNetKind] = dnetController
}

func (netWatcher *NetWatcher) createTnetInformer(tnetClient danmclientset.Interface) {
  netWatcher.Clients[TenantNetworkKind] = tnetClient
  tnetInformerFactory := danminformers.NewSharedInformerFactory(tnetClient, time.Minute*10)
  netWatcher.Factories[TenantNetworkKind] = tnetInformerFactory
  tnetController := tnetInformerFactory.Danm().V1().TenantNetworks().Informer()
  tnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddTenantNetwork,
      UpdateFunc: UpdateTenantNetwork,
      DeleteFunc: DeleteTenantNetwork,
  })
  netWatcher.Controllers[TenantNetworkKind] = tnetController
}

func (netWatcher *NetWatcher) createCnetInformer(cnetClient danmclientset.Interface) {
  netWatcher.Clients[ClusterNetworkKind] = cnetClient
  cnetInformerFactory := danminformers.NewSharedInformerFactory(cnetClient, time.Minute*10)
  netWatcher.Factories[ClusterNetworkKind] = cnetInformerFactory
  cnetController := cnetInformerFactory.Danm().V1().ClusterNetworks().Informer()
  cnetController.AddEventHandler(cache.ResourceEventHandlerFuncs{
      AddFunc: AddClusterNetwork,
      UpdateFunc: UpdateClusterNetwork,
      DeleteFunc: DeleteClusterNetwork,
  })
  netWatcher.Controllers[ClusterNetworkKind] = cnetController
}

func AddDanmNet(obj interface{}) {
  dn, isNetwork := obj.(*danmtypes.DanmNet)
  if !isNetwork {
    log.Println("ERROR: Can't create interfaces for DanmNet, 'cause we have received an invalid object from the K8s API server")
    return
  }
  err := setupHost(dn)
  if err != nil {
    log.Println("INFO: Creating host interfaces for DanmNet:" + dn.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func UpdateDanmNet(oldObj, newObj interface{}) {
  oldDn, isNetwork := oldObj.(*danmtypes.DanmNet)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for DanmNet change, 'cause we have received an invalid old object from the K8s API server")
    return
  }
  newdDn, isNetwork := newObj.(*danmtypes.DanmNet)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for DanmNet change, 'cause we have received an invalid new object from the K8s API server")
    return
  }
  zeroVnis(oldDn,newdDn)
  err := deleteNetworks(oldDn)
  if err != nil {
    log.Println("INFO: Deletion of old host interfaces for DanmNet:" + oldDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
  }
  err = setupHost(newdDn)
  if err != nil {
    log.Println("INFO: Creating host interfaces for new DanmNet:" + newdDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
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
    log.Println("INFO: Creating host interfaces for TenantNetwork:" + dnet.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func UpdateTenantNetwork(oldObj, newObj interface{}) {
  oldTn, isNetwork := oldObj.(*danmtypes.TenantNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for TenantNetwork change, 'cause we have received an invalid old object from the K8s API server")
    return
  }
  newTn, isNetwork := newObj.(*danmtypes.TenantNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for TenantNetwork change, 'cause we have received an invalid new object from the K8s API server")
    return
  }
  oldDn := ConvertTnetToDnet(oldTn)
  newdDn := ConvertTnetToDnet(newTn)
  zeroVnis(oldDn,newdDn)
  err := deleteNetworks(oldDn)
  if err != nil {
    log.Println("INFO: Deletion of old host interfaces for TenantNetwork:" + oldDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
  }
  err = setupHost(newdDn)
  if err != nil {
    log.Println("INFO: Creating host interfaces for new TenantNetwork:" + newdDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
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
    log.Println("INFO: Creating host interfaces for ClusterNetwork:" + dnet.ObjectMeta.Name + " failed with error:" + err.Error())
  }
}

func UpdateClusterNetwork(oldObj, newObj interface{}) {
  oldCn, isNetwork := oldObj.(*danmtypes.ClusterNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for ClusterNetwork change, 'cause we have received an invalid old object from the K8s API server")
    return
  }
  newCn, isNetwork := newObj.(*danmtypes.ClusterNetwork)
  if !isNetwork {
    log.Println("ERROR: Can't update interfaces for ClusterNetwork change, 'cause we have received an invalid new object from the K8s API server")
    return
  }
  oldDn := ConvertCnetToDnet(oldCn)
  newdDn := ConvertCnetToDnet(newCn)
  zeroVnis(oldDn,newdDn)
  err := deleteNetworks(oldDn)
  if err != nil {
    log.Println("INFO: Deletion of old host interfaces for ClusterNetwork:" + oldDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
  }
  err = setupHost(newdDn)
  if err != nil {
    log.Println("INFO: Creating host interfaces for new ClusterNetwork:" + newdDn.ObjectMeta.Name + " after update failed with error:" + err.Error())
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
  dnet := danmtypes.DanmNet {
    TypeMeta: tnet.TypeMeta,
    ObjectMeta: tnet.ObjectMeta,
    Spec: tnet.Spec,
  }
  //Why do I need to set this, you could ask?
  //Well, don't: https://github.com/kubernetes/client-go/issues/308
  dnet.TypeMeta.Kind = TenantNetworkKind
  return &dnet
}

func ConvertCnetToDnet(cnet *danmtypes.ClusterNetwork) *danmtypes.DanmNet {
  dnet := danmtypes.DanmNet {
    TypeMeta: cnet.TypeMeta,
    ObjectMeta: cnet.ObjectMeta,
    Spec: cnet.Spec,
  }
  dnet.TypeMeta.Kind = ClusterNetworkKind
  return &dnet
}

func ConvertDnetToTnet(dnet *danmtypes.DanmNet) *danmtypes.TenantNetwork {
  return &danmtypes.TenantNetwork {
    TypeMeta: dnet.TypeMeta,
    ObjectMeta: dnet.ObjectMeta,
    Spec: dnet.Spec,
  }
}

func ConvertDnetToCnet(dnet *danmtypes.DanmNet) *danmtypes.ClusterNetwork {
  return &danmtypes.ClusterNetwork {
    TypeMeta: dnet.TypeMeta,
    ObjectMeta: dnet.ObjectMeta,
    Spec: dnet.Spec,
  }
}

func PutNetwork(danmClient danmclientset.Interface, dnet *danmtypes.DanmNet) (bool,error) {
  var err error
  var wasResourceAlreadyUpdated bool
  if dnet.TypeMeta.Kind == DanmNetKind || dnet.TypeMeta.Kind == "" {
    _, err = danmClient.DanmV1().DanmNets(dnet.ObjectMeta.Namespace).Update(dnet)
  } else if dnet.TypeMeta.Kind == TenantNetworkKind {
    tn := ConvertDnetToTnet(dnet)
    _, err = danmClient.DanmV1().TenantNetworks(dnet.ObjectMeta.Namespace).Update(tn)
  } else if dnet.TypeMeta.Kind == ClusterNetworkKind {
    cn := ConvertDnetToCnet(dnet)
    _, err = danmClient.DanmV1().ClusterNetworks().Update(cn)
  } else {
    return wasResourceAlreadyUpdated, errors.New("can't refresh network object because it has an invalid type:" + dnet.TypeMeta.Kind)
  }
  if err != nil {
    if strings.Contains(err.Error(), datastructs.OptimisticLockErrorMsg) {
      wasResourceAlreadyUpdated = true
      return wasResourceAlreadyUpdated, nil
    }
    return wasResourceAlreadyUpdated, err
  }
  return wasResourceAlreadyUpdated, nil
}

func GetDefaultNetwork(danmClient danmclientset.Interface, defaultNetworkName, nameSpace string) (*danmtypes.DanmNet,error) {
  dnet, err := danmClient.DanmV1().DanmNets(nameSpace).Get(defaultNetworkName, meta_v1.GetOptions{})
  if err == nil && dnet.ObjectMeta.Name == defaultNetworkName  {
    return dnet, nil
  }
  tnet, err := danmClient.DanmV1().TenantNetworks(nameSpace).Get(defaultNetworkName, meta_v1.GetOptions{})
  if err == nil && tnet.ObjectMeta.Name == defaultNetworkName  {
    dn := ConvertTnetToDnet(tnet)
    return dn, nil
  }
  cnet, err := danmClient.DanmV1().ClusterNetworks().Get(defaultNetworkName, meta_v1.GetOptions{})
  if err == nil && cnet.ObjectMeta.Name == defaultNetworkName  {
    dn := ConvertCnetToDnet(cnet)
    return dn, nil
  }
  return nil, errors.New("none of DANM APIs have a suitable default network configured")
}

func GetNetworkFromInterface(danmClient danmclientset.Interface, iface datastructs.Interface, nameSpace string) (*danmtypes.DanmNet,error) {
  var netName, netType string
  if iface.Network != "" {
    netName = iface.Network
    netType = DanmNetKind
    dnet, err := danmClient.DanmV1().DanmNets(nameSpace).Get(iface.Network, meta_v1.GetOptions{})
    if err == nil && dnet.ObjectMeta.Name == iface.Network  {
      return dnet, nil
    }
  } else if iface.TenantNetwork != "" {
    netName = iface.TenantNetwork
    netType = TenantNetworkKind
    tnet, err := danmClient.DanmV1().TenantNetworks(nameSpace).Get(iface.TenantNetwork, meta_v1.GetOptions{})
    if err == nil && tnet.ObjectMeta.Name == iface.TenantNetwork  {
      dnet := ConvertTnetToDnet(tnet)
      return dnet, nil
    }
  } else if iface.ClusterNetwork != "" {
    netName = iface.ClusterNetwork
    netType = ClusterNetworkKind
    cnet, err := danmClient.DanmV1().ClusterNetworks().Get(iface.ClusterNetwork, meta_v1.GetOptions{})
    if err == nil && cnet.ObjectMeta.Name == iface.ClusterNetwork  {
      dnet := ConvertCnetToDnet(cnet)
      return dnet, nil
    }
  }
  return nil, errors.New("requested network:" + netName + " of type:" + netType + " in namespace:" + nameSpace + " does not exist")
}

func GetNetworkFromEp(danmClient danmclientset.Interface, ep danmtypes.DanmEp) (*danmtypes.DanmNet,error) {
  dummyIface := datastructs.Interface{}
  if ep.Spec.ApiType == DanmNetKind || ep.Spec.ApiType == "" {dummyIface.Network = ep.Spec.NetworkName}
  if ep.Spec.ApiType == TenantNetworkKind  {dummyIface.TenantNetwork = ep.Spec.NetworkName}
  if ep.Spec.ApiType == ClusterNetworkKind {dummyIface.ClusterNetwork = ep.Spec.NetworkName}
  return GetNetworkFromInterface(danmClient, dummyIface, ep.ObjectMeta.Namespace)
}

func RefreshNetwork(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet) (*danmtypes.DanmNet,error) {
  dummyIface := datastructs.Interface{}
  if netInfo.TypeMeta.Kind == DanmNetKind || netInfo.TypeMeta.Kind == "" {dummyIface.Network = netInfo.ObjectMeta.Name}
  if netInfo.TypeMeta.Kind == TenantNetworkKind  {dummyIface.TenantNetwork = netInfo.ObjectMeta.Name}
  if netInfo.TypeMeta.Kind == ClusterNetworkKind {dummyIface.ClusterNetwork = netInfo.ObjectMeta.Name}
  return GetNetworkFromInterface(danmClient, dummyIface, netInfo.ObjectMeta.Namespace)
}

//Little trickery: if there was no change in the VNI+host_device combo during the update we set it to 0 in the manifests.
//Thus we avoid unnecessarily recreating host interfaces.
func zeroVnis(oldDn, newDn *danmtypes.DanmNet) {
  if oldDn.Spec.Options.Vlan == newDn.Spec.Options.Vlan && oldDn.Spec.Options.Device == newDn.Spec.Options.Device {
    oldDn.Spec.Options.Vlan = 0
    newDn.Spec.Options.Vlan = 0
  }
  if oldDn.Spec.Options.Vxlan == newDn.Spec.Options.Vxlan && oldDn.Spec.Options.Device == newDn.Spec.Options.Device {
    oldDn.Spec.Options.Vxlan = 0
    newDn.Spec.Options.Vxlan = 0
  }
}