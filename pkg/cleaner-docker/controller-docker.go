package main

import (
	"fmt"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	danmv1 "github.com/nokia/danm/crd/apis/danm/v1"
	danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
	danmscheme "github.com/nokia/danm/crd/client/clientset/versioned/scheme"
	//"github.com/nokia/danm/pkg/danmep"
	"github.com/nokia/danm/pkg/ipam"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	danminformers "github.com/nokia/danm/crd/client/informers/externalversions/danm/v1"
	danmlisters "github.com/nokia/danm/crd/client/listers/danm/v1"
)

type Controller struct {
	kubeclient   kubernetes.Interface
	danmclient   danmclientset.Interface
	dockerclient *docker.Client
	initialized  bool
	Hostname     string
	podLister    corelisters.PodLister
	podSynced    cache.InformerSynced
	danmepLister  danmlisters.DanmEpLister
	danmepSynced  cache.InformerSynced
	workqueue    workqueue.RateLimitingInterface
}

func NewController(
	kubeclient kubernetes.Interface,
	danmclient danmclientset.Interface,
	podInformer coreinformers.PodInformer,
	danmepInformer danminformers.DanmEpInformer) *Controller {

	danmscheme.AddToScheme(scheme.Scheme)
	glog.Info("Creating event broadcaster")

	controller := &Controller{
		kubeclient:  kubeclient,
		danmclient:  danmclient,
		initialized: false,
		podLister:   podInformer.Lister(),
		podSynced:   podInformer.Informer().HasSynced,
		danmepLister:  danmepInformer.Lister(),
		danmepSynced:  danmepInformer.Informer().HasSynced,
		workqueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Endpoints"),
	}

	glog.Info("Setting up event handlers")

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.updatePod,
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	glog.Info("Starting svcwatcher controller")

	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podSynced, c.danmepSynced); !ok {
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
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) getEpsByNode(node string) ([]*danmv1.DanmEp, error) {
	danmeps, err := c.danmepLister.DanmEps("").List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var eps = make([]*danmv1.DanmEp, 0)
	for _, ep := range danmeps {
		if ep.Spec.Host == node {
			eps = append(eps, ep)
		}
	}
	return eps, nil
}

func findEpRelatedContainer(ep *danmv1.DanmEp, containers []docker.APIContainers) bool {
	for _, container := range containers {
		if ep.Spec.CID == container.ID {
			return true
		}
	}
	return false
}

func (c *Controller) findEpWithSameIp(ep *danmv1.DanmEp) (bool, error) {
	danmeps, err := c.danmepLister.DanmEps("").List(labels.Everything())
	if err != nil {
		return false, err
	}
	for _, epA := range danmeps {
		if epA.Spec.EndpointID != ep.Spec.EndpointID && epA.Spec.Iface.Address == ep.Spec.Iface.Address &&
			epA.Spec.NetworkName == ep.Spec.NetworkName && epA.Spec.NetworkType == ep.Spec.NetworkType {
				return true, nil
		}
	}
	return false, nil
}

func (c *Controller) cleanup() error {
	optAll := docker.ListContainersOptions{All: true}
	optExit := docker.ListContainersOptions{All: true, Filters: map[string][]string{"status": {"exited"}}}
	containerAll, err := c.dockerclient.ListContainers(optAll)
	if err != nil {
		glog.Error(err.Error())
		return err
	}
	containerExit, err := c.dockerclient.ListContainers(optExit)
	if err != nil {
		glog.Error(err.Error())
		return err
	}
	eps, err := c.getEpsByNode(GetHostname())
	if err != nil {
		glog.Error(err.Error())
		return err
	}

	for _, ep := range eps {
		fNotfound := !findEpRelatedContainer(ep, containerAll)
		fExited := findEpRelatedContainer(ep, containerExit)
		if fNotfound || fExited {
			glog.Infof("=== Find illegal danmep: %s\n", ep.Spec.EndpointID)
			glog.Infof("=== CID: %s\n", ep.Spec.CID)
			fIpInUse, err := c.findEpWithSameIp(ep)
			if err != nil {
				glog.Error(err.Error())
				return err
			}
			if len(ep.Spec.Iface.Address) != 0 && fIpInUse {
				glog.Infof("--- IP addr %s used by other ep, delete this ep only\n", ep.Spec.Iface.Address)
				c.deleteInterface(ep, true)
				return nil
			}
			glog.Infof("+++ Free up ep with ip4: %s ip6: %s\n", ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
			c.deleteInterface(ep, false)
		}
	}
	return nil
}

func (c *Controller) syncHandler(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.Error("!!! ERROR in sync !!!")
		return nil
	}
	glog.Infof("!!! key: %s\n", key)

	optAll := docker.ListContainersOptions{All: true}
	optExit := docker.ListContainersOptions{All: true, Filters: map[string][]string{"status": {"exited"}}}
	containerAll, err := c.dockerclient.ListContainers(optAll)
	if err != nil {
		glog.Error(err.Error())
		return err
	}
	containerExit, err := c.dockerclient.ListContainers(optExit)
	if err != nil {
		glog.Error(err.Error())
		return err
	}
	eps, err := c.getEpsByNode(GetHostname())
	if err != nil {
		glog.Error(err.Error())
		return err
	}

	for _, ep := range eps {
		if ep.ObjectMeta.Namespace == ns && ep.Spec.Pod == name {
			glog.Infof("!!! ep : %s", ep.Spec.EndpointID)
			glog.Infof("!!! CID: %s", ep.Spec.CID)
			fNotfound := !findEpRelatedContainer(ep, containerAll)
			fExited := findEpRelatedContainer(ep, containerExit)
			if fNotfound || fExited {
				fIpInUse, err := c.findEpWithSameIp(ep)
				if err != nil {
					glog.Error(err.Error())
					return err
				}
				if len(ep.Spec.Iface.Address) != 0 && fIpInUse {
					glog.Infof("--- IP addr %s used by other ep, delete this ep only\n", ep.Spec.Iface.Address)
					c.deleteInterface(ep, true)
					return nil
				}
				glog.Infof("+++ Free up ep with ip4: %s ip6: %s\n", ep.Spec.Iface.Address, ep.Spec.Iface.AddressIPv6)
				c.deleteInterface(ep, false)
			}
		}
	}
	return nil
}

func (c *Controller) Initialize() bool {
	var err error
	glog.Info("###---> Initialize is called")
	c.Hostname = GetHostname()
	glog.Info("###---> on host: " + c.Hostname)
	if !c.podSynced() || !c.danmepSynced() {
		glog.Info("###---> Sync is not finished yet, retry later")
		return false
	}
	c.dockerclient, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		glog.Error(err.Error())
		return false
	}
	c.cleanup()
	c.initialized = true
	return true
}

func (c *Controller) RoutineCleanup() {
	if !c.initialized {
		return
	}
	if c.workqueue.Len() != 0 {
		glog.Infof("unfinished work exists in the queue, abort cleanup this time")
	}
	c.cleanup()
}

func PodRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

//////////////////////////////////////////////////////
// deletes ep from danmnet bitarray and from danmep //
//////////////////////////////////////////////////////
func (c *Controller) deleteInterface(ep *danmv1.DanmEp, delEpOnly bool) {
	netInfo, err := c.danmclient.DanmV1().DanmNets(ep.ObjectMeta.Namespace).Get(ep.Spec.NetworkName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Infof("... DanmNet NotFound, delete ep only")
			delEpOnly = true
		} else {
			glog.Errorf("Cannot fetch net info for net: %s", ep.Spec.NetworkName)
			return
		}
	}
	// For ipvlan where cidr is defined free up reserved ip address
	if delEpOnly == false && netInfo.Spec.Options.Alloc != "" && ep.Spec.NetworkType == "ipvlan" {
		err = ipam.Free(c.danmclient, *netInfo, ep.Spec.Iface.Address)
		if err != nil {
			glog.Errorf(err.Error())
			return
		}
	}
	// delete danmep crd from apiserver
	c.danmclient.DanmV1().DanmEps(ep.ObjectMeta.Namespace).Delete(ep.ObjectMeta.Name, &meta_v1.DeleteOptions{})
}

//////////////////////////
//                      //
//  Instance functions  //
//                      //
//////////////////////////

func (c *Controller) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		glog.Error("!!! ERROR in enqueue !!!")
		return
	}
	c.workqueue.Add(key)
}

///////////////////////////
//                       //
//  Pod change handlers  //
//                       //
///////////////////////////

func (c *Controller) updatePod(old, new interface{}) {
	if !c.initialized {
		return
	}
	oldPod := old.(*corev1.Pod)
	newPod := new.(*corev1.Pod)
	if c.Hostname != oldPod.Spec.NodeName {
		return
	}
	// If pod was ready but something changed after that, then we are interested in it...
	if oldPod.ResourceVersion == newPod.ResourceVersion || !PodRunning(oldPod) {
		return
	}
	glog.Infof("updatePod is called: %s %s", new.(*corev1.Pod).GetName(), new.(*corev1.Pod).GetNamespace())
	c.enqueuePod(old)
}

