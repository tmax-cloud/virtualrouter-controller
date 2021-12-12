package main

import (
	"context"
	"flag"
	"os"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	daemon "github.com/tmax-cloud/virtualrouter-controller/internal/daemon"
	internalCrio "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/netlink"
	clientset "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned"
	informers "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/informers/externalversions"
	"github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/signals"
	"github.com/tmax-cloud/virtualrouter-controller/internal/virtualroutermanager"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {

	// internalCidr := flag.String("internalCidr", os.Getenv("internalCIDR"), "The InternalCIDR of the hosts")
	// externalCidr := flag.String("externalCidr", os.Getenv("externalCIDR"), "The ExternalCIDR of the hosts")
	nodeName := flag.String("nodeName", os.Getenv("nodeName"), "The nodeName of the hosts")
	if nodeName == nil {
		klog.Fatalf("Error node Name is empty")
	}
	klog.Info(*nodeName)
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopSignalCh := signals.SetupSignalHandler()
	stopCh := make(chan struct{})
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}
	// labelSelector := v1.LabelSelector{MatchLabels: map[string]string{"app": virtualroutermanager.VIRTUALROUTER_LABEL}}
	labelSelector := labels.Set(map[string]string{"app": virtualroutermanager.VIRTUALROUTER_LABEL}).AsSelector()
	// kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, time.Second*30, kubeinformers.WithTweakListOptions(func(opt *v1.ListOptions) {
		// opt.LabelSelector = labels.Set(labelSelector.MatchLabels).String()
		opt.LabelSelector = labelSelector.String()
		opt.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", *nodeName).String()
	}))

	myNode, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), *nodeName, v1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return
	}
	externalInterfaceName := myNode.GetObjectMeta().GetAnnotations()["externalInterface"]
	internalInterfaceName := myNode.GetObjectMeta().GetAnnotations()["internalInterface"]
	if externalInterfaceName == "" || internalInterfaceName == "" {
		klog.Error("Empty annotation in Node resource. Please check whether externalInterface and internalInterface annotation on the Node")
	}

	d := daemon.NewDaemon(&internalCrio.CrioConfig{
		RuntimeEndpoint:      "unix:///var/run/crio/crio.sock",
		RuntimeEndpointIsSet: true,
		ImageEndpoint:        "unix:///var/run/crio/crio.sock",
		ImageEndpointIsSet:   true,
		Timeout:              time.Duration(2000000000),
	}, &internalNetlink.Config{
		// InternalIPCIDR:        "10.0.0.0/24",
		// ExternalIPCIDR:        "192.168.9.0/24",
		// InternalIPCIDR:              *internalCidr,
		// ExternalIPCIDR:              *externalCidr,
		OriginInternalInterfaceName: internalInterfaceName,
		OriginExternalInterfaceName: externalInterfaceName,
		NewInternalInterfaceName:    "intif",
		NewExternalInterfaceName:    "extif",
		InternalBridgeName:          "intbr",
		ExternalBridgeName:          "extbr",
	})

	// if err := d.Initialize(); err != nil {
	// 	klog.ErrorS(err, "Daemon Initialization failed")
	// 	return
	// }

	// defer func(d *daemon.NetworkDaemon) {
	// 	klog.Info("defer called")
	// 	if err := d.ClearAll(); err != nil {
	// 		klog.ErrorS(err, "Daemon ClearAll failed")
	// 	}
	// }(d)

	err = d.Start(stopSignalCh, stopCh)
	if err != nil {
		klog.Errorf("Error running network daemon: %s", err.Error())
	}

	controller := daemon.NewController(kubeClient, exampleClient, d,
		kubeInformerFactory.Core().V1().Pods(),
		exampleInformerFactory.Tmax().V1().VirtualRouters())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
