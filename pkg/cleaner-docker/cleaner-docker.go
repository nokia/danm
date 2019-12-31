package main

import (
	"flag"
	"os"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
	danminformers "github.com/nokia/danm/crd/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	//"github.com/nokia/danm/pkg/crd/signals"
)

var (
	kubeconfig string
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	//stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	danmClient, err := danmclientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	danmInformerFactory := danminformers.NewSharedInformerFactory(danmClient, time.Second*30)

	controller := NewController(kubeClient, danmClient,
		kubeInformerFactory.Core().V1().Pods(),
		danmInformerFactory.Danm().V1().DanmEps())

	run := func(stopCh <-chan struct{}) {
		go kubeInformerFactory.Start(stopCh)
		go danmInformerFactory.Start(stopCh)
		go func() {
			for {
				res := controller.Initialize()
				if res {
					break
				}
				time.Sleep(5 * time.Second)
			}
		}()
		go func() {
			for {
				controller.RoutineCleanup()
				time.Sleep(90 * time.Second)
			}
		}()

		if err = controller.Run(10, stopCh); err != nil {
			glog.Fatalf("Error running controller: %s", err.Error())
		}
	}
	run(make(chan struct{}))

}

func GetHostname() string {
	ret, err := os.Hostname()
	if err != nil {
		glog.Fatalf("hostname %v", err)
	}
	return ret
}

func createRecorder(kubeClient *kubernetes.Clientset, comp string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("kube-system")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: comp})
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
}

