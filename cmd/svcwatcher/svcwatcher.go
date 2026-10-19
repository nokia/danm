package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	danmclientset "github.com/danm-cni/danm/crd/client/clientset/versioned"
	danminformers "github.com/danm-cni/danm/crd/client/informers/externalversions"
	"github.com/danm-cni/danm/pkg/svccontrol"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/transport"
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
		log.Fatalf("ERROR: kubeconfig could not be built because: %v", err)
	}
	cfg.WarningHandlerWithContext = rest.NoWarnings{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	leaderClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("ERROR: Kubernetes clientset of the leader election could not be built because: %v", err)
	}
	workerCtx, fenceWorkers := context.WithCancel(ctx)
	defer fenceWorkers()
	workerCfg := rest.CopyConfig(cfg)
	workerCfg.Wrap(transport.ContextCanceller(workerCtx, fmt.Errorf("svcwatcher is not the leader anymore or is shutting down")))
	kubeClient, err := kubernetes.NewForConfig(workerCfg)
	if err != nil {
		log.Fatalf("ERROR: Kubernetes clientset of the informers could not be built because: %v", err)
	}
	danmClient, err := danmclientset.NewForConfig(workerCfg)
	if err != nil {
		log.Fatalf("ERROR: DANM clientset could not be built because: %v", err)
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	danmInformerFactory := danminformers.NewSharedInformerFactory(danmClient, time.Second*30)
	controller, err := svccontrol.NewController(ctx.Done(), kubeClient, danmClient,
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Endpoints(),
		danmInformerFactory.Danm().V1().DanmEps())
	if err != nil {
		log.Fatalf("ERROR: svcwatcher controller could not be created because: %v", err)
	}
	kubeInformerFactory.Start(ctx.Done())
	danmInformerFactory.Start(ctx.Done())
	if err := controller.WarmUp(); err != nil {
		log.Fatalf("ERROR: svcwatcher caches could not be synced because: %v", err)
	}
	run := func() {
		if err := controller.Run(10); err != nil {
			log.Fatalf("ERROR: svcwatcher controller stopped because: %v", err)
		}
	}
	rl, err := resourcelock.New(resourcelock.LeasesResourceLock,
		"kube-system",
		"danm-svc-controller",
		leaderClient.CoreV1(),
		leaderClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      GetHostname(),
			EventRecorder: createRecorder(leaderClient, "danm-svc-controller-leader"),
		})
	if err != nil {
		log.Fatalf("ERROR: leader election lock could not be created because: %v", err)
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 20 * time.Second,
		RenewDeadline: 13 * time.Second,
		RetryPeriod:   3 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				controller.SetLeader(true)
				go run()
			},
			OnStoppedLeading: func() {
				controller.SetLeader(false)
				fenceWorkers()
				cancel()
				utilruntime.HandleError(fmt.Errorf("Lost master"))
			},
		},
	})
	log.Fatalln("ERROR: leader lease was lost")
}

func GetHostname() string {
	ret, err := os.Hostname()
	if err != nil {
		log.Fatalf("ERROR: hostname of the node could not be determined because: %v", err)
	}
	return ret
}

func createRecorder(kubeClient *kubernetes.Clientset, comp string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("kube-system")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: comp})
}
