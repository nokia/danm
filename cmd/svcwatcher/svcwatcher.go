package main

import (
	"flag"
	"log"
	"time"
	"os"
	"fmt"
	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/clientcmd"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1 "k8s.io/api/core/v1"
	danmclientset "github.com/nokia/danm/crd/client/clientset/versioned"
	danminformers "github.com/nokia/danm/crd/client/informers/externalversions"
	"github.com/nokia/danm/pkg/svccontrol"
)

var (
	kubeconfig, version, commitHash string
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	printVersion := flag.Bool("version", false, "prints Git version information of the binary to standard out")
	flag.Parse()
	if *printVersion {
		log.Println("DANM binary was built from release: " + version)
		log.Println("DANM binary was built from commit: " + commitHash)
		return
	}
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

	controller := svccontrol.NewController(kubeClient, danmClient,
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Endpoints(),
		danmInformerFactory.Danm().V1().DanmEps())

	run := func(stopCh <-chan struct{}) {
		go kubeInformerFactory.Start(stopCh)
		go danmInformerFactory.Start(stopCh)

		if err = controller.Run(10, stopCh); err != nil {
			glog.Fatalf("Error running controller: %s", err.Error())
		}
	}
	
	rl, err := resourcelock.New(resourcelock.EndpointsResourceLock,
		"kube-system",
		"danm-svc-controller",
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      GetHostname(),
			EventRecorder: createRecorder(kubeClient, "danm-svc-controller"),
		})
	if err != nil {
		glog.Fatalf("Error creating lock: %v", err)
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 10 * time.Second,
		RenewDeadline: 5 * time.Second,
		RetryPeriod:   3 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				utilruntime.HandleError(fmt.Errorf("Lost master"))
			},
		},
	})

	glog.Fatalln("Lost lease")
}

func GetHostname() (string) {
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
