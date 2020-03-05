package svccontrol

import (
	"fmt"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"strings"
	"time"
	danmv1 "github.com/nokia/danm/crd/apis/danm/v1"
	danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
	danmscheme "github.com/nokia/danm/crd/client/clientset/versioned/scheme"
	danminformers "github.com/nokia/danm/crd/client/informers/externalversions/danm/v1"
	danmlisters "github.com/nokia/danm/crd/client/listers/danm/v1"
  "github.com/nokia/danm/pkg/datastructs"
)

const (
  MaxUpdateRetry = 5
  RetryInterval  = 200
)

type Controller struct {
	kubeclient    kubernetes.Interface
	danmclient    danmclientset.Interface

	podLister     corelisters.PodLister
	podSynced     cache.InformerSynced
	serviceLister corelisters.ServiceLister
	serviceSynced cache.InformerSynced
	epsLister     corelisters.EndpointsLister
	epsSynced     cache.InformerSynced
	danmepLister  danmlisters.DanmEpLister
	danmepSynced  cache.InformerSynced
	workqueue     workqueue.RateLimitingInterface
}

func NewController(
	kubeclient kubernetes.Interface,
	danmclient danmclientset.Interface,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	epsInformer coreinformers.EndpointsInformer,
	danmepInformer danminformers.DanmEpInformer) *Controller {

	danmscheme.AddToScheme(scheme.Scheme)
	glog.Info("Creating event broadcaster")

	controller := &Controller{
		kubeclient:    kubeclient,
		danmclient:    danmclient,
		podLister:     podInformer.Lister(),
		podSynced:     podInformer.Informer().HasSynced,
		serviceLister: serviceInformer.Lister(),
		serviceSynced: serviceInformer.Informer().HasSynced,
		epsLister:     epsInformer.Lister(),
		epsSynced:     epsInformer.Informer().HasSynced,
		danmepLister:  danmepInformer.Lister(),
		danmepSynced:  danmepInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Endpoints"),
	}

	glog.Info("Setting up event handlers")

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addPod,
		UpdateFunc: controller.updatePod,
		DeleteFunc: controller.delPod,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addSvc,
		UpdateFunc: controller.updateSvc,
		DeleteFunc: controller.delSvc,
	})

	epsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addEps,
		UpdateFunc: controller.updateEps,
		DeleteFunc: controller.delEps,
	})

	danmepInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addDanmep,
		UpdateFunc: controller.updateDanmep,
		DeleteFunc: controller.delDanmep,
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	glog.Info("Starting svcwatcher controller")

	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceSynced, c.epsSynced, c.podSynced, c.danmepSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		c.workqueue.Forget(obj)
		glog.V(5).Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Println("!!! ERROR !!!")
		return nil
	}
	fmt.Printf("!!! ns, name: %s %s\n", ns, name)
	fmt.Printf("!!! key: %s\n", key)
	return nil
}

//////////////////////////
//                      //
//  Instance functions  //
//                      //
//////////////////////////
func (c *Controller) EpCheckUpdate(ipAddr, ip6Addr string, eps *corev1.Endpoints, pod *corev1.Pod, early bool) error {
  wasIpv4AddressFound := isIpInEp(ipAddr,  eps)
  wasIpv6AddressFound := isIpInEp(ip6Addr, eps)
  if (wasIpv4AddressFound || ipAddr == "") && (wasIpv6AddressFound || ip6Addr == "") {
    return nil
  }
  host := getPodHost(pod)
	targetRef := &corev1.ObjectReference{
		Kind:            "pod",
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		ResourceVersion: pod.ResourceVersion,
	}
	if PodReady(pod) || early {
		eps.Subsets[0].Addresses = createChangedEpAddressList(ipAddr, ip6Addr, host, eps, targetRef, eps.Subsets[0].Addresses)
	} else {
		eps.Subsets[0].NotReadyAddresses = createChangedEpAddressList(ipAddr, ip6Addr, host, eps, targetRef, eps.Subsets[0].NotReadyAddresses)
	}
	return c.UpdateEndpoints(eps)
}

func (c *Controller) UpdateEndpoints(eps *corev1.Endpoints) error {
	if eps.Subsets != nil &&
     len(eps.Subsets[0].Addresses) == 0 &&
     len(eps.Subsets[0].NotReadyAddresses) == 0 {
		eps.Subsets = nil
	}
  var updateCounter int
  var err error
  for {
    if updateCounter >= MaxUpdateRetry {
      break
    }
	  _, err := c.kubeclient.CoreV1().Endpoints(eps.Namespace).Update(eps)
    if err == nil {
      break
    } else if !strings.Contains(err.Error(), datastructs.OptimisticLockErrorMsg){
      updateCounter++
    }
    time.Sleep(RetryInterval * time.Millisecond)
  }
  return err
}

func (c *Controller) UpdateEndpointsList(epList []*corev1.Endpoints) error {
  var err error
	for _, eps := range epList {
		err = c.UpdateEndpoints(eps)
    if err != nil {
      return err
    }
	}
  return nil
}

func (c *Controller) CreateModifyEndpoints(svc *corev1.Service, doesEpAlreadyExist bool, des []*danmv1.DanmEp) {
	epNew := c.MakeNewEps(svc, des)
  if doesEpAlreadyExist {
		c.kubeclient.CoreV1().Endpoints(svc.Namespace).Update(&epNew)
	} else {
		c.kubeclient.CoreV1().Endpoints(svc.Namespace).Create(&epNew)
	}
}

func (c* Controller) UpdatePodRvInEps(epsList []*corev1.Endpoints, pod *corev1.Pod) ([]*corev1.Endpoints) {
	var epList []*corev1.Endpoints
	for _, eps := range epsList {
		if eps.Subsets == nil {
			continue
		}
    newEps := eps.DeepCopy()
		// it is not possible that the same pod is in both ready and in not ready
		for i, a := range eps.Subsets[0].Addresses {
			if a.TargetRef != nil {
				if a.TargetRef.Name == pod.Name && a.TargetRef.Namespace == pod.Namespace {
					newEps.Subsets[0].Addresses[i].TargetRef.ResourceVersion = pod.ResourceVersion
					epList = append(epList, newEps)
				}
			}
		}
		for i, a := range eps.Subsets[0].NotReadyAddresses {
			if a.TargetRef != nil {
				if a.TargetRef.Name == pod.Name && a.TargetRef.Namespace == pod.Namespace {
					newEps.Subsets[0].NotReadyAddresses[i].TargetRef.ResourceVersion = pod.ResourceVersion
					epList = append(epList, newEps)
				}
			}
		}
	}
	return epList
}

func (c* Controller) UpdatePodStatusInEps(epsList []*corev1.Endpoints, pod *corev1.Pod, oldReady, newReady bool) ([]*corev1.Endpoints) {
	var epList []*corev1.Endpoints
	for _, eps := range epsList {
		svc, err := c.serviceLister.Services(eps.Namespace).Get(eps.Name)
		if err != nil {
			glog.Errorf("pod update: get svc %s", err)
			continue
		}
		if eps.Subsets == nil {
			continue
		}
    // Make map for addresses
    readyAddrs := map[string]corev1.EndpointAddress{}
    notReadyAddrs := map[string]corev1.EndpointAddress{}
    for _, a := range eps.Subsets[0].Addresses {
      readyAddrs[a.IP] = a
    }
    for _, a := range eps.Subsets[0].NotReadyAddresses {
      notReadyAddrs[a.IP] = a
    }
		early := (svc.Annotations[TolerateUnreadyEps] == "true")
		// it is not possible that the same pod is in both ready and in not ready
		for _, a := range eps.Subsets[0].Addresses {
			if (a.TargetRef != nil) && (oldReady || (newReady && early)) && a.TargetRef.Name == pod.Name && a.TargetRef.Namespace == pod.Namespace {
				if !early {
          delete(readyAddrs, a.IP)
          notReadyAddrs[a.IP] = a
				}
			}
		}
		for _, a := range eps.Subsets[0].NotReadyAddresses {
			if ( a.TargetRef != nil ) && newReady && a.TargetRef.Name == pod.Name && a.TargetRef.Namespace == pod.Namespace {
        delete(notReadyAddrs, a.IP)
        readyAddrs[a.IP] = a
			}
		}
    newEps := eps.DeepCopy()
    newEps.Subsets[0].Addresses = nil
    newEps.Subsets[0].NotReadyAddresses = nil
    for _, a := range readyAddrs {
      newEps.Subsets[0].Addresses = append(newEps.Subsets[0].Addresses, a)
    }
    for _, a := range notReadyAddrs {
      newEps.Subsets[0].NotReadyAddresses = append(newEps.Subsets[0].NotReadyAddresses, a)
    }
    epList = append(epList, newEps)
	}
	return epList
}

func (c *Controller) MakeNewEps(svc *corev1.Service, des []*danmv1.DanmEp) (corev1.Endpoints) {
  epNew := corev1.Endpoints{
        	ObjectMeta: meta_v1.ObjectMeta{
                	Name:        svc.Name,
                	Namespace:   svc.Namespace,
                	Annotations: svc.GetAnnotations(),
        	},
	}
	if des == nil {
		epNew.Subsets = nil
		return epNew
	}
	var readyEpAddrs []corev1.EndpointAddress
	var notReadyEpAddrs []corev1.EndpointAddress
	for _, de := range des {
		pod, err := c.podLister.Pods(de.Namespace).Get(de.Spec.Pod)
		if err != nil {
			glog.Errorf("makeneweps: get pod %s", err)
			continue
		}
		targetRef := &corev1.ObjectReference{
			Kind:            "pod",
			Namespace:       pod.Namespace,
			Name:            pod.Name,
			ResourceVersion: pod.ResourceVersion,
		}
    hostName := getPodHost(pod)
    if PodReady(pod) || svc.Annotations[TolerateUnreadyEps] == "true" {
      readyEpAddrs = createChangedEpAddressList(strings.Split(de.Spec.Iface.Address, "/")[0], strings.Split(de.Spec.Iface.AddressIPv6, "/")[0], hostName, nil, targetRef, readyEpAddrs)
    } else {
      notReadyEpAddrs = createChangedEpAddressList(strings.Split(de.Spec.Iface.Address, "/")[0], strings.Split(de.Spec.Iface.AddressIPv6, "/")[0], hostName, nil, targetRef, notReadyEpAddrs)
    }
	}
	var epPorts []corev1.EndpointPort
	for _, svcPort := range svc.Spec.Ports {
		ep := corev1.EndpointPort{}
		if svcPort.Name != "" {
			ep.Name = svcPort.Name
		}
		if svcPort.Port != 0 {
			ep.Port = svcPort.Port
		}
		if svcPort.Protocol != "" {
			ep.Protocol = svcPort.Protocol
		}
		epPorts = append(epPorts, ep)
	}
	subsets := []corev1.EndpointSubset{
		{
			Addresses:         readyEpAddrs,
			NotReadyAddresses: notReadyEpAddrs,
			Ports:             epPorts,
		},
	}
	epNew.Subsets = subsets
	return epNew
}
//////////////////////////////
//                          //
//  Danmep change handlers  //
//                          //
//////////////////////////////
func (c *Controller) addDanmep(obj interface{}) {
	if !c.podSynced() || !c.serviceSynced() || !c.epsSynced() || !c.danmepSynced() {
		return
	}
	glog.V(5).Infof("addDanmep is called: %s %s", obj.(*danmv1.DanmEp).GetName(), obj.(*danmv1.DanmEp).GetNamespace())
	de := obj.(*danmv1.DanmEp)
  ipAddr, ip6Addr := getIpsFromDanmEp(de)
	sel := labels.Everything()
	servicesList, err := c.serviceLister.List(sel)
	if err != nil {
		glog.Errorf("addDanmEp: get services: %s", err)
		return
	}
	svcList := MatchExistingSvc(de, servicesList)
	if len(svcList) > 0 {
		for _, svc := range svcList {
			pod, err := c.podLister.Pods(de.Namespace).Get(de.Spec.Pod)
			if err != nil {
				glog.Errorf("addDanmEp: get pod %s", err)
				continue
			}
			eps, err := c.epsLister.Endpoints(svc.Namespace).Get(svc.Name)
			if err != nil && !errors.IsNotFound(err) {
				glog.Errorf("addDanmEp: get ep %s", err)
				continue
			}
			if eps != nil && eps.Subsets != nil {
				early := (svc.Annotations[TolerateUnreadyEps] == "true")
				err := c.EpCheckUpdate(ipAddr, ip6Addr, eps.DeepCopy(), pod, early)
          if err != nil {
            glog.Errorf("Add danmep conflict: %s %s %+v", ipAddr, ip6Addr, eps)
          }
				continue
			}
			desList := []*danmv1.DanmEp{de}
			c.CreateModifyEndpoints(svc, true, desList)
		}
	}
}

func (c *Controller) updateDanmep(old, new interface{}) {
	glog.V(5).Infof("updateDanmep is called: %s %s", new.(*danmv1.DanmEp).GetName(), new.(*danmv1.DanmEp).GetNamespace())
	oldDanmEp := old.(*danmv1.DanmEp)
	newDanmEp := new.(*danmv1.DanmEp)
	if oldDanmEp.ResourceVersion == newDanmEp.ResourceVersion {
		return
	}
	c.delDanmep(old)
	c.addDanmep(new)
}

func (c *Controller) delDanmep(obj interface{}) {
	glog.V(5).Infof("delDanmep is called: %s %s", obj.(*danmv1.DanmEp).GetName(), obj.(*danmv1.DanmEp).GetNamespace())
	de := obj.(*danmv1.DanmEp)
  ipAddr, ip6Addr := getIpsFromDanmEp(de)
	deNs := de.Namespace
	var epList []*corev1.Endpoints
	sel := labels.Everything()
	epsList, err := c.epsLister.List(sel)
	if err != nil {
		glog.Errorf("delDanmep: get epslist: %s", err)
		return
	}
	for _, ep := range epsList {
		if ep.Subsets == nil {
			continue
		}
		epNew := ep.DeepCopy()
		annotations := epNew.GetAnnotations()
		selectorMap, svcNets, err := GetDanmSvcAnnotations(annotations)
		if err != nil {
			glog.Errorf("delDanmEp: selector %s", err)
			return
		}
		if len(selectorMap) == 0 || !isDepSelectedBySvc(de, svcNets) || epNew.Namespace != deNs {
			continue
		}
		deMap := de.GetLabels()
		deFit := IsContain(deMap, selectorMap)
		if !deFit {
			continue
		}
		for index, address := range ep.Subsets[0].Addresses {
      epNew.Subsets[0].Addresses = deleteFromEpAddressList(ipAddr, ip6Addr, index, address, epNew.Subsets[0].Addresses)
		}
		for index, address := range ep.Subsets[0].NotReadyAddresses {
      epNew.Subsets[0].NotReadyAddresses = deleteFromEpAddressList(ipAddr, ip6Addr, index, address, epNew.Subsets[0].NotReadyAddresses)
		}
    epList = append(epList, epNew)
	}
	if len(epList) > 0 {
		err = c.UpdateEndpointsList(epList)
    if err != nil {
      glog.Errorf("delete DanmEp even could not be processed for V4 address: %s and V6 address: %s because of error:%v", ipAddr, ip6Addr, err)
    }
	}
}

///////////////////////////
//                       //
//  Pod change handlers  //
//                       //
///////////////////////////
func (c *Controller) addPod(obj interface{}) {
	// pod adding is handled by cni where danmep is involved, no action is needed
	if !c.podSynced() || !c.serviceSynced() || !c.epsSynced() || !c.danmepSynced() {
		return
	}
	glog.V(5).Infof("addPod is called: %s %s", obj.(*corev1.Pod).GetName(), obj.(*corev1.Pod).GetNamespace())
}

func (c *Controller) updatePod(old, new interface{}) {
	glog.V(5).Infof("updatePod is called: %s %s", new.(*corev1.Pod).GetName(), new.(*corev1.Pod).GetNamespace())
	oldPod := old.(*corev1.Pod)
	newPod := new.(*corev1.Pod)
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}
	labelChange := PodLabelChanged(oldPod, newPod)
	oldReady := PodReady(oldPod)
	newReady := PodReady(newPod)
	sel := labels.Everything()
	epsList, err := c.epsLister.List(sel)
	if err != nil {
		glog.Errorf("updatePod: get eps %s", err)
		return
	}
	if oldReady == newReady && !labelChange {
		// nothing is changed just resource version. endpoints targetref need to be updated
		epList := c.UpdatePodRvInEps(epsList, newPod)
		if len(epList) > 0 {
			c.UpdateEndpointsList(epList)
		}
		return
	}
	// first we need to reflect status change
	if oldReady != newReady {
		// status change
		epList := c.UpdatePodStatusInEps(epsList, newPod, oldReady, newReady)
		if len(epList) > 0 {
			c.UpdateEndpointsList(epList)
		}
	}
	// label change has lower priority
	if labelChange {
		// label change
		podName := newPod.Name
		podNs := newPod.Namespace
		desList, err := c.danmepLister.List(sel)
		if err != nil {
			glog.Errorf("updatePod: get danmep %s", err)
			return
		}
		for _, de := range desList {
			deNew := de.DeepCopy()
			if deNew.Spec.Pod == podName && deNew.Namespace == podNs {
				deLabels := newPod.Labels
				deNew.SetLabels(deLabels)
				c.danmclient.DanmV1().DanmEps(deNew.Namespace).Update(deNew)
			}
		}
	}
}

func (c *Controller) delPod(obj interface{}) {
	// pod deletion is handled by cni where danmep is involved, no action is needed
	glog.V(5).Infof("delPod is called: %s %s", obj.(*corev1.Pod).GetName(), obj.(*corev1.Pod).GetNamespace())
}

///////////////////////////
//                       //
//  Svc change handlers  //
//                       //
///////////////////////////
func (c *Controller) addSvc(obj interface{}) {
	if !c.podSynced() || !c.serviceSynced() || !c.epsSynced() || !c.danmepSynced() {
		return
	}
	glog.V(5).Infof("addSvc is called: %s %s", obj.(*corev1.Service).GetName(), obj.(*corev1.Service).GetNamespace())
	svc := obj.(*corev1.Service)
	svcNs := svc.Namespace
	svcName := svc.Name
	annotations := svc.Annotations
	selectorMap, svcNets, err := GetDanmSvcAnnotations(annotations)
	if err != nil {
		glog.Errorf("addSvc: get anno %s", err)
		return
	}
	if len(selectorMap) > 0 && len(svcNets) > 0 {
		sel := labels.Everything()
		d, err := c.danmepLister.List(sel)
		if err != nil {
			glog.Errorf("addSvc: get danmep %s", err)
			return
		}
		deList := SelectDesMatchLabels(d, selectorMap, svcNets, svcNs)
		e, err := c.epsLister.List(sel)
		if err != nil {
			glog.Errorf("addSvc: get eps %s", err)
			return
		}
		epFound := FindEpsForSvc(e, svcName, svcNs)
		c.CreateModifyEndpoints(svc, epFound, deList)
	}
}

func (c *Controller) updateSvc(old, new interface{}) {
	glog.V(5).Infof("updateSvc is called: %s %s", new.(*corev1.Service).GetName(), new.(*corev1.Service).GetNamespace())
	oldSvc := old.(*corev1.Service)
	newSvc := new.(*corev1.Service)
	if oldSvc.ResourceVersion == newSvc.ResourceVersion || !SvcChanged(oldSvc, newSvc) {
		return
	}
	c.addSvc(new)
}

func (c *Controller) delSvc(obj interface{}) {
	glog.V(5).Infof("delSvc is called: %s %s", obj.(*corev1.Service).GetName(), obj.(*corev1.Service).GetNamespace())
}

///////////////////////////
//                       //
//  Eps change handlers  //
//                       //
///////////////////////////
func (c *Controller) addEps(obj interface{}) {
	if !c.podSynced() || !c.serviceSynced() || !c.epsSynced() || !c.danmepSynced() {
		return
	}
	glog.V(5).Infof("addEps is called: %s %s", obj.(*corev1.Endpoints).GetName(), obj.(*corev1.Endpoints).GetNamespace())
}

func (c *Controller) updateEps(old, new interface{}) {
	glog.V(5).Infof("updateEps is called: %s %s", new.(*corev1.Endpoints).GetName(), new.(*corev1.Endpoints).GetNamespace())
	oldEps := old.(*corev1.Endpoints)
	newEps := new.(*corev1.Endpoints)
	if oldEps.ResourceVersion == newEps.ResourceVersion {
		return
	}
}

func (c *Controller) delEps(obj interface{}) {
	glog.V(5).Infof("delEps is called: %s %s", obj.(*corev1.Endpoints).GetName(), obj.(*corev1.Endpoints).GetNamespace())
}

func isIpInEp(ip string, eps *corev1.Endpoints) bool {
  var isIpPresent bool
    for _, a := range eps.Subsets[0].Addresses {
      if a.IP == ip {
      isIpPresent = true
      break
    }
  }
  return isIpPresent
}

func createChangedEpAddressList(v4Address, v6Address, host string, eps *corev1.Endpoints, targetRef *corev1.ObjectReference, epAddrs []corev1.EndpointAddress) []corev1.EndpointAddress {
  if eps == nil || (v4Address != "" && !isIpInEp(v4Address, eps)) {
    epAddrs = append(epAddrs, corev1.EndpointAddress{IP: v4Address, Hostname: host, TargetRef: targetRef})
  }
  if eps == nil || (v6Address != "" && !isIpInEp(v6Address, eps)) {
    epAddrs = append(epAddrs, corev1.EndpointAddress{IP: v6Address, Hostname: host, TargetRef: targetRef})
  }
  return epAddrs
}

func getPodHost(pod *corev1.Pod) string {
 hostName := pod.Spec.Hostname
  if hostName == "" {
    hostName = pod.Name
  }
  return hostName
}

func getIpsFromDanmEp(de *danmv1.DanmEp) (string,string) {
  var ipAddr, ip6Addr string
  if de.Spec.Iface.Address != "" {
    ipAddr = strings.Split(de.Spec.Iface.Address, "/")[0]
  }
  if de.Spec.Iface.AddressIPv6 != "" {
    ip6Addr = strings.Split(de.Spec.Iface.AddressIPv6, "/")[0]
  }
  return ipAddr, ip6Addr
}

func deleteFromEpAddressList(v4Address, v6Address string, index int, address corev1.EndpointAddress, epAddrs []corev1.EndpointAddress) []corev1.EndpointAddress {
  if v4Address == address.IP || v6Address == address.IP {
    epAddrs = append(epAddrs[:index], epAddrs[index+1:]...)
  }
  return epAddrs
}
